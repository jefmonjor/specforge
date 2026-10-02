package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileEntry representa un archivo relevante del proyecto para el contexto
type FileEntry struct {
	RelativePath string
	Size         int64
	Extension    string
	IsKeyFile    bool
	Snippet      string
}

// ProjectContext reúne el mapa del proyecto, árbol de archivos y resumen para el LLM
type ProjectContext struct {
	ProjectRoot string
	ProjectInfo ProjectInfo
	TotalFiles  int
	TotalSize   int64
	Files       []FileEntry
	DirectoryTree string
}

// IgnoreFilter gestiona las exclusiones de archivos respetando .gitignore, .geminiignore y defaults
type IgnoreFilter struct {
	patterns []string
}

// NewDefaultIgnoreFilter inicializa un filtro con exclusiones industriales estándar
func NewDefaultIgnoreFilter(rootDir string) *IgnoreFilter {
	filter := &IgnoreFilter{
		patterns: []string{
			"node_modules",
			"target",
			".git",
			".sdd-cache",
			".specify",
			"bin",
			"obj",
			"dist",
			"vendor",
			".idea",
			".vscode",
			"*.pyc",
			"__pycache__",
			"*.exe",
			"*.tar.gz",
			"*.zip",
			"*.log",
			".DS_Store",
		},
	}

	filter.loadIgnoreFile(filepath.Join(rootDir, ".gitignore"))
	filter.loadIgnoreFile(filepath.Join(rootDir, ".geminiignore"))

	return filter
}

func (f *IgnoreFilter) loadIgnoreFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			continue
		}
		clean := strings.TrimSuffix(trimmed, "/")
		clean = strings.TrimPrefix(clean, "/")
		if clean != "" {
			f.patterns = append(f.patterns, clean)
		}
	}
}

// ShouldIgnore determina si una ruta o directorio debe ser excluido
func (f *IgnoreFilter) ShouldIgnore(relPath string, isDir bool) bool {
	normalized := filepath.ToSlash(relPath)
	parts := strings.Split(normalized, "/")

	for _, part := range parts {
		for _, pattern := range f.patterns {
			if strings.EqualFold(part, pattern) {
				return true
			}
			matched, err := filepath.Match(pattern, part)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// RenderMarkdown formatea el contexto estructurado en Markdown listo para prompts del LLM
func (pc *ProjectContext) RenderMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Contexto del Proyecto (CodeGraph As-Is)\n\n")
	sb.WriteString(fmt.Sprintf("- **Stack Detectado:** %s\n", pc.ProjectInfo.Type))
	sb.WriteString(fmt.Sprintf("- **Total Archivos Relevantes:** %d\n", pc.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **Tamaño Total Código:** %.2f KB\n\n", float64(pc.TotalSize)/1024.0))

	sb.WriteString("## Árbol de Directorios\n```plaintext\n")
	sb.WriteString(pc.DirectoryTree)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Archivos y Contratos Clave\n\n")
	for _, f := range pc.Files {
		if f.IsKeyFile && f.Snippet != "" {
			sb.WriteString(fmt.Sprintf("### `%s`\n```\n%s\n```\n\n", f.RelativePath, f.Snippet))
		}
	}

	return sb.String()
}
