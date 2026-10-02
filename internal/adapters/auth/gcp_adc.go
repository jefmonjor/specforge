package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/oauth2/google"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

type GCPADCDetector struct{}

func NewGCPADCDetector() ports.ADCDetector {
	return &GCPADCDetector{}
}

type adcJSON struct {
	ClientID       string `json:"client_id"`
	QuotaProjectID string `json:"quota_project_id"`
	Type           string `json:"type"`
	Account        string `json:"account"`
}

func (d *GCPADCDetector) DetectCredentials(ctx context.Context) (*domain.AuthConfig, error) {
	authCfg := &domain.AuthConfig{
		Provider:    "gcp_adc",
		Location:    "europe-west1",
		UseVertexAI: true,
		HasADC:      false,
	}

	// 1. Localizar fichero de credenciales por defecto de gcloud
	adcPath := getADCFilePath()
	if adcPath != "" {
		if data, err := os.ReadFile(adcPath); err == nil {
			authCfg.HasADC = true
			var parsed adcJSON
			if err := json.Unmarshal(data, &parsed); err == nil {
				if parsed.QuotaProjectID != "" {
					authCfg.Project = parsed.QuotaProjectID
				}
				if parsed.Account != "" {
					authCfg.Account = parsed.Account
				}
			}
		}
	}

	// 2. Usar SDK oficial de Google para validar
	creds, err := google.FindDefaultCredentials(ctx)
	if err == nil && creds != nil {
		authCfg.HasADC = true
		if creds.ProjectID != "" && authCfg.Project == "" {
			authCfg.Project = creds.ProjectID
		}
	}

	// 3. Fallback a gcloud CLI si el proyecto no vino en el fichero
	if authCfg.Project == "" || authCfg.Account == "" {
		if proj := getGCloudConfigValue("project"); proj != "" {
			authCfg.Project = proj
		}
		if acc := getGCloudConfigValue("account"); acc != "" {
			authCfg.Account = acc
		}
	}

	// 4. Default si se detectó ADC pero no el proyecto específico
	if authCfg.Project == "" {
		authCfg.Project = "default-project"
	}

	return authCfg, nil
}

func (d *GCPADCDetector) GetAccessToken(ctx context.Context) (string, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("no se encontraron credenciales ADC válidas: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("error obteniendo token de acceso: %w", err)
	}

	return token.AccessToken, nil
}

func getADCFilePath() string {
	if custom := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); custom != "" {
		return custom
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			p := filepath.Join(appData, "gcloud", "application_default_credentials.json")
			if fileExists(p) {
				return p
			}
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
		if fileExists(p) {
			return p
		}
	}

	return ""
}

func getGCloudConfigValue(key string) string {
	cmd := exec.Command("gcloud", "config", "get-value", key)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	val := strings.TrimSpace(string(out))
	if strings.HasPrefix(val, "(") || val == "" {
		return ""
	}
	lines := strings.Split(val, "\n")
	return strings.TrimSpace(lines[0])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
