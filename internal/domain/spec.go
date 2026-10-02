package domain

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Scenario representa un escenario BDD / Gherkin extraído de la especificación
type Scenario struct {
	Index       int      `json:"index"`
	Title       string   `json:"title"`
	Given       []string `json:"given"`
	When        []string `json:"when"`
	Then        []string `json:"then"`
	RawContent  string   `json:"raw_content"`
}

// BDDFeature representa la especificación funcional completa
type BDDFeature struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Scenarios   []Scenario `json:"scenarios"`
	SpecHash    string     `json:"spec_hash"`
}

var sealRegex = regexp.MustCompile(`<!--\s*seal:\s*sha256:([a-fA-F0-9]{64})\s*-->`)

// ExtractSpecSeal extrae el hash SHA-256 del sello criptográfico si existe en el contenido
func ExtractSpecSeal(content string) (string, error) {
	matches := sealRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return "", fmt.Errorf("no se encontró un sello criptográfico válido (<!-- seal: sha256:... -->)")
	}
	return strings.ToLower(matches[1]), nil
}

// CalculateCleanSpecHash calcula el hash del contenido de la spec excluyendo la línea del sello
func CalculateCleanSpecHash(content string) string {
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		if !sealRegex.MatchString(line) {
			cleaned = append(cleaned, line)
		}
	}
	cleanContent := strings.TrimSpace(strings.Join(cleaned, "\n"))
	return CalculateBytesSHA256([]byte(cleanContent))
}

// VerifySpecIntegrity comprueba que la especificación en disco no ha sido alterada
func VerifySpecIntegrity(specPath string, expectedHash string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("error leyendo especificación %s: %w", specPath, err)
	}

	calculatedHash := CalculateCleanSpecHash(string(data))
	if calculatedHash != expectedHash {
		return fmt.Errorf("la especificación ha sido modificada manualmente (esperado: %s, actual: %s). Genera un nuevo sello antes de continuar", expectedHash, calculatedHash)
	}

	return nil
}

// ExtractNeedsClarification detecta cualquier cuestión abierta marcada con [NEEDS CLARIFICATION]
func ExtractNeedsClarification(content string) []string {
	var items []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "[NEEDS CLARIFICATION]") {
			val := strings.TrimPrefix(trimmed, "-")
			val = strings.TrimPrefix(val, "*")
			val = strings.TrimSpace(val)
			items = append(items, val)
		}
	}
	return items
}

// ParseScenarios extrae los escenarios Feature/Scenario/Given/When/Then del archivo spec.md
func ParseScenarios(content string) ([]Scenario, error) {
	var scenarios []Scenario
	scanner := bufio.NewScanner(strings.NewReader(content))

	var current *Scenario
	scenarioIndex := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)

		if strings.HasPrefix(lower, "scenario:") || strings.HasPrefix(lower, "escenario:") || strings.HasPrefix(lower, "scenario outline:") {
			if current != nil {
				scenarios = append(scenarios, *current)
			}
			parts := strings.SplitN(line, ":", 2)
			title := ""
			if len(parts) > 1 {
				title = strings.TrimSpace(parts[1])
			}
			scenarioIndex++
			current = &Scenario{
				Index:      scenarioIndex,
				Title:      title,
				Given:      make([]string, 0),
				When:       make([]string, 0),
				Then:       make([]string, 0),
				RawContent: line + "\n",
			}
			continue
		}

		if current != nil {
			current.RawContent += line + "\n"
			if strings.HasPrefix(lower, "given ") || strings.HasPrefix(lower, "dado ") {
				current.Given = append(current.Given, strings.TrimSpace(line[5:]))
			} else if strings.HasPrefix(lower, "when ") || strings.HasPrefix(lower, "cuando ") {
				current.When = append(current.When, strings.TrimSpace(line[5:]))
			} else if strings.HasPrefix(lower, "then ") || strings.HasPrefix(lower, "entonces ") {
				current.Then = append(current.Then, strings.TrimSpace(line[5:]))
			}
		}
	}

	if current != nil {
		scenarios = append(scenarios, *current)
	}

	if len(scenarios) == 0 {
		// Fallback: Si no hay formato Gherkin estricto, crear un escenario global con el contenido
		scenarios = append(scenarios, Scenario{
			Index:      1,
			Title:      "Requerimiento General de la Especificación",
			RawContent: content,
		})
	}

	return scenarios, nil
}
