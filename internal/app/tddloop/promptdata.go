package tddloop

import (
	"strings"

	"specforge/internal/app/prompts"
	"specforge/internal/domain/lessons"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
)

const (
	maxContextFiles     = 3
	maxContextFileBytes = 8_000
	maxLessonsBytes     = 6_000
)

// promptData builds the context shared by every prompt of a scenario.
func (s *Service) promptData(r *run, sc tdd.ScenarioRef) prompts.Data {
	var source string
	for _, d := range r.doc.Scenarios {
		if d.Index == sc.Index {
			source = d.Source
		}
	}
	p := r.o.Profile
	return prompts.Data{
		SpecTitle:   r.doc.Title,
		SpecPath:    r.lay.Rel(r.o.SpecPath),
		Stack:       p.Name(),
		Index:       sc.Index,
		Total:       len(r.st.Scenarios),
		Scenario:    source,
		Marker:      sc.Marker,
		MarkerHint:  p.MarkerExample(sc.Marker),
		TestCommand: p.TestCommand(sc.Marker),
		Glossary:    spec.Section(r.md, spec.GlossaryTitle),
		Invariants:  spec.Section(r.md, spec.InvariantsTitle),
		Plan:        r.plan,
		Decisions:   s.d.Asker.Decisions(s.origin(r, sc)),
		Lessons:     s.lessons(r),
		MaxAttempts: r.o.MaxAttempts,
		Attempt:     r.st.Attempts,
		Known:       knownFailures(r.st),
		Surfaces:    r.allowedSurfaces(),

		Legacy:           r.o.Legacy,
		LegacySources:    legacySources(r.o.Legacy, r.md),
		JavaRelease:      r.o.JavaRelease,
		ForbiddenImports: r.o.ForbiddenImports,
	}
}

// legacySources is the specification's section that cites the legacy
// code, shown only in a rewrite.
func legacySources(legacyDir, md string) string {
	if legacyDir == "" {
		return ""
	}
	return spec.Section(md, spec.LegacySourcesTitle)
}

func (s *Service) lessons(r *run) string {
	data, err := s.d.Files.ReadFile(r.lay.Lessons())
	if err != nil {
		return ""
	}
	text := lessons.For(string(data), string(r.o.Profile.Kind))
	if len(text) > maxLessonsBytes {
		text = text[len(text)-maxLessonsBytes:]
	}
	return text
}

// specTests returns the test files that already carry a marker of this
// specification, so the agent extends them instead of scattering tests.
func (s *Service) specTests(r *run, only []string) []prompts.File {
	prefix := spec.Marker(r.specID, 0)
	prefix = prefix[:len(prefix)-3] // "SDD_0001_"
	var paths []string
	if only != nil {
		for _, p := range only {
			if r.o.Profile.IsTestFile(p) {
				paths = append(paths, p)
			}
		}
	} else {
		hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
		if err != nil {
			return nil
		}
		for p := range hashes {
			paths = append(paths, p)
		}
	}

	var files []prompts.File
	for _, p := range sortedStrings(paths) {
		if len(files) == maxContextFiles {
			break
		}
		data, err := s.d.Files.ReadFile(r.lay.Abs(p))
		if err != nil || (only == nil && !strings.Contains(string(data), prefix)) {
			continue
		}
		content := string(data)
		if len(content) > maxContextFileBytes {
			content = content[:maxContextFileBytes] + "\n[… truncated …]"
		}
		files = append(files, prompts.File{Path: p, Content: content})
	}
	return files
}
