package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

const body = "# Spec\n\nScenario: S\n  Given x\n"

func TestSealRoundTrip(t *testing.T) {
	sealed, hash := Seal(body)
	if err := Verify(sealed); err != nil {
		t.Fatalf("fresh seal must verify: %v", err)
	}
	s, ok := ReadSeal(sealed)
	if !ok || s.Algorithm != SealV1 || s.Hash != hash {
		t.Fatalf("ReadSeal = %+v %v", s, ok)
	}
}

func TestSealIsIndependentOfLineEndingsAndTrailingSpaces(t *testing.T) {
	sealed, _ := Seal(body)
	crlf := strings.ReplaceAll(sealed, "\n", "\r\n")
	spaced := strings.ReplaceAll(sealed, "Given x", "Given x   ")
	for name, content := range map[string]string{"crlf": crlf, "trailing spaces": spaced} {
		if err := Verify(content); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResealReplacesThePreviousSeal(t *testing.T) {
	once, _ := Seal(body)
	twice, _ := Seal(once)
	if strings.Count(twice, "<!-- seal:") != 1 {
		t.Fatalf("resealing must not stack seals:\n%s", twice)
	}
	if once != twice {
		t.Fatalf("sealing is idempotent")
	}
}

func TestTamperingIsDetected(t *testing.T) {
	sealed, hash := Seal(body)
	tampered := strings.Replace(sealed, "Given x", "Given y", 1)
	var te *TamperedError
	if err := Verify(tampered); !errors.As(err, &te) || te.Expected != hash {
		t.Fatalf("want TamperedError, got %v", err)
	}
}

func TestAQuotedOldSealInTheBodyIsPartOfTheContent(t *testing.T) {
	// Regression: the old regex matched the first seal in the file, so a
	// seal quoted in the body was validated instead of the real one.
	quoted := body + "\nExample: `<!-- seal: sha256-v1:" + strings.Repeat("a", 64) + " -->`\n"
	sealed, _ := Seal(quoted)
	if err := Verify(sealed); err != nil {
		t.Fatalf("quoted seal must not confuse verification: %v", err)
	}
	if err := Verify(strings.Replace(sealed, strings.Repeat("a", 64), strings.Repeat("b", 64), 1)); err == nil {
		t.Fatal("editing the quoted text is a change to the content")
	}
}

func TestUnsealed(t *testing.T) {
	if err := Verify(body); !errors.Is(err, ErrNotSealed) {
		t.Fatalf("want ErrNotSealed, got %v", err)
	}
}

func TestLegacyV0SealsStillVerify(t *testing.T) {
	// The v0 rule: drop every line holding a v0 seal, join, trim, sha256.
	content := "# Feature: Test\nScenario: Demo\nGiven test\nWhen run\nThen pass"
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	legacy := content + "\n\n<!-- seal: sha256:" + hash + " -->\n"

	if err := Verify(legacy); err != nil {
		t.Fatalf("a v0 seal written by SpecForge 3.0 must still verify: %v", err)
	}
	if err := Verify(legacy + "edit\n"); err == nil {
		// The seal is no longer the last line, so the file reads as unsealed.
		t.Fatal("content after the seal must not verify")
	}
}

func FuzzSeal(f *testing.F) {
	f.Add(body)
	f.Add("")
	f.Add("<!-- seal: sha256-v1:" + strings.Repeat("0", 64) + " -->")
	f.Fuzz(func(t *testing.T, s string) {
		sealed, hash := Seal(s)
		if err := Verify(sealed); err != nil {
			t.Fatalf("Seal output must verify: %v", err)
		}
		if again, h2 := Seal(sealed); again != sealed || h2 != hash {
			t.Fatal("Seal must be idempotent")
		}
	})
}
