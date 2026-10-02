package doc

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"specforge/internal/adapters/storage"
)

// DocumentConverter gestiona la transformación de PDFs, Word y Excel a Markdown
type DocumentConverter struct{}

func NewDocumentConverter() *DocumentConverter {
	return &DocumentConverter{}
}

var supportedExtensions = map[string]bool{
	".pdf":  true,
	".docx": true,
	".xlsx": true,
	".xls":  true,
	".pptx": true,
	".csv":  true,
	".html": true,
}

func (c *DocumentConverter) IsSupported(ext string) bool {
	return supportedExtensions[strings.ToLower(ext)]
}

func (c *DocumentConverter) ConvertFile(sourceFile, targetFile string) (string, error) {
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		return "", fmt.Errorf("el archivo de origen %s no existe", sourceFile)
	}

	ext := strings.ToLower(filepath.Ext(sourceFile))
	if !c.IsSupported(ext) {
		return "", fmt.Errorf("formato '%s' no soportado para conversión a Markdown", ext)
	}

	if targetFile == "" {
		targetFile = strings.TrimSuffix(sourceFile, filepath.Ext(sourceFile)) + ".md"
	}

	logger := storage.GetLogger()
	logger.Info("Convirtiendo documento %s -> %s con markitdown...", sourceFile, targetFile)

	// 1. Intentar markitdown CLI o python -m markitdown
	cmdName := "markitdown"
	args := []string{sourceFile, "-o", targetFile}

	if _, err := exec.LookPath("markitdown"); err != nil {
		cmdName = "python"
		args = []string{"-m", "markitdown", sourceFile, "-o", targetFile}
	}

	cmd := exec.Command(cmdName, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("fallo convirtiendo %s a markdown: %w (stderr: %s)", sourceFile, err, stderrBuf.String())
	}

	return targetFile, nil
}

func (c *DocumentConverter) ConvertDirectory(dirPath string, recursive bool) ([]string, error) {
	var converted []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			base := info.Name()
			if base == "node_modules" || base == "target" || base == ".git" || base == ".specify" || base == ".sdd-cache" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if c.IsSupported(ext) && ext != ".md" {
			targetMd := strings.TrimSuffix(path, ext) + ".md"
			targetInfo, statErr := os.Stat(targetMd)
			if statErr != nil || info.ModTime().After(targetInfo.ModTime()) {
				if outPath, err := c.ConvertFile(path, targetMd); err == nil {
					converted = append(converted, outPath)
				}
			}
		}

		return nil
	})

	return converted, err
}
