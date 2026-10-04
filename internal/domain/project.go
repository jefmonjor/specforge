package domain

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectType representa la familia o tecnología principal del proyecto
type ProjectType string

const (
	ProjectJavaSpring ProjectType = "java-spring"
	ProjectJavaLegacy ProjectType = "java-legacy"
	ProjectReact      ProjectType = "react"
	ProjectAngular    ProjectType = "angular"
	ProjectNode       ProjectType = "node"
	ProjectPython     ProjectType = "python"
	ProjectGo         ProjectType = "go"
	ProjectUnknown    ProjectType = "unknown"
)

// ProjectInfo consolida los metadatos tecnológicos del repositorio
type ProjectInfo struct {
	Type         ProjectType `json:"type"`
	RootPath     string      `json:"root_path"`
	StandardDoc  string      `json:"standard_doc,omitempty"`
	RequiresArch bool        `json:"requires_arch"`
	IsJVM        bool        `json:"is_jvm"`
}

// DetectProject analiza la raíz del proyecto para clasificar su tecnología
func DetectProject(rootDir string) ProjectInfo {
	info := ProjectInfo{
		Type:     ProjectUnknown,
		RootPath: rootDir,
	}

	// 1. Go
	if fileExists(filepath.Join(rootDir, "go.mod")) {
		info.Type = ProjectGo
		info.StandardDoc = "standards/go.md"
		return info
	}

	// 2. Java
	pomPath := filepath.Join(rootDir, "pom.xml")
	gradlePath := filepath.Join(rootDir, "build.gradle")
	if fileExists(pomPath) || fileExists(gradlePath) {
		info.IsJVM = true
		info.RequiresArch = true
		info.Type = ProjectJavaSpring
		info.StandardDoc = "standards/java.md"
		return info
	}

	// 3. Frontend / Node
	packageJsonPath := filepath.Join(rootDir, "package.json")
	if fileExists(packageJsonPath) {
		content, err := os.ReadFile(packageJsonPath)
		if err == nil {
			str := string(content)
			if strings.Contains(str, `"react"`) {
				info.Type = ProjectReact
				info.StandardDoc = "standards/react.md"
				return info
			}
			if strings.Contains(str, `"@angular/core"`) {
				info.Type = ProjectAngular
				info.StandardDoc = "standards/react.md"
				return info
			}
		}
		info.Type = ProjectNode
		info.StandardDoc = "standards/react.md"
		return info
	}

	// 4. Python
	if fileExists(filepath.Join(rootDir, "pyproject.toml")) ||
		fileExists(filepath.Join(rootDir, "requirements.txt")) {
		info.Type = ProjectPython
		info.StandardDoc = "standards/python.md"
		return info
	}

	return info
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
