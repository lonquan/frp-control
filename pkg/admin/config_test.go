package admin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigManagerWritesValidatesAndProtectsPaths(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "frps.toml")
	require.NoError(t, os.WriteFile(mainPath, []byte("bindPort = 7000\n"), 0o600))

	manager, err := NewConfigManager(root, mainPath, "server")
	require.NoError(t, err)

	file, err := manager.ReadFile("frps.toml")
	require.NoError(t, err)
	require.Equal(t, "frps.toml", file.Path)

	validation := manager.Validate("frps.toml", []byte("bindPort = 7001\n"))
	require.True(t, validation.Valid)

	_, err = manager.WriteFile("frps.toml", []byte("bindPort = \"bad\"\n"), file.Revision)
	require.Error(t, err)

	updated, err := manager.WriteFile("frps.toml", []byte("bindPort = 7001\n"), file.Revision)
	require.NoError(t, err)
	require.NotEqual(t, file.Revision, updated.Revision)

	_, err = manager.ReadFile("../outside.toml")
	require.ErrorContains(t, err, "越界")
}
