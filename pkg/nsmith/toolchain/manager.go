package toolchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const defaultRootName = ".nanvil"

// Manifest tracks pinned toolchain versions.
type Manifest struct {
	UpdatedAt time.Time         `json:"updated_at"`
	Versions  map[string]string `json:"versions"`
}

// DefaultRoot returns the toolchain cache directory (~/.nanvil/toolchains).
func DefaultRoot() (string, error) {
	if v := os.Getenv("NANVIL_TOOLCHAINS"); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultRootName, "toolchains"), nil
}

// Manager manages toolchain installs under Root.
type Manager struct {
	Root string
}

// NewManager creates a manager with the default root.
func NewManager() (*Manager, error) {
	root, err := DefaultRoot()
	if err != nil {
		return nil, err
	}
	return &Manager{Root: root}, nil
}

func (m *Manager) manifestPath() string {
	return filepath.Join(m.Root, "manifest.json")
}

// LoadManifest reads the pinned version manifest.
func (m *Manager) LoadManifest() (Manifest, error) {
	raw, err := os.ReadFile(m.manifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{Versions: map[string]string{}}, nil
		}
		return Manifest{}, err
	}
	var man Manifest
	if err := json.Unmarshal(raw, &man); err != nil {
		return Manifest{}, err
	}
	if man.Versions == nil {
		man.Versions = map[string]string{}
	}
	return man, nil
}

// SaveManifest writes the pinned version manifest.
func (m *Manager) SaveManifest(man Manifest) error {
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return err
	}
	man.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.manifestPath(), raw, 0o644)
}

func (m *Manager) PythonVenvDir() string  { return filepath.Join(m.Root, "python", "venv") }
func (m *Manager) PythonBin(name string) string {
	return filepath.Join(m.PythonVenvDir(), "bin", name)
}

func (m *Manager) DotnetToolsDir() string { return filepath.Join(m.Root, "dotnet", "tools") }
func (m *Manager) NCCSPath() string       { return filepath.Join(m.DotnetToolsDir(), "nccs") }

func (m *Manager) JavaDir() string { return filepath.Join(m.Root, "java") }
