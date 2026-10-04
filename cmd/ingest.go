package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/doc"
	"specforge/internal/adapters/ingest"
	"specforge/internal/adapters/storage"
)

var (
	flagIngestOutput    string
	flagIngestSync      bool
	flagIngestPrint     bool
	flagIngestNoConvert bool
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingesta As-Is y generación del mapa de contexto ultrarrápido (CodeGraph)",
	Long: `Recorre el árbol de archivos del repositorio a máxima velocidad ignorando
dependencias y directorios pesados (node_modules, target, .git, .sdd-cache),
detecta los contratos y archivos clave, convierte automáticamente cualquier documento
heterogéneo (PDF, Excel, Word) a Markdown y genera docs/_context/context-pack.md.`,
	RunE: runIngest,
}

func init() {
	ingestCmd.Flags().StringVarP(&flagIngestOutput, "output", "o", "", "ruta personalizada para guardar el context-pack")
	ingestCmd.Flags().BoolVar(&flagIngestSync, "sync", false, "sincronizar índice CodeGraph local si está instalado")
	ingestCmd.Flags().BoolVar(&flagIngestPrint, "print", false, "imprimir el volcado de contexto por consola")
	ingestCmd.Flags().BoolVar(&flagIngestNoConvert, "no-convert", false, "omitir la conversión automática de documentos Office/PDF a Markdown")

	rootCmd.AddCommand(ingestCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	start := time.Now()
	fmt.Println("==================================================================")
	fmt.Println("🚀 SpecForge — Ingesta As-Is y Motor de Contexto")
	fmt.Println("==================================================================")

	// Conversión automática por defecto de PDFs, Word y Excel a Markdown
	if !flagIngestNoConvert {
		converter := doc.NewDocumentConverter()
		converted, err := converter.ConvertDirectory(cwd, true)
		if err == nil && len(converted) > 0 {
			fmt.Printf("\n[0/3] Conversión automática de documentos a Markdown (MarkItDown)...\n")
			fmt.Printf("  ✓ %d documento(s) (PDF/Excel/Word) convertidos a .md de forma desatendida:\n", len(converted))
			for _, f := range converted {
				rel, _ := filepath.Rel(cwd, f)
				fmt.Printf("    • %s\n", rel)
			}
		}
	}

	fmt.Println("\n[1/3] Escaneando repositorio a alta velocidad...")
	scanner := ingest.NewProjectScanner()
	projCtx, err := scanner.ScanProject(cwd)
	if err != nil {
		return fmt.Errorf("fallo durante el escaneo: %w", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("  ✓ Escaneo completado en %v\n", elapsed)
	fmt.Printf("  ✓ Stack detectado:          %s\n", projCtx.ProjectInfo.Type)
	fmt.Printf("  ✓ Archivos analizados:      %d\n", projCtx.TotalFiles)
	fmt.Printf("  ✓ Volumen de código:        %.2f KB\n", float64(projCtx.TotalSize)/1024.0)

	// 2. Generar volcado Markdown del context-pack
	fmt.Println("\n[2/3] Generando volcado estructurado del proyecto...")
	mdContent := projCtx.RenderMarkdown()

	targetFile := flagIngestOutput
	if targetFile == "" {
		contextDir := filepath.Join(cwd, "docs", "_context")
		_ = os.MkdirAll(contextDir, 0755)
		targetFile = filepath.Join(contextDir, "context-pack.md")
	}

	if err := os.WriteFile(targetFile, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("error guardando context pack en %s: %w", targetFile, err)
	}
	fmt.Printf("  ✓ Context-Pack guardado en: %s\n", targetFile)

	// 3. Generar respuestas y brief de ingesta en docs/_ingest/ si faltan
	ingestDir := filepath.Join(cwd, "docs", "_ingest")
	_ = os.MkdirAll(ingestDir, 0755)

	answersFile := filepath.Join(ingestDir, "answers.json")
	if _, err := os.Stat(answersFile); os.IsNotExist(err) {
		answers := map[string]interface{}{
			"module":         filepath.Base(cwd),
			"language":       string(projCtx.ProjectInfo.Type),
			"classification": "classified",
			"generated_at":   time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.MarshalIndent(answers, "", "  ")
		_ = os.WriteFile(answersFile, data, 0644)
		fmt.Printf("  ✓ Archivo de gobierno generado: %s\n", answersFile)
	}

	// 4. Sincronización CodeGraph si se solicitó o existe .codegraph/
	if flagIngestSync || fileExists(filepath.Join(cwd, ".codegraph")) {
		fmt.Println("\n[3/3] Sincronizando índice semántico CodeGraph...")
		syncCmd := exec.Command("codegraph", "sync")
		syncCmd.Dir = cwd
		if out, err := syncCmd.CombinedOutput(); err == nil {
			fmt.Println("  ✓ CodeGraph sincronizado con éxito.")
		} else {
			logger.Warn("Aviso: codegraph sync no pudo ejecutarse: %v (salida: %s)", err, string(out))
			fmt.Println("  ⚠️ codegraph sync omitido (CLI no disponible en el entorno).")
		}
	} else {
		fmt.Println("\n[3/3] Índice semántico CodeGraph omitido (usando mapa nativo de Go).")
	}

	if flagIngestPrint {
		fmt.Println("\n--- Volcado de Contexto ---")
		fmt.Println(mdContent)
	}

	fmt.Println("\n==================================================================")
	fmt.Println("✓ Ingesta completada con éxito.")
	fmt.Println("  El context-pack está listo para alimentar las sesiones de IA.")
	fmt.Println("==================================================================")

	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
