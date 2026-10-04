package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryFile representa uno de los 6 pilares de memoria explícita
type MemoryFile struct {
	Name        string
	Title       string
	Description string
}

var ContextMemoryFiles = []MemoryFile{
	{Name: "agente.md", Title: "1. MISIÓN Y CONSTITUCIÓN PRINCIPAL", Description: "Leyes fundamentales de artesanía de software y TDD."},
	{Name: "persona.md", Title: "2. PERSONALIDAD Y TONO", Description: "Estilo técnico, directo, conciso e inglés en código."},
	{Name: "ng-rules.md", Title: "3. REGLAS PROHIBIDAS (NG RULES)", Description: "Anti-patrones estrictamente prohibidos."},
	{Name: "glossary.md", Title: "4. GLOSARIO Y LENGUAJE UBICUO", Description: "Definición estricta de términos de negocio y arquitectura."},
	{Name: "references.md", Title: "5. PATRONES DE REFERENCIA (GOLDEN MASTERS)", Description: "Ejemplos de implementación idiomática."},
	{Name: "lessons.md", Title: "6. HISTORIAL DE LECCIONES APRENDIDAS", Description: "Registro dinámico de fallos previos y soluciones."},
}

// LoadAgentContext lee los 6 archivos de memoria y los fusiona en un System Instruction consolidado
func LoadAgentContext(projectRoot string) (string, error) {
	agentDir := filepath.Join(projectRoot, ".sdd", "agent")
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		// Fallback a .specify/agent si existiera
		altDir := filepath.Join(projectRoot, ".specify", "agent")
		if _, err := os.Stat(altDir); err == nil {
			agentDir = altDir
		} else {
			return "", nil
		}
	}

	var sb strings.Builder
	sb.WriteString("==================================================================\n")
	sb.WriteString("# CEREBRO CONTEXTUAL Y MEMORIA EXPLÍCITA DEL AGENTE\n")
	sb.WriteString("==================================================================\n\n")

	hasContent := false
	for _, mf := range ContextMemoryFiles {
		filePath := filepath.Join(agentDir, mf.Name)
		data, err := os.ReadFile(filePath)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			hasContent = true
			sb.WriteString(fmt.Sprintf("## %s (%s)\n", mf.Title, mf.Name))
			sb.WriteString(strings.TrimSpace(string(data)))
			sb.WriteString("\n\n---\n\n")
		}
	}

	if !hasContent {
		return "", nil
	}

	return sb.String(), nil
}

// RecordLesson inyecta una nueva lección aprendida en lessons.md para auto-aprendizaje del agente
func RecordLesson(projectRoot, findingType, cause, solution string) error {
	agentDir := filepath.Join(projectRoot, ".sdd", "agent")
	_ = os.MkdirAll(agentDir, 0755)

	lessonsFile := filepath.Join(agentDir, "lessons.md")
	if _, err := os.Stat(lessonsFile); os.IsNotExist(err) {
		initialHeader := "# Registro de Lecciones Aprendidas (Auto-Memoria Dinámica)\n\n"
		_ = os.WriteFile(lessonsFile, []byte(initialHeader), 0644)
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	cleanCause := strings.ReplaceAll(strings.TrimSpace(cause), "\n", " ")
	if len(cleanCause) > 120 {
		cleanCause = cleanCause[:120] + "..."
	}
	cleanSolution := strings.ReplaceAll(strings.TrimSpace(solution), "\n", " ")

	entry := fmt.Sprintf("- [%s] [%s] Fallo: %s | Solución: %s. No repetir este patrón.\n",
		timestamp, findingType, cleanCause, cleanSolution)

	f, err := os.OpenFile(lessonsFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error abriendo lessons.md: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("error escribiendo en lessons.md: %w", err)
	}

	return nil
}
