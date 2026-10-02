package domain

import (
	"fmt"
	"os"
	"strings"
)

// DiagnosticReport contiene el análisis contextual y los pasos de resolución para el desarrollador
type DiagnosticReport struct {
	Title       string
	Cause       string
	Explanation string
	Action      string
	ManualCmd   string
	ResumeCmd   string
	DocsRef     string
}

// DiagnoseError analiza la traza de error y genera una guía de remediación a medida
func DiagnoseError(err error) *DiagnosticReport {
	if err == nil {
		return nil
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// 1. Código muerto / Knip
	if strings.Contains(lower, "knip violation") || strings.Contains(lower, "dead code or unused dependencies") {
		return &DiagnosticReport{
			Title:       "Knip: Código Muerto o Dependencias Huérfanas",
			Cause:       "Archivos, funciones exportadas o paquetes en package.json sin uso detectados.",
			Explanation: "El analizador Knip detectó que la implementación introdujo código no referenciado o dependencias innecesarias.",
			Action:      "Elimina los archivos no utilizados, limpia las exportaciones huérfanas o desinstala las librerías muertas.",
			ManualCmd:   "npx knip",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§8)",
		}
	}

	// 2. Duplicación DRY / jscpd
	if strings.Contains(lower, "dry violation") || strings.Contains(lower, "jscpd") || strings.Contains(lower, "clone found") {
		return &DiagnosticReport{
			Title:       "jscpd: Violación del Principio DRY (Código Duplicado)",
			Cause:       "Se detectaron bloques de código duplicados que superan el umbral corporativo (0%).",
			Explanation: "El framework prohíbe duplicar lógica entre archivos o funciones para garantizar mantenibilidad.",
			Action:      "Extrae la lógica común a una función compartida, servicio o componente reutilizable.",
			ManualCmd:   "npx jscpd ./src --threshold 0",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§8)",
		}
	}

	// 3. Mutation Testing / Stryker
	if strings.Contains(lower, "mutation testing failed") || strings.Contains(lower, "stryker") || strings.Contains(lower, "surviving mutants") {
		return &DiagnosticReport{
			Title:       "Stryker: Mutantes Supervivientes (Tests Débiles)",
			Cause:       "Mutantes de código sobrevivieron a la suite de pruebas unitarias.",
			Explanation: "Stryker mutó operadores lógicos y los tests continuaron pasando en verde (falsos positivos).",
			Action:      "Refuerza las pruebas unitarias añadiendo aserciones estrictas para los casos límite identificados.",
			ManualCmd:   "npx stryker run",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§8)",
		}
	}

	// 4. Violación YAGNI
	if strings.Contains(lower, "yagni violation") {
		return &DiagnosticReport{
			Title:       "YAGNI: Test en Verde Prematuro en Fase RED",
			Cause:       "La prueba unitaria generada pasó con éxito antes de escribir la implementación.",
			Explanation: "En TDD, una prueba debe demostrar primero su capacidad de fallar contra el requerimiento.",
			Action:      "Revisa la prueba unitaria: asegura que la aserción evalúa lógica aún no implementada.",
			ManualCmd:   "sdd loop",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§7)",
		}
	}

	// 5. Violación de Integridad / Manipulación de Spec
	if strings.Contains(lower, "la especificación ha sido modificada manualmente") || strings.Contains(lower, "sello criptográfico") {
		return &DiagnosticReport{
			Title:       "Integridad Criptográfica: Especificación BDD Alterada",
			Cause:       "El archivo spec.md en disco no coincide con el hash SHA-256 sellado en el estado.",
			Explanation: "El framework protege la especificación contra cambios accidentales o manuales no validados.",
			Action:      "Si los cambios son intencionados, ejecuta 'sdd interview' para resellar. Si no, revierte spec.md.",
			ManualCmd:   "sdd interview",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§6)",
		}
	}

	// 6. Protección de Ramas
	if strings.Contains(lower, "rama master") || strings.Contains(lower, "rama de feature") {
		return &DiagnosticReport{
			Title:       "Protección de Ramas: Ejecución Bloqueada en Master",
			Cause:       "Intentaste ejecutar un comando destructivo o de setup directamente en master/main.",
			Explanation: "El SDDFramework exige trabajar siempre en ramas de feature para garantizar revisiones por PR.",
			Action:      "Crea una rama de trabajo: git checkout -b feature/nombre-de-tu-funcionalidad",
			ManualCmd:   "git checkout -b feature/<nombre>",
			ResumeCmd:   "sdd setup",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§2)",
		}
	}

	// 7. Límite de Reintentos en GREEN
	if strings.Contains(lower, "límite de 3 reintentos") || strings.Contains(lower, "intento 3/3") {
		return &DiagnosticReport{
			Title:       "Límite de Reintentos Alcanzado en Fase GREEN (3/3)",
			Cause:       "La IA no consiguió poner los tests en verde tras 3 intentos sucesivos.",
			Explanation: "El framework pausa automáticamente para evitar consumo innecesario de tokens del modelo.",
			Action:      "Revisa la traza del compilador en ~/.sdd/logs/, corrige el código manualmente y reanuda.",
			ManualCmd:   "Get-Content ~/.sdd/logs/sdd-*.log -Tail 50",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§11)",
		}
	}

	// 8. Clean Architecture / ArchUnit
	if strings.Contains(lower, "archunit violation") || strings.Contains(lower, "arquitectura hexagonal") {
		return &DiagnosticReport{
			Title:       "ArchUnit: Violación de Clean Architecture o Capas",
			Cause:       "Se detectaron dependencias o imports prohibidos entre capas del software.",
			Explanation: "Las capas internas (dominio/negocio) no pueden acoplarse directamente a infraestructura o frameworks.",
			Action:      "Invierte la dependencia mediante interfaces/puertos o desacopla los imports ilegales.",
			ManualCmd:   "mvn test -Dtest=*ArchitectureTest*",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§8)",
		}
	}

	// 9. Linter Estricto
	if strings.Contains(lower, "linter violation") {
		return &DiagnosticReport{
			Title:       "Linter: Violación de Estándares de Estilo o Convenciones",
			Cause:       "El análisis estático detectó errores de estilo, tipos o variables sin usar.",
			Explanation: "El código no cumple con los estándares exigidos para el stack tecnológico.",
			Action:      "Corrige las líneas señaladas en el reporte o ejecuta el autofix de tu linter.",
			ManualCmd:   "npm run lint -- --fix  (o go vet ./...)",
			ResumeCmd:   "sdd loop --resume",
			DocsRef:     "docs/GUIA_DE_USO_V3.md (§8)",
		}
	}

	// 10. Fallo genérico con diagnóstico básico
	return &DiagnosticReport{
		Title:       "Interrupción en la Ejecución",
		Cause:       msg,
		Explanation: "El proceso se detuvo. El estado previo se encuentra protegido en .sdd-state.json.",
		Action:      "Revisa las trazas de depuración o los logs silenciosos en ~/.sdd/logs/.",
		ManualCmd:   "sdd loop --debug",
		ResumeCmd:   "sdd loop --resume",
		DocsRef:     "docs/GUIA_DE_USO_V3.md",
	}
}

// PrintDiagnosticBox imprime en la terminal una caja visual con el diagnóstico guiado
func PrintDiagnosticBox(diag *DiagnosticReport) {
	if diag == nil {
		return
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "==================================================================")
	fmt.Fprintf(os.Stderr, "🩺 DIAGNÓSTICO AUTOMÁTICO Y GUÍA DE REMEDIACIÓN\n")
	fmt.Fprintln(os.Stderr, "==================================================================")
	fmt.Fprintf(os.Stderr, "  • Tipo de Hallazgo: %s\n", diag.Title)
	fmt.Fprintf(os.Stderr, "  • Causa:            %s\n", diag.Cause)
	fmt.Fprintf(os.Stderr, "  • Explicación:      %s\n", diag.Explanation)
	fmt.Fprintf(os.Stderr, "  • Acción Inmediata: %s\n", diag.Action)
	if diag.ManualCmd != "" {
		fmt.Fprintf(os.Stderr, "  • Comando de Test:  %s\n", diag.ManualCmd)
	}
	if diag.ResumeCmd != "" {
		fmt.Fprintf(os.Stderr, "  • Para Reanudar:    %s\n", diag.ResumeCmd)
	}
	if diag.DocsRef != "" {
		fmt.Fprintf(os.Stderr, "  • Documentación:    %s\n", diag.DocsRef)
	}
	fmt.Fprintln(os.Stderr, "==================================================================")
}
