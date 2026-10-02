package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/doc"
	"specforge/internal/adapters/storage"
)

var (
	flagDocOutput    string
	flagDocRecursive bool
)

var docCmd = &cobra.Command{
	Use:   "doc [archivo-o-carpeta]",
	Short: "Convierte documentos (PDF, Word, Excel .xlsx) a Markdown con MarkItDown",
	Long: `Transforma documentos heterogéneos (PDFs de arquitectura, hojas de cálculo Excel
con fórmulas de conciliación, y documentos Word) a formato Markdown limpio y estructurado
utilizando Microsoft MarkItDown. Permite que el motor SDD y los agentes de IA comprendan
fórmulas financieras y tablas de datos complejas.`,
	RunE: runDoc,
}

func init() {
	docCmd.Flags().StringVarP(&flagDocOutput, "output", "o", "", "archivo o carpeta destino para el markdown resultante")
	docCmd.Flags().BoolVarP(&flagDocRecursive, "recursive", "r", true, "convertir recursivamente en subdirectorios")

	rootCmd.AddCommand(docCmd)
}

func runDoc(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("no se encontró la ruta especificada: %w", err)
	}

	converter := doc.NewDocumentConverter()
	fmt.Println("==================================================================")
	fmt.Println("📄 SpecForge v3.0 — Conversor de Documentos a Markdown")
	fmt.Println("==================================================================")
	fmt.Println("  ✓ Motor: Microsoft MarkItDown (PDF, Excel .xlsx, Word .docx)")

	if !info.IsDir() {
		fmt.Printf("\n[1/1] Convirtiendo archivo único: %s\n", targetPath)
		outMd, err := converter.ConvertFile(targetPath, flagDocOutput)
		if err != nil {
			return err
		}
		fmt.Printf("  ✓ Documento convertido con éxito:\n    %s\n", outMd)
		logger.Info("Documento convertido: %s -> %s", targetPath, outMd)
		return nil
	}

	fmt.Printf("\n[1/1] Escaneando directorio: %s\n", targetPath)
	converted, err := converter.ConvertDirectory(targetPath, flagDocRecursive)
	if err != nil {
		logger.Warn("Aviso durante la conversión: %v", err)
	}

	if len(converted) == 0 {
		fmt.Println("  ℹ️ No se encontraron documentos pendientes de conversión (.pdf, .docx, .xlsx).")
	} else {
		fmt.Printf("  ✓ %d documento(s) convertidos a Markdown exitosamente:\n", len(converted))
		for _, f := range converted {
			rel, _ := filepath.Rel(".", f)
			fmt.Printf("    • %s\n", rel)
		}
	}

	fmt.Println("\n==================================================================")
	fmt.Println("✓ Conversión finalizada. Los archivos .md están listos para ingesta y BDD.")
	fmt.Println("==================================================================")

	return nil
}
