package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"zheng-harness/internal/config"
	pluginruntime "zheng-harness/internal/plugin"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	serverapi "zheng-harness/internal/server"
	"zheng-harness/internal/store"
)

//go:embed testdata/web/index.html
var testWebUIFS embed.FS

func TestServerStartupFailsWithoutJWTConfig(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	builder, err := runtimebuilder.New(cfg, runtimebuilder.Options{WorkspaceRoot: ".", NewPluginManager: pluginruntime.NewManager})
	if err != nil {
		t.Fatalf("runtimebuilder.New() error = %v", err)
	}
	app := serverApp{
		stderr: io.Discard,
		cfg: cfg,
		builder: builder,
	}
	err = app.run(context.Background(), []string{"--db", filepath.Join(t.TempDir(), "server.db")})
	if err == nil {
		t.Fatal("run() error = nil, want jwt config failure")
	}
	if got := err.Error(); got != "server JWT secret source must be configured" {
		t.Fatalf("run() error = %q, want jwt config failure", got)
	}
}

func TestServerStartsAndExposesHealthEndpoint(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Server.JWTSecret = "test-token"
	builder, err := runtimebuilder.New(cfg, runtimebuilder.Options{WorkspaceRoot: ".", NewPluginManager: pluginruntime.NewManager})
	if err != nil {
		t.Fatalf("runtimebuilder.New() error = %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "server.db")
	started := make(chan struct{}, 1)
	stopServing := make(chan struct{})
	shutdownCalled := false
	managerShutdownCalled := false
	var handler http.Handler
	app := serverApp{
		stdout: io.Discard,
		stderr: io.Discard,
		cfg: cfg,
		builder: builder,
		newSession: func(path string, opts runtimebuilder.StoreOptions) (*store.SQLiteSessionStore, error) {
			return builder.NewSessionStore(path, opts)
		},
		newMemory: func(path string, opts runtimebuilder.StoreOptions) (*store.SQLiteMemoryStore, error) {
			return builder.NewMemoryStore(path, opts)
		},
		notifySignal: func(chan<- os.Signal, ...os.Signal) {},
		stopSignal: func(chan<- os.Signal) {},
		newHTTPServer: func(addr string, h http.Handler) *http.Server {
			handler = h
			return &http.Server{Addr: addr, Handler: h}
		},
		serve: func(*http.Server) error {
			started <- struct{}{}
			<-stopServing
			return http.ErrServerClosed
		},
		shutdownHTTP: func(context.Context, *http.Server) error {
			shutdownCalled = true
			close(stopServing)
			return nil
		},
		newRouter: func() chi.Router { return chi.NewRouter() },
		readFile: func(string) ([]byte, error) { return nil, errors.New("unexpected secret file read") },
		openSQL: func(string) (*sql.DB, error) { return &sql.DB{}, nil },
		closeSQL: func(*sql.DB) error { return nil },
		checkWAL: func(*sql.DB) (string, error) { return "wal", nil },
		shutdownManager: func(context.Context, *runtime.SessionManager) error {
			managerShutdownCalled = true
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-started
		cancel()
	}()
	if err := app.run(ctx, []string{"--db", dbPath, "--jwt-secret", "test-token"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if handler == nil {
		t.Fatal("handler was not installed")
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("GET /healthz body = %q", body)
	}
	if !shutdownCalled {
		t.Fatal("http shutdown was not called")
	}
	if !managerShutdownCalled {
		t.Fatal("session manager shutdown was not called")
	}
}

func TestWebUIRoutesMountedWithoutBreakingAPI(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Server.WebUIEnabled = true
	cfg.Server.WebUI.Enabled = true
	cfg.Server.WebUI.IndexPath = "testdata/web/index.html"
	api := &serverapi.API{Config: cfg}
	router := chi.NewRouter()
	registerRoutesWithWebFS(router, api, testWebUIFS)

	rootRec := httptest.NewRecorder()
	router.ServeHTTP(rootRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200, body=%s", rootRec.Code, rootRec.Body.String())
	}
	if got := rootRec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("GET / content-type = %q, want text/html; charset=utf-8", got)
	}
	if body := rootRec.Body.String(); body != "<!doctype html>\n<html><body>test web ui</body></html>\n" {
		t.Fatalf("GET / body = %q", body)
	}

	apiRec := httptest.NewRecorder()
	router.ServeHTTP(apiRec, httptest.NewRequest(http.MethodPost, "/api/v1/run", nil))
	if apiRec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/v1/run status = %d, want 401", apiRec.Code)
	}
}

func TestWebUIEnabledFalse(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	api := &serverapi.API{Config: cfg}
	router := chi.NewRouter()
	registerRoutesWithWebFS(router, api, testWebUIFS)

	rootRec := httptest.NewRecorder()
	router.ServeHTTP(rootRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRec.Code != http.StatusNotFound {
		t.Fatalf("GET / status = %d, want 404", rootRec.Code)
	}
}

func TestVerifyWALFailsClosedWhenDatabaseNotInWALMode(t *testing.T) {
	t.Parallel()

	app := serverApp{
		openSQL: func(string) (*sql.DB, error) { return &sql.DB{}, nil },
		closeSQL: func(*sql.DB) error { return nil },
		checkWAL: func(*sql.DB) (string, error) { return "delete", nil },
	}
	err := app.verifyWAL("ignored.db", runtimebuilder.ServerConfig{EnableWAL: true})
	if err == nil {
		t.Fatal("verifyWAL() error = nil, want failure")
	}
	if got := err.Error(); got != "server startup requires sqlite WAL mode, got \"delete\"" {
		t.Fatalf("verifyWAL() error = %q", got)
	}
}

func TestResolveJWTSecretFromFile(t *testing.T) {
	t.Parallel()

	app := serverApp{readFile: func(string) ([]byte, error) { return []byte("  file-token\n"), nil }}
	secret, err := app.resolveJWTSecret(runtimebuilder.ServerConfig{JWTSecretFile: "secret.txt"})
	if err != nil {
		t.Fatalf("resolveJWTSecret() error = %v", err)
	}
	if secret != "file-token" {
		t.Fatalf("resolveJWTSecret() = %q, want file-token", secret)
	}
}

func TestWithSignalCancellationStopsSignalOnCancel(t *testing.T) {
	t.Parallel()

	stopped := false
	app := serverApp{
		notifySignal: func(chan<- os.Signal, ...os.Signal) {},
		stopSignal: func(chan<- os.Signal) { stopped = true },
	}
	ctx, cancel := app.withSignalCancellation(context.Background())
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled")
	}
	deadline := time.Now().Add(time.Second)
	for !stopped && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("stopSignal was not called")
	}
}
