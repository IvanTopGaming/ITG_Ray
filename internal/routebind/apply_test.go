package routebind

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/itg-team/itg-ray/internal/rules"
	"github.com/itg-team/itg-ray/internal/server"
)

func TestApply_PersistsRebind(t *testing.T) {
	store := rules.NewStore(t.TempDir())
	require.NoError(t, store.Save(model(bound("a", "sub", "NL"))))
	servers := []server.Server{srv("b", "sub", "NL")}

	changed, err := Apply(store, servers)
	require.NoError(t, err)
	require.True(t, changed)
	m, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "b", m.Groups[0].Rules[0].ServerID)

	changed, err = Apply(store, servers)
	require.NoError(t, err)
	require.False(t, changed)
}

func TestApply_MissingDoesNotRewrite(t *testing.T) {
	store := rules.NewStore(t.TempDir())
	require.NoError(t, store.Save(model(bound("a", "sub", "NL"))))
	changed, err := Apply(store, []server.Server{srv("z", "sub", "FI")})
	require.NoError(t, err)
	require.False(t, changed)
	m, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "a", m.Groups[0].Rules[0].ServerID)
}

func TestApply_PersistsNameDrift(t *testing.T) {
	store := rules.NewStore(t.TempDir())
	require.NoError(t, store.Save(model(bound("a", "sub", "NL"))))
	changed, err := Apply(store, []server.Server{srv("a", "sub", "NL renamed")})
	require.NoError(t, err)
	require.True(t, changed)
	m, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, "NL renamed", m.Groups[0].Rules[0].ServerName)
}
