package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"specforge/internal/adapters/fsys"
)

// HomeEnv overrides where SpecForge keeps its user files (config and logs).
const HomeEnv = "SPECFORGE_HOME"

// Dirs locates the user-level files.
type Dirs struct {
	Config string
	Logs   string
}

// UserDirs follows the platform conventions (XDG on Linux, Application
// Support on macOS, %AppData% on Windows) unless SPECFORGE_HOME is set.
func UserDirs() (Dirs, error) {
	if home := os.Getenv(HomeEnv); home != "" {
		return Dirs{Config: home, Logs: filepath.Join(home, "logs")}, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Dirs{}, err
	}
	return Dirs{Config: filepath.Join(cfg, "specforge"), Logs: filepath.Join(cache, "specforge", "logs")}, nil
}

// UserFile is the path of the user configuration.
func (d Dirs) UserFile() string { return filepath.Join(d.Config, "config.yaml") }

// LoadUser reads the user configuration. A missing file is an empty
// configuration. SpecForge 3 kept an "agent" in ~/.specforge/config.json;
// it is still honoured until the new file exists.
func LoadUser(d Dirs) (User, error) {
	var u User
	data, err := os.ReadFile(d.UserFile())
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &u); err != nil {
			return u, fmt.Errorf("%s: %w", d.UserFile(), err)
		}
		return u, nil
	case !errors.Is(err, fs.ErrNotExist):
		return u, err
	}
	if home, err := os.UserHomeDir(); err == nil {
		if legacy, err := os.ReadFile(filepath.Join(home, ".specforge", "config.json")); err == nil {
			_ = json.Unmarshal(legacy, &u)
		}
	}
	return u, nil
}

// SaveUser writes the user configuration privately (0600 in a 0700 dir).
func SaveUser(d Dirs, u User) error {
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(u)
	if err != nil {
		return err
	}
	return fsys.WriteAtomic(d.UserFile(), data, 0o600)
}

// LoadProject reads specforge.yaml at root; a missing file is an empty
// configuration. Unknown keys are an error, so a typo never silently
// leaves a default in place.
func LoadProject(root string) (Project, error) {
	var p Project
	path := filepath.Join(root, ProjectFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}
