package domain

// ActionType define el tipo de interacción en el navegador
type ActionType string

const (
	ActionClick     ActionType = "click"
	ActionTypeInput ActionType = "type"
	ActionAssert    ActionType = "assert"
	ActionWait      ActionType = "wait"
	ActionNavigate  ActionType = "navigate"
)

// Spec encapsula el contenido de una especificación BDD para pruebas E2E
type Spec struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// VisualAction modela la acción atómica tipada que decide el agente de IA
type VisualAction struct {
	ActionType     ActionType `json:"action_type"`
	TargetSelector string     `json:"target_selector,omitempty"`
	Value          string     `json:"value,omitempty"`
	Explanation    string     `json:"explanation,omitempty"`
	IsSuccess      bool       `json:"is_success,omitempty"`
}

// UIElement representa un elemento interactivo del DOM simplificado
type UIElement struct {
	ID          string `json:"id"`
	Tag         string `json:"tag"`
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	AriaLabel   string `json:"aria_label,omitempty"`
	Selector    string `json:"selector"`
}

// UISnapshot encapsula el estado visual y los elementos visibles de la página
type UISnapshot struct {
	URL      string      `json:"url"`
	Title    string      `json:"title"`
	Elements []UIElement `json:"elements"`
}
