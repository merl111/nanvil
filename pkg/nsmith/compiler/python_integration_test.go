//go:build nsmith_integration

package compiler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nsmith/compiler"
	"github.com/nspcc-dev/neo-go/pkg/nsmith/initscaffold"
	"github.com/nspcc-dev/neo-go/pkg/nsmith/toolchain"
	"github.com/stretchr/testify/require"
)

func TestPythonCompileIntegration(t *testing.T) {
	if os.Getenv("NSMITH_INTEGRATION") == "" {
		t.Skip("set NSMITH_INTEGRATION=1 to run neo3-boa integration test")
	}
	cache := t.TempDir()
	t.Setenv("NANVIL_TOOLCHAINS", cache)
	dir := t.TempDir()
	_, err := initscaffold.Create(initscaffold.Options{Name: "PyInt", Lang: "python", Dir: dir})
	require.NoError(t, err)
	m, err := toolchain.NewManager()
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, m.Install(ctx, toolchain.InstallOptions{Languages: []string{"python"}, Update: true}))
	src := filepath.Join(dir, "contract.py")
	res, err := compiler.Compile(ctx, compiler.CompileRequest{Path: src, Lang: compiler.LangPython})
	require.NoError(t, err)
	require.FileExists(t, res.NEF)
	require.FileExists(t, res.Manifest)
}
