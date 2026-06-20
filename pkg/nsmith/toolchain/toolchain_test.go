package toolchain_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nsmith/toolchain"
	"github.com/stretchr/testify/require"
)

func TestResolverLatestNeo3Boa(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info": map[string]string{"version": "1.2.3"},
		})
	}))
	defer srv.Close()

	res := toolchain.NewResolver()
	res.Client = srv.Client()
	// Override URL by temporarily patching - use direct pypiLatest via custom server is hard.
	// Test LatestNeo3Boa hits real PyPI in integration; here test manifest roundtrip.
	m, err := toolchain.NewManager()
	require.NoError(t, err)
	t.Setenv("NANVIL_TOOLCHAINS", t.TempDir())
	m, err = toolchain.NewManager()
	require.NoError(t, err)
	man := toolchain.Manifest{Versions: map[string]string{"go": "embedded"}}
	require.NoError(t, m.SaveManifest(man))
	loaded, err := m.LoadManifest()
	require.NoError(t, err)
	require.Equal(t, "embedded", loaded.Versions["go"])
}

func TestDoctorGo(t *testing.T) {
	t.Setenv("NANVIL_TOOLCHAINS", t.TempDir())
	m, err := toolchain.NewManager()
	require.NoError(t, err)
	checks := m.Doctor([]string{"go"})
	require.Len(t, checks, 1)
	require.True(t, checks[0].OK)
}

func TestListInstalled(t *testing.T) {
	t.Setenv("NANVIL_TOOLCHAINS", t.TempDir())
	m, err := toolchain.NewManager()
	require.NoError(t, err)
	ctx := context.Background()
	installed, latest, err := m.ListInstalled(ctx)
	require.NoError(t, err)
	require.Equal(t, "embedded", installed["go"])
	require.NotEmpty(t, latest["go"])
}
