package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagConsistencyFailOn string
)

var consistencyCmd = &cobra.Command{
	Use:   "consistency",
	Short: "Ejecuta los gates deterministas de consistencia y anti-contaminación",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error obteniendo directorio actual: %v\n", err)
			os.Exit(1)
		}

		exitCode := runConsistencyChecks(cwd)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	},
}

func init() {
	consistencyCmd.Flags().StringVar(&flagConsistencyFailOn, "FailOn", "blocker", "nivel de fallo ('blocker' o 'warning')")
	rootCmd.AddCommand(consistencyCmd)
}

func runConsistencyChecks(projDir string) int {
	pomPath := filepath.Join(projDir, "pom.xml")
	answersPath := filepath.Join(projDir, "docs", "_ingest", "answers.json")

	// 1. Verificar si answers.json fue borrado teniendo historial en Git
	if !fileExists(answersPath) {
		cmd := exec.Command("git", "log", "-n", "1", "--oneline", "--", "docs/_ingest/answers.json")
		cmd.Dir = projDir
		if out, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			fmt.Println("[consistency] BLOCKER: El archivo docs/_ingest/answers.json ha sido borrado ilegalmente")
			return 6
		}
		// Si no existe pero hay pom con javax
		if fileExists(pomPath) {
			data, _ := os.ReadFile(pomPath)
			if strings.Contains(string(data), "javax.") {
				fmt.Println("[consistency] BLOCKER: Falta docs/_ingest/answers.json en proyecto java-legacy")
				return 6
			}
		}
		return 0
	}

	// 2. Verificar si answers.json fue manipulado respecto a HEAD en Git
	gitShowCmd := exec.Command("git", "show", "HEAD:docs/_ingest/answers.json")
	gitShowCmd.Dir = projDir
	if headOut, err := gitShowCmd.Output(); err == nil && len(headOut) > 0 {
		var headAns map[string]interface{}
		_ = json.Unmarshal(headOut, &headAns)

		currData, _ := os.ReadFile(answersPath)
		var currAns map[string]interface{}
		_ = json.Unmarshal(currData, &currAns)

		headLang, _ := headAns["language"].(string)
		currLang, _ := currAns["language"].(string)

		if headLang != "" && currLang != "" && headLang != currLang {
			fmt.Printf("[consistency] BLOCKER: answers.json alterado (%s -> %s)\n", headLang, currLang)
			return 6
		}
	}

	// 3. Leer clasificación actual
	answersData, err := os.ReadFile(answersPath)
	if err != nil {
		return 6
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(answersData, &parsed); err != nil {
		return 6
	}

	lang, _ := parsed["language"].(string)

	// Si no es java-legacy, el gate de anti-contaminación legacy no aplica
	if lang != "java-legacy" {
		return 0
	}

	// 4. Si es java-legacy, comprobar pom.xml
	if !fileExists(pomPath) {
		return 0
	}
	pomData, err := os.ReadFile(pomPath)
	if err != nil {
		return 0
	}
	pomContent := string(pomData)

	// Comprobar imports jakarta en archivos Java
	javaFiles := findJavaFiles(filepath.Join(projDir, "src", "main", "java"))
	jakartaRegex := regexp.MustCompile(`import\s+jakarta\.(servlet|annotation|persistence|validation|ws\.rs|inject)\b`)
	for _, jf := range javaFiles {
		content, err := os.ReadFile(jf)
		if err == nil && jakartaRegex.Match(content) {
			fmt.Printf("[consistency] BLOCKER: Contaminación de import jakarta en %s\n", jf)
			return 6
		}
	}

	// Comprobar dependencias spring-boot en pom.xml
	if strings.Contains(pomContent, "<artifactId>spring-boot-") {
		fmt.Println("[consistency] BLOCKER: Inclusión ilegal de dependencia spring-boot en pom.xml")
		return 6
	}

	// Comprobar compilador Java >= 17
	compilerRegex := regexp.MustCompile(`<(maven\.compiler\.target|maven\.compiler\.source|java\.version|release)>\s*([^<]+?)\s*</`)
	matches := compilerRegex.FindAllStringSubmatch(pomContent, -1)
	for _, m := range matches {
		if len(m) > 2 {
			ver := strings.TrimSpace(m[2])
			if strings.HasPrefix(ver, "${") {
				continue // Dinámico: warning, no bloquea
			}
			if num, err := strconv.Atoi(ver); err == nil {
				if num >= 17 {
					fmt.Printf("[consistency] BLOCKER: Compilador moderno Java %d (>= 17) no permitido en legacy\n", num)
					return 6
				}
			}
		}
	}

	return 0
}

func findJavaFiles(dir string) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".java") {
			files = append(files, path)
		}
		return nil
	})
	return files
}
