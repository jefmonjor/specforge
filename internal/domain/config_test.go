package domain

import (
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.Version != "3.0.0" {
		t.Errorf("versión esperada 3.0.0, obtenida: %s", cfg.Version)
	}
	if cfg.Agent != "gemini" {
		t.Errorf("agente por defecto esperado 'gemini', obtenido: %s", cfg.Agent)
	}
	if !cfg.Auth.UseVertexAI {
		t.Errorf("UseVertexAI debería ser true por defecto")
	}

	cfg.PopulateEnvMap()
	if cfg.Environment["GOOGLE_GENAI_USE_VERTEXAI"] != "true" {
		t.Errorf("GOOGLE_GENAI_USE_VERTEXAI no poblado")
	}
	if cfg.Environment["GOOGLE_CLOUD_PROJECT"] != "default-project" {
		t.Errorf("GOOGLE_CLOUD_PROJECT incorrecto")
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := NewDefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("configuración por defecto debería ser válida: %v", err)
	}

	cfg.Agent = "invalid_agent"
	if err := cfg.Validate(); err == nil {
		t.Errorf("se esperaba error con agente inválido")
	}

	cfg.Agent = "claude"
	cfg.Auth.UseVertexAI = true
	cfg.Auth.Project = ""
	if err := cfg.Validate(); err == nil {
		t.Errorf("se esperaba error con UseVertexAI activo sin Project")
	}
}
