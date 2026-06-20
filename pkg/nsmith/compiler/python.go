package compiler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/nsmith/detect"
)

// PythonBackend compiles Python contracts via neo3-boa.
type PythonBackend struct{}

func (PythonBackend) Language() Language { return LangPython }

func (PythonBackend) Detect(path string) (int, error) {
	res, err := detect.Detect(path)
	if err != nil {
		if strings.EqualFold(filepath.Ext(path), ".py") {
			return 2, nil
		}
		return 0, nil
	}
	if res.Language != detect.LangPython {
		return 0, nil
	}
	return res.Score, nil
}

func (PythonBackend) Ensure(ctx context.Context) error {
	m, err := defaultManager()
	if err != nil {
		return err
	}
	return m.EnsureLanguage(ctx, "python")
}

func (PythonBackend) Compile(ctx context.Context, req CompileRequest) (CompileResult, error) {
	src := req.Path
	info, err := os.Stat(src)
	if err != nil {
		return CompileResult{}, err
	}
	if info.IsDir() {
		src, err = findPythonContract(src)
		if err != nil {
			return CompileResult{}, err
		}
	}
	m, err := defaultManager()
	if err != nil {
		return CompileResult{}, err
	}
	boa := pythonBin(m, "neo3-boa")
	args := []string{"compile", src}
	if req.Debug {
		args = append(args, "-d")
	}
	if err := runCmd(ctx, filepath.Dir(src), compileEnv(), boa, args...); err != nil {
		return CompileResult{}, fmt.Errorf("neo3-boa: %w", err)
	}
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	dir := filepath.Dir(src)
	nef := filepath.Join(dir, base+".nef")
	manifest := filepath.Join(dir, base+".manifest.json")
	if req.OutPrefix != "" {
		out := req.OutPrefix
		if !filepath.IsAbs(out) {
			out = filepath.Join(dir, out)
		}
		if err := copyIfDifferent(nef, out+".nef"); err != nil {
			return CompileResult{}, err
		}
		if err := copyIfDifferent(manifest, out+".manifest.json"); err != nil {
			return CompileResult{}, err
		}
		nef, manifest = out+".nef", out+".manifest.json"
	}
	res := CompileResult{NEF: nef, Manifest: manifest}
	if req.Debug {
		dbg := filepath.Join(dir, base+".nefdbgnfo")
		if _, err := os.Stat(dbg); err == nil {
			res.Extras = append(res.Extras, dbg)
		}
	}
	return res, nil
}

func findPythonContract(dir string) (string, error) {
	var pyFiles []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".py") && !strings.HasSuffix(p, "_test.py") {
			pyFiles = append(pyFiles, p)
		}
		return nil
	})
	if len(pyFiles) == 0 {
		return "", fmt.Errorf("no .py contract in %s", dir)
	}
	for _, p := range pyFiles {
		if detectHasNeoBoa(p) {
			return p, nil
		}
	}
	return pyFiles[0], nil
}

func detectHasNeoBoa(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(raw)
	return strings.Contains(s, "neo3.boa") || strings.Contains(s, "@n3")
}
