package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnsureExeInUserPath comprueba si el directorio del binario está en el PATH del usuario y lo añade automáticamente
func EnsureExeInUserPath() (bool, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, "", err
	}

	exeDir := filepath.Dir(exePath)
	currentEnvPath := os.Getenv("PATH")

	// Comprobar si ya está presente en el PATH de la sesión actual
	paths := strings.Split(currentEnvPath, string(os.PathListSeparator))
	for _, p := range paths {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(exeDir)) {
			return false, "El binario ya se encuentra accesible en el PATH.", nil
		}
	}

	if runtime.GOOS == "windows" {
		return addToWindowsUserPath(exeDir)
	}

	return addToUnixPath(exeDir)
}

func addToWindowsUserPath(dir string) (bool, string, error) {
	// Modificar el registro de usuario HKCU\Environment (User Scope)
	// Esto no requiere elevación UAC/Administrador y notifica a Windows inmediatamente
	psCmd := fmt.Sprintf(`
$d = '%s'
$p = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($p -notlike "*$d*") {
    $new = if ($p) { "$p;$d" } else { $d }
    [Environment]::SetEnvironmentVariable('Path', $new, 'User')
    Write-Output "OK"
} else {
    Write-Output "ALREADY"
}
`, dir)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	out, err := cmd.Output()
	if err != nil {
		return false, "", fmt.Errorf("error configurando PATH de usuario: %w", err)
	}

	res := strings.TrimSpace(string(out))
	if res == "OK" {
		_ = os.Setenv("PATH", os.Getenv("PATH")+";"+dir)
		return true, fmt.Sprintf("✓ Directorio '%s' añadido automáticamente al PATH de usuario de Windows.\n  (Abre una nueva terminal para ejecutar 'sdd' desde cualquier lugar).", dir), nil
	}

	return false, "El directorio ya está registrado en el PATH de usuario.", nil
}

func addToUnixPath(dir string) (bool, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", err
	}

	exportLine := fmt.Sprintf("\nexport PATH=\"%s:$PATH\"\n", dir)
	targets := []string{".bashrc", ".zshrc"}
	modified := false

	for _, t := range targets {
		rcPath := filepath.Join(home, t)
		if data, err := os.ReadFile(rcPath); err == nil {
			if !strings.Contains(string(data), dir) {
				f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					_, _ = f.WriteString(exportLine)
					_ = f.Close()
					modified = true
				}
			}
		}
	}

	if modified {
		return true, fmt.Sprintf("✓ Añadido export PATH a ~/.bashrc y ~/.zshrc para '%s'.", dir), nil
	}

	return false, "El directorio ya está registrado en el PATH.", nil
}
