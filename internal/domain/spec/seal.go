package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Seal algorithms. SealV1 is written by this version; SealV0 is the
// original format, still verified so existing specs keep working.
const (
	SealV0 = "sha256"
	SealV1 = "sha256-v1"
)

// ErrNotSealed reports a specification without a seal line.
var ErrNotSealed = errors.New("the specification is not sealed")

// TamperedError reports a specification whose content no longer matches
// its seal.
type TamperedError struct {
	Expected string
	Actual   string
}

func (e *TamperedError) Error() string {
	return fmt.Sprintf("the specification changed after it was sealed (sealed %s, now %s)",
		short(e.Expected), short(e.Actual))
}

// SealInfo is the seal found at the end of a specification.
type SealInfo struct {
	Algorithm string
	Hash      string
}

var (
	sealLine = regexp.MustCompile(`^<!--\s*seal:\s*(sha256(?:-v1)?):([a-fA-F0-9]{64})\s*-->$`)
	// legacySealAnywhere reproduces the v0 hashing rule: any line that
	// contains a v0 seal is excluded from the hash.
	legacySealAnywhere = regexp.MustCompile(`<!--\s*seal:\s*sha256:([a-fA-F0-9]{64})\s*-->`)
)

// ReadSeal returns the seal on the last non-blank line, if there is one.
func ReadSeal(content string) (SealInfo, bool) {
	_, s, ok := splitSeal(content)
	return s, ok
}

// Hash returns the v1 hash of content, ignoring a trailing seal line.
// Line endings and trailing whitespace are normalised first, so the hash is
// identical on Windows and Unix checkouts.
func Hash(content string) string {
	body, _, _ := splitSeal(content)
	return sha(normalizeBody(body))
}

// Seal returns content normalised and ended with a fresh v1 seal, replacing
// any previous seal, together with the hash.
func Seal(content string) (sealed, hash string) {
	body, _, _ := splitSeal(content)
	norm := normalizeBody(body)
	hash = sha(norm)
	return norm + "\n\n<!-- seal: " + SealV1 + ":" + hash + " -->\n", hash
}

// StripSeal returns content without its seal line, if it has one.
func StripSeal(content string) string {
	body, _, ok := splitSeal(content)
	if !ok {
		return content
	}
	return strings.TrimRight(body, " \t\r\n") + "\n"
}

// Verify checks that content matches its own seal. It returns ErrNotSealed
// or a *TamperedError.
func Verify(content string) error {
	_, s, ok := splitSeal(content)
	if !ok {
		return ErrNotSealed
	}
	actual := hashFor(content, s.Algorithm)
	if !strings.EqualFold(actual, s.Hash) {
		return &TamperedError{Expected: strings.ToLower(s.Hash), Actual: actual}
	}
	return nil
}

func hashFor(content, algo string) string {
	if algo == SealV0 {
		var kept []string
		for _, line := range strings.Split(content, "\n") {
			if !legacySealAnywhere.MatchString(line) {
				kept = append(kept, line)
			}
		}
		return sha(strings.TrimSpace(strings.Join(kept, "\n")))
	}
	return Hash(content)
}

// splitSeal separates the body from a seal on the last non-blank line.
func splitSeal(content string) (body string, seal SealInfo, ok bool) {
	trimmed := strings.TrimRight(content, " \t\r\n")
	cut := strings.LastIndex(trimmed, "\n")
	last := strings.TrimSpace(trimmed[cut+1:])
	m := sealLine.FindStringSubmatch(last)
	if m == nil {
		return content, SealInfo{}, false
	}
	if cut < 0 {
		return "", SealInfo{Algorithm: m[1], Hash: m[2]}, true
	}
	return trimmed[:cut], SealInfo{Algorithm: m[1], Hash: m[2]}, true
}

func normalizeBody(body string) string {
	lines := strings.Split(normalizeNewlines(body), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
