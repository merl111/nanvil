package initscaffold_test

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nsmith/initscaffold"
	"github.com/stretchr/testify/require"
)

func TestInitGoProject(t *testing.T) {
	dir := t.TempDir()
	out, err := initscaffold.Create(initscaffold.Options{Name: "Demo", Lang: "go", Dir: filepath.Join(dir, "demo")})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(out, "main.go"))
	require.FileExists(t, filepath.Join(out, "contract.yml"))
}

func TestInitPythonProject(t *testing.T) {
	dir := t.TempDir()
	out, err := initscaffold.Create(initscaffold.Options{Name: "PyDemo", Lang: "python", Dir: filepath.Join(dir, "py")})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(out, "contract.py"))
	require.FileExists(t, filepath.Join(out, "requirements.txt"))
}
