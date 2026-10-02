package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:baseline
var BaselineFS embed.FS

// ExtractDir extrae recursivamente un subdirectorio del baseline embebido al directorio destino
func ExtractDir(subDir, targetDir string, overwrite bool) error {
	cleanSub := filepath.ToSlash(subDir)
	var prefix string
	if cleanSub == "" || cleanSub == "." {
		prefix = "baseline"
	} else {
		prefix = filepath.ToSlash(filepath.Join("baseline", cleanSub))
	}

	return fs.WalkDir(BaselineFS, prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(prefix, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if !overwrite {
			if _, err := os.Stat(destPath); err == nil {
				return nil // Mantener archivo preexistente si overwrite es falso
			}
		}

		data, err := BaselineFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error leyendo archivo embebido %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

// ReadEmbeddedFile lee el contenido de un archivo embebido dado su path relativo a baseline
func ReadEmbeddedFile(relativePath string) ([]byte, error) {
	fullPath := filepath.ToSlash(filepath.Join("baseline", relativePath))
	return BaselineFS.ReadFile(fullPath)
}

// GetBaselineVersion devuelve la versión del baseline embebida en VERSION
func GetBaselineVersion() string {
	data, err := ReadEmbeddedFile("VERSION")
	if err != nil {
		return "3.0.0"
	}
	return strings.TrimSpace(string(data))
}
