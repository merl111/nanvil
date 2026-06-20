package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// InstallOptions controls toolchain installation.
type InstallOptions struct {
	Languages []string
	Pin       map[string]string // lang -> version
	Update    bool
}

// Install bootstraps requested language toolchains.
func (m *Manager) Install(ctx context.Context, opts InstallOptions) error {
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return err
	}
	man, err := m.LoadManifest()
	if err != nil {
		return err
	}
	res := NewResolver()
 langs:
	for _, lang := range opts.Languages {
		switch lang {
		case "go":
			man.Versions["go"] = "embedded"
		case "python":
			ver := opts.Pin["python"]
			if ver == "" {
				if !opts.Update {
					if v, ok := man.Versions["python"]; ok && v != "" {
						continue langs
					}
				}
				ver, err = res.LatestNeo3Boa(ctx)
				if err != nil {
					return err
				}
			}
			if err := m.installPython(ctx, ver); err != nil {
				return err
			}
			man.Versions["python"] = ver
		case "csharp":
			ver := opts.Pin["csharp"]
			if ver == "" {
				if !opts.Update {
					if v, ok := man.Versions["csharp"]; ok && v != "" {
						continue langs
					}
				}
				ver, err = res.LatestNCCS(ctx)
				if err != nil {
					return err
				}
			}
			if err := m.installCSharp(ctx, ver); err != nil {
				return err
			}
			man.Versions["csharp"] = ver
		case "java":
			ver := opts.Pin["java"]
			if ver == "" {
				if !opts.Update {
					if v, ok := man.Versions["java"]; ok && v != "" {
						continue langs
					}
				}
				ver, err = res.LatestNeow3jPlugin(ctx)
				if err != nil {
					return err
				}
			}
			if err := os.MkdirAll(m.JavaDir(), 0o755); err != nil {
				return err
			}
			man.Versions["java"] = ver
		default:
			return fmt.Errorf("unknown language toolchain %q", lang)
		}
	}
	return m.SaveManifest(man)
}

func (m *Manager) installPython(ctx context.Context, version string) error {
	py, err := findPython3()
	if err != nil {
		return err
	}
	venv := m.PythonVenvDir()
	if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(venv, "pyvenv.cfg")); os.IsNotExist(err) {
		if err := runCmd(ctx, "", py, "-m", "venv", venv); err != nil {
			return fmt.Errorf("python venv: %w", err)
		}
	}
	pip := m.PythonBin("pip")
	if runtime.GOOS == "windows" {
		pip = filepath.Join(m.PythonVenvDir(), "Scripts", "pip.exe")
	}
	return runCmd(ctx, "", pip, "install", "--upgrade", "pip", "neo3-boa=="+version)
}

func (m *Manager) installCSharp(ctx context.Context, version string) error {
	if _, err := exec.LookPath("dotnet"); err != nil {
		return fmt.Errorf("dotnet SDK not found in PATH (required for C# contracts)")
	}
	tools := m.DotnetToolsDir()
	if err := os.MkdirAll(tools, 0o755); err != nil {
		return err
	}
	return runCmd(ctx, "", "dotnet", "tool", "install", "Neo.Compiler.CSharp",
		"--version", version, "--tool-path", tools)
}

func findPython3() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			if major, minor, ok := pythonVersion(p); ok && (major > 3 || (major == 3 && minor >= 13)) {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("python 3.13+ not found in PATH (required for neo3-boa)")
}

func pythonVersion(bin string) (major, minor int, ok bool) {
	out, err := exec.Command(bin, "-c", "import sys; print(sys.version_info.major, sys.version_info.minor)").Output()
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func runCmd(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
