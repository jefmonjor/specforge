package ingest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/domain"
)

// ProjectScanner implementa la recolección rápida de contexto de repositorio
type ProjectScanner struct {
	MaxSnippetBytes int
	MaxSnippetLines int
}

func NewProjectScanner() *ProjectScanner {
	return &ProjectScanner{
		MaxSnippetBytes: 4096,
		MaxSnippetLines: 80,
	}
}

// ScanProject recorre el árbol respetando exclusiones y genera el ProjectContext
func (s *ProjectScanner) ScanProject(rootDir string) (*domain.ProjectContext, error) {
	projInfo := domain.DetectProject(rootDir)
	filter := domain.NewDefaultIgnoreFilter(rootDir)

	ctx := &domain.ProjectContext{
		ProjectRoot: rootDir,
		ProjectInfo: projInfo,
		Files:       make([]domain.FileEntry, 0),
	}

	var treeBuilder strings.Builder

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil || relPath == "." {
			return nil
		}

		isDir := d.IsDir()

		if filter.ShouldIgnore(relPath, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		// Construir árbol visual
		depth := strings.Count(filepath.ToSlash(relPath), "/")
		indent := strings.Repeat("  ", depth)
		if isDir {
			treeBuilder.WriteString(fmt.Sprintf("%s📁 %s/\n", indent, d.Name()))
			return nil
		}

		treeBuilder.WriteString(fmt.Sprintf("%s📄 %s\n", indent, d.Name()))

		info, err := d.Info()
		if err != nil {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		isKey := isKeyFile(relPath, ext)

		entry := domain.FileEntry{
			RelativePath: relPath,
			Size:         info.Size(),
			Extension:    ext,
			IsKeyFile:    isKey,
		}

		if isKey && info.Size() > 0 && info.Size() < 500000 {
			entry.Snippet = s.readSnippet(path)
		}

		ctx.Files = append(ctx.Files, entry)
		ctx.TotalFiles++
		ctx.TotalSize += info.Size()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error escaneando proyecto: %w", err)
	}

	ctx.DirectoryTree = treeBuilder.String()
	return ctx, nil
}

func (s *ProjectScanner) readSnippet(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > s.MaxSnippetLines {
		lines = lines[:s.MaxSnippetLines]
		lines = append(lines, "... [contenido truncado para contexto]")
	}

	snippet := strings.Join(lines, "\n")
	if len(snippet) > s.MaxSnippetBytes {
		snippet = snippet[:s.MaxSnippetBytes] + "\n... [bytes truncados]"
	}

	return snippet
}

func isKeyFile(relPath, ext string) bool {
	lower := strings.ToLower(relPath)
	base := filepath.Base(lower)

	// Archivos de configuración o definición clave
	keyNames := []string{
		"pom.xml", "package.json", "go.mod", "pyproject.toml",
		"requirements.txt", "readme.md", "dockerfile", "app.yaml",
		"settings.json", "spec.md", "manifest.json",
	}

	for _, k := range keyNames {
		if base == k {
			return true
		}
	}

	// Archivos de código fuente principales
	sourceExts := []string{".go", ".java", ".ts", ".tsx", ".py", ".cbl", ".cob", ".rpgle"}
	for _, se := range sourceExts {
		if ext == se {
			if strings.Contains(lower, "main") || strings.Contains(lower, "controller") ||
				strings.Contains(lower, "service") || strings.Contains(lower, "api") ||
				strings.Contains(lower, "app") || strings.Contains(lower, "index") {
				return true
			}
		}
	}

	return false
}
