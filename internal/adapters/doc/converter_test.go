package doc

import "testing"

func TestDocumentConverterIsSupported(t *testing.T) {
	c := NewDocumentConverter()

	supported := []string{".pdf", ".docx", ".xlsx", ".pptx", ".csv", ".html"}
	for _, ext := range supported {
		if !c.IsSupported(ext) {
			t.Errorf("la extensión %s debería estar soportada", ext)
		}
	}

	unsupported := []string{".exe", ".bin", ".dll", ".zip"}
	for _, ext := range unsupported {
		if c.IsSupported(ext) {
			t.Errorf("la extensión %s NO debería estar soportada", ext)
		}
	}
}
