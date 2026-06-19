package impersonate_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/impersonate"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestRegistryImpersonate(t *testing.T) {
	r := impersonate.NewRegistry()
	h := util.Uint160{1}
	require.False(t, r.IsImpersonated(h))
	r.Impersonate(h)
	require.True(t, r.IsImpersonated(h))
	r.StopImpersonating(h)
	require.False(t, r.IsImpersonated(h))
}

func TestRegistryAutoMode(t *testing.T) {
	r := impersonate.NewRegistry()
	r.SetAutoMode(true)
	require.True(t, r.IsImpersonated(util.Uint160{99}))
}

func TestRegistryListAndEnabled(t *testing.T) {
	r := impersonate.NewRegistry()
	require.True(t, r.Enabled())
	r.SetEnabled(false)
	require.False(t, r.Enabled())

	h := util.Uint160{1}
	r.Impersonate(h)
	list := r.List()
	require.Len(t, list, 1)
	require.Equal(t, h, list[0])
}

func TestGlobalRegistry(t *testing.T) {
	g := impersonate.Global()
	g.Reset()
	g.SetEnabled(true)
	h := util.Uint160{2}
	g.Impersonate(h)
	require.True(t, g.IsImpersonated(h))
	g.Reset()
	require.False(t, g.IsImpersonated(h))
}
