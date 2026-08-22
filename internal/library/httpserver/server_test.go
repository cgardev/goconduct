package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerRegistersHandlersDeterministically(t *testing.T) {
	server := New(Configuration{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := server.Handle("GET /healthz", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	if err := server.Handle("GET /healthz", http.NotFoundHandler()); err == nil {
		t.Fatal("expected duplicate pattern error")
	}
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("response status is %d", response.Code)
	}
}

func TestServerServesUntilContextCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := New(Configuration{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := server.Handle("GET /healthz", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(ctx, listener)
	}()
	response, err := http.Get("http://" + listener.Addr().String() + "/healthz")
	if err != nil {
		cancel()
		t.Fatalf("request server: %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	if response.StatusCode != http.StatusNoContent {
		cancel()
		t.Fatalf("response status is %d", response.StatusCode)
	}
	cancel()
	if err := <-serveErrors; err != nil {
		t.Fatalf("serve after cancellation: %v", err)
	}
}
