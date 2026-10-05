// Package audit is the adversarial security review: reconnaissance maps the
// attack surface, a red-team hunter proposes findings, and an independent
// verifier tries to refute them. The result fails closed:
//
//   - a step whose output is not the required JSON is retried once and then
//     stops the audit with an error, never with an empty "passing" report;
//   - the final report is validated against report-schema.json;
//   - severe findings the verifier could not settle (needs_validation) are
//     put to the developer, and block when nobody can answer.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"specforge/assets"
	"specforge/internal/app/clarify"
	"specforge/internal/domain/security"
	"specforge/internal/jsontext"
	"specforge/internal/ports"
)

// Scope selects what is audited.
type Scope string

const (
	// ScopeDiff audits the changes between a base ref and the working tree.
	ScopeDiff Scope = "diff"
	// ScopeFull audits every source file, in chunks.
	ScopeFull Scope = "full"
)

// Options configure one audit.
type Options struct {
	Root          string
	Scope         Scope
	Base          string
	Threshold     security.Severity
	Language      string
	Model         string
	AgentEnv      []string
	AgentTimeout  time.Duration
	MaxChunkBytes int
}

// Deps are the collaborators of the audit.
type Deps struct {
	Agent  ports.Agent
	VCS    ports.DiffSource
	Files  ports.Files
	Lister func(root string) ([]string, error) // fallback when root is not a git repository
	Asker  *clarify.Asker
	Events Events
	Log    *slog.Logger
	Now    func() time.Time
}

// Events lets the presentation layer follow the audit.
type Events interface {
	Target(scope Scope, base string, chunks int)
	Step(chunk, total int, step string)
	Retried(step string)
}

// Result of an audit.
type Result struct {
	Report *security.Report
	// Blocking are the findings that fail the audit.
	Blocking []security.Finding
	// Empty is true when there was nothing to audit (no changes).
	Empty bool
	// Dir is where the reports were written.
	Dir string
}

// BlockedError reports findings that fail the audit.
type BlockedError struct {
	Findings  []security.Finding
	Threshold security.Severity
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("%d security finding(s) block the merge (threshold %s)", len(e.Findings), e.Threshold)
}

// StepError reports a step whose output never matched its JSON contract.
type StepError struct {
	Step   string
	Output string
}

func (e *StepError) Error() string {
	return fmt.Sprintf("the %s step did not return the required JSON (the audit fails closed rather than report a pass)", e.Step)
}

// Service runs audits.
type Service struct{ d Deps }

// New returns an audit service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	if d.Events == nil {
		d.Events = nopEvents{}
	}
	return &Service{d: d}
}

type nopEvents struct{}

func (nopEvents) Target(Scope, string, int) {}
func (nopEvents) Step(int, int, string)     {}
func (nopEvents) Retried(string)            {}

type method struct {
	recon, hunting, validation, attackClasses string
	schema                                    *jsonschema.Schema
}

func loadMethod() (method, error) {
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(assets.FS, "audit/"+name)
		return string(b), err
	}
	var m method
	var errs []error
	var err error
	if m.recon, err = read("RECONNAISSANCE.md"); err != nil {
		errs = append(errs, err)
	}
	if m.hunting, err = read("HUNTING.md"); err != nil {
		errs = append(errs, err)
	}
	if m.validation, err = read("VALIDATION.md"); err != nil {
		errs = append(errs, err)
	}
	if m.attackClasses, err = read("ATTACK-CLASSES.md"); err != nil {
		errs = append(errs, err)
	}
	schemaBytes, err := fs.ReadFile(assets.FS, "audit/report-schema.json")
	if err != nil {
		errs = append(errs, err)
	} else {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
		if err != nil {
			errs = append(errs, err)
		} else {
			c := jsonschema.NewCompiler()
			if err := c.AddResource("report-schema.json", doc); err != nil {
				errs = append(errs, err)
			} else if m.schema, err = c.Compile("report-schema.json"); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return m, errors.Join(errs...)
}

// Run audits the project and writes docs/security/.
func (s *Service) Run(ctx context.Context, o Options) (Result, error) {
	if o.MaxChunkBytes <= 0 {
		o.MaxChunkBytes = 120_000
	}
	if o.AgentTimeout <= 0 {
		o.AgentTimeout = 20 * time.Minute
	}
	m, err := loadMethod()
	if err != nil {
		return Result{}, fmt.Errorf("loading the audit methodology: %w", err)
	}

	chunks, base, err := s.targets(ctx, o)
	if err != nil {
		return Result{}, err
	}
	s.d.Events.Target(o.Scope, base, len(chunks))
	dir := filepath.Join(o.Root, "docs", "security")
	if len(chunks) == 0 {
		return Result{Empty: true, Dir: dir, Report: &security.Report{Findings: []security.Finding{}}}, nil
	}

	total := &security.Report{Findings: []security.Finding{}}
	var ledgers []json.RawMessage
	for i, code := range chunks {
		rep, ledger, err := s.auditChunk(ctx, o, m, i+1, len(chunks), code)
		if err != nil {
			return Result{}, err
		}
		ledgers = append(ledgers, ledger)
		prefix := ""
		if len(chunks) > 1 {
			prefix = fmt.Sprintf("C%d", i+1)
		}
		total.Merge(rep, prefix)
	}
	total.GeneratedAt = s.d.Now().UTC().Format(time.RFC3339)

	if err := s.settle(ctx, o, total, dir); err != nil {
		return Result{}, err
	}
	if err := s.write(dir, o.Language, total, ledgers); err != nil {
		return Result{}, err
	}

	res := Result{Report: total, Blocking: total.Blocking(o.Threshold), Dir: dir}
	if len(res.Blocking) > 0 {
		return res, &BlockedError{Findings: res.Blocking, Threshold: o.Threshold}
	}
	return res, nil
}

func (s *Service) auditChunk(ctx context.Context, o Options, m method, n, total int, code string) (*security.Report, json.RawMessage, error) {
	s.d.Events.Step(n, total, "recon")
	ledger, err := s.step(ctx, o, "recon",
		m.recon+"\n\n## Code under audit\n\n"+code,
		func(raw json.RawMessage) bool { return hasKey(raw, "units") })
	if err != nil {
		return nil, nil, err
	}

	s.d.Events.Step(n, total, "hunting")
	candidates, err := s.step(ctx, o, "hunting",
		m.hunting+"\n\n"+m.attackClasses+"\n\n## Coverage ledger\n\n```json\n"+string(ledger)+"\n```\n\n## Source code\n\n"+code,
		func(raw json.RawMessage) bool { return hasKey(raw, "candidates") })
	if err != nil {
		return nil, nil, err
	}

	s.d.Events.Step(n, total, "validation")
	var rep *security.Report
	_, err = s.step(ctx, o, "validation",
		m.validation+"\n\n## Candidate findings from the hunter\n\n```json\n"+string(candidates)+"\n```\n\n## Code for counter-verification\n\n"+code,
		func(raw json.RawMessage) bool {
			r, ok := validReport(m.schema, raw)
			if ok {
				rep = r
			}
			return ok
		})
	if err != nil {
		return nil, nil, err
	}
	return rep, ledger, nil
}

// step runs one prompt and returns the first JSON value accepted by ok,
// retrying once with an explicit reminder.
func (s *Service) step(ctx context.Context, o Options, name, prompt string, ok func(json.RawMessage) bool) (json.RawMessage, error) {
	const reminder = "\n\n---\nYour previous answer did not contain the JSON required above. Reply again with that JSON object only, in a ```json block."
	var out string
	for attempt := 0; attempt < 2; attempt++ {
		p := prompt
		if attempt > 0 {
			p += reminder
			s.d.Events.Retried(name)
		}
		var err error
		out, err = s.d.Agent.Run(ctx, ports.AgentRequest{Prompt: p, Dir: o.Root, Model: o.Model, Env: o.AgentEnv, Timeout: o.AgentTimeout})
		if err != nil {
			return nil, fmt.Errorf("audit %s step: %w", name, err)
		}
		for _, c := range jsontext.Candidates(out) {
			if ok(json.RawMessage(c)) {
				return json.RawMessage(c), nil
			}
		}
	}
	return nil, &StepError{Step: name, Output: excerpt(out)}
}

func validReport(schema *jsonschema.Schema, raw json.RawMessage) (*security.Report, bool) {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil || schema.Validate(inst) != nil {
		return nil, false
	}
	var r security.Report
	if json.Unmarshal(raw, &r) != nil || r.Validate() != nil {
		return nil, false
	}
	r.Recount()
	return &r, true
}

func hasKey(raw json.RawMessage, key string) bool {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// settle asks the developer about severe findings the verifier could not
// decide. Without a terminal they stay unresolved, and therefore blocking.
func (s *Service) settle(ctx context.Context, o Options, r *security.Report, dir string) error {
	if s.d.Asker == nil {
		return nil
	}
	q := settleQuestion(o.Language)
	origin := clarify.Origin{Phase: "AUDIT", DecisionsFile: filepath.Join(dir, "decisions.md"), QuestionsFile: filepath.Join(dir, "questions.md")}
	for _, i := range r.Unresolved(o.Threshold) {
		f := &r.Findings[i]
		text := fmt.Sprintf(q.text, f.ID, f.Severity, f.Title, f.File, f.Line, f.Description)
		answer, err := s.d.Asker.Ask(ctx, origin, ports.Question{Text: text, Options: q.options})
		var pending *clarify.PendingQuestionError
		if errors.As(err, &pending) {
			return nil // no terminal: the finding keeps blocking
		}
		if err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case strings.ToLower(q.options[0]):
			f.Status = security.Confirmed
			f.Decision = answer
		case strings.ToLower(q.options[1]):
			f.Status = security.Rejected
			f.Decision = answer
		}
	}
	r.Recount()
	return nil
}

func (s *Service) write(dir, lang string, r *security.Report, ledgers []json.RawMessage) error {
	findings, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	ledger, err := json.MarshalIndent(ledgers, "", "  ")
	if err != nil {
		return err
	}
	return errors.Join(
		s.d.Files.WritePrivate(filepath.Join(dir, "findings.json"), findings),
		s.d.Files.WritePrivate(filepath.Join(dir, "coverage-ledger.json"), ledger),
		s.d.Files.WritePrivate(filepath.Join(dir, "REPORT.md"), []byte(r.Markdown(lang))),
	)
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2000 {
		return s[:2000] + "…"
	}
	return s
}

type settleText struct {
	text    string
	options []string
}

func settleQuestion(lang string) settleText {
	if lang == "es" {
		return settleText{
			text:    "El verificador no pudo decidir sobre %s (%s): %s en %s:%d.\n%s\n¿Es explotable?",
			options: []string{"Sí: confirmarla", "No: descartarla", "No lo sé: mantenerla bloqueante"},
		}
	}
	return settleText{
		text:    "The verifier could not decide on %s (%s): %s at %s:%d.\n%s\nIs it exploitable?",
		options: []string{"Yes: confirm it", "No: reject it", "Not sure: keep it blocking"},
	}
}
