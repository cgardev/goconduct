package architecture

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var errResponseWrite = errors.New("response write failure")

type failingResponseWriter struct {
	header http.Header
	status int
}

func (writer *failingResponseWriter) Header() http.Header {
	return writer.header
}

func (*failingResponseWriter) Write([]byte) (int, error) {
	return 0, errResponseWrite
}

func (writer *failingResponseWriter) WriteHeader(status int) {
	writer.status = status
}

func TestDashboardAssetHandler_ServesApplicationRoutesAndAssets(t *testing.T) {
	assets := fstest.MapFS{
		"index.html": {Data: []byte("<html>application shell</html>")},
		"main.js":    {Data: []byte("console.log('goconduct')")},
		".secret":    {Data: []byte("hidden")},
	}
	handler := newDashboardAssetHandlerFromFS(assets, discardLogger())
	testCases := []struct {
		name      string
		method    string
		path      string
		wantCode  int
		wantBody  string
		wantCache string
	}{
		{
			name: "the application root", method: http.MethodGet, path: "/",
			wantCode: http.StatusOK, wantBody: "application shell", wantCache: "no-store",
		},
		{
			name: "an application route", method: http.MethodGet, path: "/components/library",
			wantCode: http.StatusOK, wantBody: "application shell", wantCache: "no-store",
		},
		{
			name: "a compiled asset", method: http.MethodGet, path: "/main.js",
			wantCode: http.StatusOK, wantBody: "goconduct", wantCache: "immutable",
		},
		{
			name: "an absent compiled asset", method: http.MethodGet, path: "/missing.js",
			wantCode: http.StatusNotFound,
		},
		{
			name: "a hidden asset", method: http.MethodGet, path: "/.secret",
			wantCode: http.StatusNotFound,
		},
		{
			name: "an unsupported method", method: http.MethodPost, path: "/",
			wantCode: http.StatusMethodNotAllowed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, nil)

			handler.ServeHTTP(response, request)

			if response.Code != testCase.wantCode {
				t.Fatalf("asset status is %d, want %d", response.Code, testCase.wantCode)
			}
			if testCase.wantBody != "" && !strings.Contains(response.Body.String(), testCase.wantBody) {
				t.Errorf("asset body does not contain %q", testCase.wantBody)
			}
			if testCase.wantCache != "" &&
				!strings.Contains(response.Header().Get("Cache-Control"), testCase.wantCache) {
				t.Errorf("cache policy is %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestDashboardAssetHandler_HandlesHeadRequest(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("application shell")}}
	handler := newDashboardAssetHandlerFromFS(assets, discardLogger())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodHead, "/deep/link", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.Len() != 0 {
		t.Errorf("HEAD response is status %d with %d bytes", response.Code, response.Body.Len())
	}
}

func TestDashboardAssetHandler_LogsApplicationShellWriteFailure(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	assets := fstest.MapFS{"index.html": {Data: []byte("application shell")}}
	handler := newDashboardAssetHandlerFromFS(assets, logger)
	writer := &failingResponseWriter{header: make(http.Header)}
	request := httptest.NewRequest(http.MethodGet, "/deep/link", nil)

	handler.ServeHTTP(writer, request)

	if !strings.Contains(logs.String(), "The dashboard cannot write its application shell.") ||
		!strings.Contains(logs.String(), errResponseWrite.Error()) {
		t.Errorf("structured log omits the write failure: %q", logs.String())
	}
}

func TestDashboardAssets_ContainOnlyAngularApplication(t *testing.T) {
	assets, err := fs.Sub(dashboardAssets, dashboardWebRoot)
	if err != nil {
		t.Fatalf("open embedded dashboard assets: %v", err)
	}
	var names []string
	var combined strings.Builder
	if err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		names = append(names, name)
		payload, readErr := fs.ReadFile(assets, name)
		if readErr != nil {
			return readErr
		}
		combined.Write(payload)
		return nil
	}); err != nil {
		t.Fatalf("walk embedded dashboard assets: %v", err)
	}

	joinedNames := strings.Join(names, "\n")
	if !strings.Contains(joinedNames, "main-") || !strings.Contains(joinedNames, "styles-") {
		t.Fatalf("embedded assets do not contain the Angular bundles: %s", joinedNames)
	}
	if !strings.Contains(joinedNames, "3rdpartylicenses.txt") {
		t.Fatalf("embedded assets do not contain third-party notices: %s", joinedNames)
	}
	for _, retired := range []string{"dashboard.js", "dashboard.css", "runtime.js", "graph.css"} {
		if strings.Contains(joinedNames, retired) {
			t.Errorf("embedded assets still contain %q", retired)
		}
	}
	content := strings.ToLower(combined.String())
	if strings.Contains(content, "@font-face") {
		t.Error("embedded frontend contains a bundled custom font")
	}
	if !strings.Contains(content, "system-ui") {
		t.Error("embedded frontend does not declare system typography")
	}
	if !strings.Contains(content, "package: @angular/core") {
		t.Error("embedded frontend does not identify its Angular license")
	}
}
