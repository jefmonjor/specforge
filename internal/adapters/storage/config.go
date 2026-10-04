package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"specforge/internal/domain"
)

// ConfigStorage implementa la persistencia de configuración en ~/.specforge/config.json
type ConfigStorage struct {
	customPath string
}

// NewConfigStorage crea una nueva instancia del gestor de configuración
func NewConfigStorage(customPath string) *ConfigStorage {
	return &ConfigStorage{customPath: customPath}
}

// GetConfigPath resuelve la ruta absoluta al fichero de configuración
func (s *ConfigStorage) GetConfigPath() (string, error) {
	if s.customPath != "" {
		return s.customPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error obteniendo directorio home del usuario: %w", err)
	}
	return filepath.Join(home, ".specforge", "config.json"), nil
}

// Load lee y deserializa el fichero de configuración
func (s *ConfigStorage) Load() (*domain.SDDConfig, error) {
	path, err := s.GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no existe configuración en %s. Ejecuta 'sdd init'", path)
		}
		return nil, fmt.Errorf("error leyendo %s: %w", path, err)
	}

	var cfg domain.SDDConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parseando JSON de %s: %w", path, err)
	}

	return &cfg, nil
}

// Save serializa y escribe la configuración en disco
func (s *ConfigStorage) Save(cfg *domain.SDDConfig) error {
	path, err := s.GetConfigPath()
	if err != nil {
		return err
	}

	cfg.PopulateEnvMap()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creando directorio %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando configuración: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error guardando archivo %s: %w", path, err)
	}

	return nil
}

// Exists comprueba si el archivo de configuración ya existe
func (s *ConfigStorage) Exists() bool {
	path, err := s.GetConfigPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
