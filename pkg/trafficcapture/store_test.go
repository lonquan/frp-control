package trafficcapture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManagerSettingsAndCaptureLifecycle(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:       true,
		Proxies:       []string{"web"},
		TTLSeconds:    3600,
		MaxEntryBytes: 1024,
		MaxTotalBytes: 4096,
	}))
	require.True(t, manager.Enabled("web"))
	require.False(t, manager.Enabled("other"))

	capture := manager.Begin(
		"web",
		"http",
		"http",
		"127.0.0.1:8080",
		"GET",
		"example.com",
		"/",
		"",
		map[string][]string{"Accept": {"text/plain"}},
		nil,
		false,
	)
	capture.ResponseStatus = 200
	require.NoError(t, manager.Complete(capture))

	items, err := manager.List("web", "GET", "200", "example", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	stored, err := manager.Get(capture.ID)
	require.NoError(t, err)
	require.Equal(t, "example.com", stored.Host)
}

func TestManagerListPurgesExpiredCaptures(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:       true,
		Proxies:       []string{"web"},
		TTLSeconds:    1,
		MaxEntryBytes: 1024,
		MaxTotalBytes: 4096,
	}))

	capture := manager.Begin("web", "http", "http", "127.0.0.1:8080", "GET", "example.com", "/", "", nil, nil, false)
	require.NoError(t, manager.Complete(capture))
	path := filepath.Join(manager.dir, capture.ID+".json")
	old := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(path, old, old))

	items, err := manager.List("", "", "", "", 10)
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err), "expired capture should be removed during listing")
}

func TestManagerPersistsExcludePatterns(t *testing.T) {
	dataDir := t.TempDir()
	manager, err := NewManager(dataDir)
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:         true,
		Proxies:         []string{"web"},
		ExcludePatterns: []string{" favicon.ico ", "FAVICON.ICO", "/static/*"},
		TTLSeconds:      3600,
		MaxEntryBytes:   1024,
		MaxTotalBytes:   4096,
	}))

	reloaded, err := NewManager(dataDir)
	require.NoError(t, err)
	require.Equal(t, []string{"favicon.ico", "/static/*"}, reloaded.Settings().ExcludePatterns)
	require.True(t, reloaded.Excluded("/assets/favicon.ico", ""))
	require.True(t, reloaded.Excluded("/static/js/app.js", "v=1"))
	require.False(t, reloaded.Excluded("/api/items", ""))
}
