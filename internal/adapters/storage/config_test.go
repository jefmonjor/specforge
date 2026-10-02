package storage

import (
	"os"
	"path/filepath"
	"testing"
	"specforge/internal/domain"
)

func TestConfigStorageSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-storage-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "config.json")
	store := NewConfigStorage(targetFile)

	if store.Exists() {
		t.Errorf("no debería existir antes de guardar")
	}

	cfg := domain.NewDefaultConfig()
	cfg.Agent = "gemini"
	cfg.Auth.Project = "test-project-123"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("error guardando config: %v", err)
	}

	if !store.Exists() {
		t.Errorf("debería existir tras guardar")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("error cargando config: %v", err)
	}

	if loaded.Auth.Project != "test-project-123" {
		t.Errorf("proyecto esperado 'test-project-123', obtenido: %s", loaded.Auth.Project)
	}
	if loaded.Environment["GOOGLE_CLOUD_PROJECT"] != "test-project-123" {
		t.Errorf("variable de entorno no poblada correctamente")
	}
}
