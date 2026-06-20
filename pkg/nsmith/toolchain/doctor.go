package toolchain

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Check reports host prerequisites for each language.
type Check struct {
	Language string
	OK       bool
	Detail   string
}

// Doctor runs prerequisite checks for requested languages.
func (m *Manager) Doctor(langs []string) []Check {
	var out []Check
	for _, lang := range langs {
		switch lang {
		case "go":
			out = append(out, Check{Language: "go", OK: true, Detail: "embedded pkg/compiler"})
		case "python":
			out = append(out, m.checkPython())
		case "csharp":
			out = append(out, m.checkCSharp())
		case "java":
			out = append(out, m.checkJava())
		default:
			out = append(out, Check{Language: lang, OK: false, Detail: "unknown language"})
		}
	}
	return out
}

func (m *Manager) checkPython() Check {
	py, err := findPython3()
	if err != nil {
		return Check{Language: "python", OK: false, Detail: err.Error()}
	}
	boa := m.PythonBin("neo3-boa")
	if runtime.GOOS == "windows" {
		boa = filepath.Join(m.PythonVenvDir(), "Scripts", "neo3-boa.exe")
	}
	if _, err := os.Stat(boa); err != nil {
		return Check{Language: "python", OK: false, Detail: fmt.Sprintf("%s OK; run: nsmith install --lang python (%v)", py, err)}
	}
	man, _ := m.LoadManifest()
	return Check{Language: "python", OK: true, Detail: fmt.Sprintf("%s; neo3-boa %s", py, man.Versions["python"])}
}

func (m *Manager) checkCSharp() Check {
	if _, err := exec.LookPath("dotnet"); err != nil {
		return Check{Language: "csharp", OK: false, Detail: "dotnet SDK not in PATH"}
	}
	if _, err := os.Stat(m.NCCSPath()); err != nil {
		return Check{Language: "csharp", OK: false, Detail: "nccs not installed; run: nsmith install --lang csharp"}
	}
	man, _ := m.LoadManifest()
	return Check{Language: "csharp", OK: true, Detail: "nccs " + man.Versions["csharp"]}
}

func (m *Manager) checkJava() Check {
	if _, err := exec.LookPath("java"); err != nil {
		return Check{Language: "java", OK: false, Detail: "java not in PATH"}
	}
	man, _ := m.LoadManifest()
	ver := man.Versions["java"]
	if ver == "" {
		ver = "unpinned"
	}
	return Check{Language: "java", OK: true, Detail: "neow3j plugin pin " + ver + " (compile via gradlew in project)"}
}

// ListInstalled returns manifest version pins.
func (m *Manager) ListInstalled(ctx context.Context) (map[string]string, map[string]string, error) {
	man, err := m.LoadManifest()
	if err != nil {
		return nil, nil, err
	}
	latest := map[string]string{"go": "embedded"}
	res := NewResolver()
	if v, err := res.LatestNeo3Boa(ctx); err == nil {
		latest["python"] = v
	}
	if v, err := res.LatestNCCS(ctx); err == nil {
		latest["csharp"] = v
	}
	if v, err := res.LatestNeow3jPlugin(ctx); err == nil {
		latest["java"] = v
	}
	installed := map[string]string{}
	for k, v := range man.Versions {
		installed[k] = v
	}
	if _, ok := installed["go"]; !ok {
		installed["go"] = "embedded"
	}
	return installed, latest, nil
}

// EnsureLanguage installs toolchain if missing.
func (m *Manager) EnsureLanguage(ctx context.Context, lang string) error {
	man, err := m.LoadManifest()
	if err != nil {
		return err
	}
	switch lang {
	case "go":
		return nil
	case "python":
		if _, err := os.Stat(m.PythonBin("neo3-boa")); err == nil {
			return nil
		}
	case "csharp":
		if _, err := os.Stat(m.NCCSPath()); err == nil {
			return nil
		}
	case "java":
		if man.Versions["java"] != "" {
			return nil
		}
	default:
		return fmt.Errorf("unknown language %q", lang)
	}
	return m.Install(ctx, InstallOptions{Languages: []string{lang}})
}
