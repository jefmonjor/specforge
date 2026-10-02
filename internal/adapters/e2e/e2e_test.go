package e2e

import (
	"strings"
	"testing"

	"specforge/internal/domain"
)

func TestParseVisualAction(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantType domain.ActionType
		wantSel  string
		wantVal  string
		wantOK   bool
	}{
		{
			name: "JSON puro de click",
			raw: `{
				"action_type": "click",
				"target_selector": "#submit-btn",
				"explanation": "Enviar formulario de reserva"
			}`,
			wantType: domain.ActionClick,
			wantSel:  "#submit-btn",
			wantOK:   false,
		},
		{
			name: "JSON con bloques markdown",
			raw: "```json\n" + `{
				"action_type": "type",
				"target_selector": "input[name=\"email\"]",
				"value": "cliente@banco.es",
				"explanation": "Ingresar correo"
			}` + "\n```",
			wantType: domain.ActionTypeInput,
			wantSel:  "input[name=\"email\"]",
			wantVal:  "cliente@banco.es",
			wantOK:   false,
		},
		{
			name: "Aserción positiva de éxito BDD",
			raw: `{
				"action_type": "assert",
				"is_success": true,
				"explanation": "El mensaje de confirmación 'Reserva Creada' es visible en pantalla"
			}`,
			wantType: domain.ActionAssert,
			wantOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := parseVisualAction(tt.raw)
			if err != nil {
				t.Fatalf("error inesperado parseando acción: %v", err)
			}
			if action.ActionType != tt.wantType {
				t.Errorf("ActionType = %v, esperado %v", action.ActionType, tt.wantType)
			}
			if action.TargetSelector != tt.wantSel {
				t.Errorf("TargetSelector = %v, esperado %v", action.TargetSelector, tt.wantSel)
			}
			if action.Value != tt.wantVal {
				t.Errorf("Value = %v, esperado %v", action.Value, tt.wantVal)
			}
			if action.IsSuccess != tt.wantOK {
				t.Errorf("IsSuccess = %v, esperado %v", action.IsSuccess, tt.wantOK)
			}
		})
	}
}

func TestBuildVisionPrompt(t *testing.T) {
	spec := &domain.Spec{
		Content: "Feature: Login bancario\nScenario: Acceso correcto con credenciales",
	}
	snapshot := &domain.UISnapshot{
		URL:   "http://localhost:3000/login",
		Title: "Portal Bancario",
		Elements: []domain.UIElement{
			{
				ID:       "elem_0",
				Tag:      "input",
				Type:     "text",
				Text:     "Usuario",
				Selector: "#user-id",
			},
			{
				ID:       "elem_1",
				Tag:      "button",
				Text:     "Entrar",
				Selector: "#btn-login",
			},
		},
	}
	history := []string{"Paso 0: Navegación inicial"}

	prompt := buildVisionPrompt(spec, snapshot, history)

	if !strings.Contains(prompt, "Feature: Login bancario") {
		t.Errorf("el prompt no contiene el escenario BDD")
	}
	if !strings.Contains(prompt, "Portal Bancario") {
		t.Errorf("el prompt no contiene el título de la página")
	}
	if !strings.Contains(prompt, "#btn-login") {
		t.Errorf("el prompt no contiene el selector del botón")
	}
}

func TestFindBrowserExecutable(t *testing.T) {
	path := findBrowserExecutable()
	t.Logf("Navegador detectado en el host: %s", path)
}
