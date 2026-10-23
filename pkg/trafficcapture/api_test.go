package trafficcapture

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestCaptureCollectionDeleteClearsAllRecords(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(Settings{
		Enabled:       true,
		Proxies:       []string{"web"},
		TTLSeconds:    3600,
		MaxEntryBytes: 1024,
		MaxTotalBytes: 4096,
	}))

	var captureIDs []string
	for _, host := range []string{"one.example.com", "two.example.com"} {
		capture := manager.Begin("web", "http", "http", "127.0.0.1:8080", "GET", host, "/", "", nil, nil, false)
		require.NoError(t, manager.Complete(capture))
		captureIDs = append(captureIDs, capture.ID)
	}

	router := mux.NewRouter()
	manager.RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/v2/captures", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)

	items, err := manager.List("", "", "", "", 10)
	require.NoError(t, err)
	require.Empty(t, items)
	for _, id := range captureIDs {
		_, err := manager.Get(id)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	require.True(t, manager.Settings().Enabled)
	_, err = os.Stat(filepath.Join(manager.dir, "settings.json"))
	require.NoError(t, err)
}
