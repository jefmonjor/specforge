package domain

import (
	"fmt"

	"specforge/internal/buildinfo"
)

// SDDConfig representa la configuración global de SpecForge almacenada en ~/.specforge/config.json
type SDDConfig struct {
	Version     string            `json:"version"`
	Agent       string            `json:"agent"` // "gemini" o "claude"
	Auth        AuthConfig        `json:"auth"`
	Repository  RepoConfig        `json:"repository"`
	SonarQube   SonarConfig       `json:"sonarqube,omitempty"`
	Environment map[string]string `json:"env"`
}

// AuthConfig define los parámetros de autenticación y proveedor LLM
type AuthConfig struct {
	Provider    string `json:"provider"`          // "gcp_adc", "api_key", etc.
	Project     string `json:"project"`           // Proyecto GCP (e.g. my-cloud-project)
	Location    string `json:"location"`          // Región GCP (e.g. europe-west1)
	UseVertexAI bool   `json:"use_vertex_ai"`     // Flag corporativo para Vertex AI
	HasADC      bool   `json:"has_adc"`           // Si se detectaron Application Default Credentials
	Account     string `json:"account,omitempty"` // Cuenta activa detectada
}

// RepoConfig define el origen y modo del baseline y artefactos
type RepoConfig struct {
	Mode           string `json:"mode"`                       // "nexus", "local", "file"
	NexusURL       string `json:"nexus_url,omitempty"`        // URL base de Nexus
	AIFrameworkURL string `json:"ai_framework_url,omitempty"` // URL o ruta del AI-Framework
}

// SonarConfig define la conexión a la plataforma de calidad SonarQube
type SonarConfig struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
}

// NewDefaultConfig crea una configuración inicial por defecto con valores recomendados
func NewDefaultConfig() *SDDConfig {
	return &SDDConfig{
		Version: buildinfo.Version,
		Agent:   "gemini",
		Auth: AuthConfig{
			Provider:    "gcp_adc",
			Project:     "default-project",
			Location:    "europe-west1",
			UseVertexAI: true,
			HasADC:      false,
		},
		Repository: RepoConfig{
			Mode: "local",
		},
		Environment: make(map[string]string),
	}
}

// PopulateEnvMap genera las variables de entorno para que procesos hijos y el script sdd.ps1 las hereden
func (c *SDDConfig) PopulateEnvMap() {
	if c.Environment == nil {
		c.Environment = make(map[string]string)
	}

	if c.Auth.UseVertexAI {
		c.Environment["GOOGLE_GENAI_USE_VERTEXAI"] = "true"
	}
	if c.Auth.Project != "" {
		c.Environment["GOOGLE_CLOUD_PROJECT"] = c.Auth.Project
	}
	if c.Auth.Location != "" {
		c.Environment["GOOGLE_CLOUD_LOCATION"] = c.Auth.Location
	}
	if c.Repository.NexusURL != "" {
		c.Environment["SDD_NEXUS_URL"] = c.Repository.NexusURL
	}
	if c.Repository.AIFrameworkURL != "" {
		c.Environment["SDD_AIFRAMEWORK_URL"] = c.Repository.AIFrameworkURL
	}
	if c.SonarQube.URL != "" {
		c.Environment["SONAR_HOST_URL"] = c.SonarQube.URL
		c.Environment["SONARQUBE_URL"] = c.SonarQube.URL
	}
}

// Validate verifica que los campos obligatorios contengan valores coherentes
func (c *SDDConfig) Validate() error {
	if c.Agent != "gemini" && c.Agent != "claude" {
		return fmt.Errorf("agente no soportado: '%s' (opciones: 'gemini', 'claude')", c.Agent)
	}
	if c.Auth.UseVertexAI && c.Auth.Project == "" {
		return fmt.Errorf("se requiere un proyecto de GCP cuando use_vertex_ai está activo")
	}
	return nil
}
