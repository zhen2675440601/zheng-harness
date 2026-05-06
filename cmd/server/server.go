package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zheng-harness/internal/config"
	pluginruntime "zheng-harness/internal/plugin"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	serverapi "zheng-harness/internal/server"
	"zheng-harness/internal/store"
)

const defaultDBPath = "./agent.db"

type serverApp struct {
	stdout          io.Writer
	stderr          io.Writer
	cfg             config.Config
	builder         *runtimebuilder.Builder
	newSession      func(string, runtimebuilder.StoreOptions) (*store.SQLiteSessionStore, error)
	newMemory       func(string, runtimebuilder.StoreOptions) (*store.SQLiteMemoryStore, error)
	notifySignal    func(chan<- os.Signal, ...os.Signal)
	stopSignal      func(chan<- os.Signal)
	newHTTPServer   func(string, http.Handler) *http.Server
	serve           func(*http.Server) error
	shutdownHTTP    func(context.Context, *http.Server) error
	newRouter       func() chi.Router
	readFile        func(string) ([]byte, error)
	openSQL         func(string) (*sql.DB, error)
	closeSQL        func(*sql.DB) error
	checkWAL        func(*sql.DB) (string, error)
	shutdownManager func(context.Context, *runtime.SessionManager) error
}

func runServer(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Load(runtimebuilder.FilterConfigArgs(args))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	builder, err := runtimebuilder.New(cfg, runtimebuilder.Options{WorkspaceRoot: ".", NewPluginManager: pluginruntime.NewManager})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app := serverApp{
		stdout:  stdout,
		stderr:  stderr,
		cfg:     cfg,
		builder: builder,
		newSession: func(dbPath string, opts runtimebuilder.StoreOptions) (*store.SQLiteSessionStore, error) {
			return builder.NewSessionStore(dbPath, opts)
		},
		newMemory: func(dbPath string, opts runtimebuilder.StoreOptions) (*store.SQLiteMemoryStore, error) {
			return builder.NewMemoryStore(dbPath, opts)
		},
		notifySignal: signal.Notify,
		stopSignal:   signal.Stop,
		newHTTPServer: func(addr string, handler http.Handler) *http.Server {
			return &http.Server{
				Addr:              addr,
				Handler:           handler,
				ReadHeaderTimeout: 20 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      300 * time.Second,
				IdleTimeout:       60 * time.Second,
				MaxHeaderBytes:    1 << 20, // 1MB
			}
		},
		serve:        func(server *http.Server) error { return server.ListenAndServe() },
		shutdownHTTP: func(ctx context.Context, server *http.Server) error { return server.Shutdown(ctx) },
		newRouter:    func() chi.Router { return chi.NewRouter() },
		readFile:     os.ReadFile,
		openSQL:      func(path string) (*sql.DB, error) { return sql.Open("sqlite", path) },
		closeSQL: func(db *sql.DB) error {
			if db == nil {
				return nil
			}
			return db.Close()
		},
		checkWAL: func(db *sql.DB) (string, error) {
			var mode string
			if err := db.QueryRow(`PRAGMA journal_mode;`).Scan(&mode); err != nil {
				return "", err
			}
			return strings.ToLower(strings.TrimSpace(mode)), nil
		},
		shutdownManager: func(ctx context.Context, manager *runtime.SessionManager) error {
			if manager == nil {
				return nil
			}
			return manager.Shutdown(ctx)
		},
	}
	if err := app.run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (a serverApp) run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("zheng-server", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	_ = fs.String("config", "", "config file path")
	_ = fs.String("provider", "", "provider")
	_ = fs.String("plugin-provider", "", "plugin provider")
	_ = fs.String("model", "", "model")
	_ = fs.String("api-key", "", "API key")
	_ = fs.String("base-url", "", "base URL")
	_ = fs.Int("max-steps", 0, "max steps")
	_ = fs.String("step-timeout", "", "step timeout")
	_ = fs.Int("memory-limit-mb", 0, "memory limit")
	_ = fs.String("verify-mode", "", "verify mode")
	listenAddress := fs.String("listen-address", a.cfg.Server.ListenAddress, "server listen address")
	jwtSecret := fs.String("jwt-secret", a.cfg.Server.JWTSecret, "server JWT secret")
	jwtSecretFile := fs.String("jwt-secret-file", a.cfg.Server.JWTSecretFile, "server JWT secret file")
	activeSessionCap := fs.Int("active-session-cap", a.cfg.Server.ActiveSessionCap, "server active session cap")
	shutdownTimeout := fs.Duration("shutdown-timeout", a.cfg.Server.ShutdownTimeout, "server shutdown timeout")
	enableWAL := fs.Bool("server-enable-wal", a.cfg.Server.EnableWAL, "enable sqlite WAL mode for server")
	dbPath := fs.String("db", defaultDBPath, "sqlite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	serverCfg := runtimebuilder.ServerConfig{
		ListenAddress:    *listenAddress,
		JWTSecret:        *jwtSecret,
		JWTSecretFile:    *jwtSecretFile,
		ActiveSessionCap: *activeSessionCap,
		ShutdownTimeout:  *shutdownTimeout,
		EnableWAL:        *enableWAL,
	}.Normalize()
	if err := serverCfg.Validate(); err != nil {
		return err
	}
	resolvedSecret, err := a.resolveJWTSecret(serverCfg)
	if err != nil {
		return err
	}
	serverCfg.JWTSecret = resolvedSecret
	serverCfg.JWTSecretFile = ""

	sessionStore, memoryStore, cleanup, err := a.openRuntimeDeps(*dbPath, runtimebuilder.StoreOptions{EnableWAL: serverCfg.EnableWAL})
	if err != nil {
		return err
	}
	defer cleanup()
	if err := a.verifyWAL(*dbPath, serverCfg); err != nil {
		return err
	}
	_ = memoryStore

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{
		ActiveSessionCap: serverCfg.ActiveSessionCap,
		ShutdownTimeout:  serverCfg.ShutdownTimeout,
	})
	api := &serverapi.API{
		SessionStore: sessionStore,
		MemoryStore:  memoryStore,
		Manager:      manager,
		Builder:      a.builder,
		Config:       a.cfg,
		JWTSecret:    resolvedSecret,
	}
	router := a.newRouter()
	router.Use(middleware.RequestID)
	router.Use(api.Recoverer)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		serverapi.WriteJSONForServer(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(api.AuthMiddleware)
		r.Post("/run", api.JSON(api.HandleRun))
		r.Post("/resume", api.JSON(api.HandleResume))
		r.Get("/sessions/{id}/inspect", api.JSON(api.HandleInspect))
		r.Get("/sessions/{id}/stream", api.JSON(api.HandleStream))
	})

	server := a.newHTTPServer(serverCfg.ListenAddress, router)
	errCh := make(chan error, 1)
	go func() {
		err := a.serve(server)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		close(errCh)
	}()

	ctx, stop := a.withSignalCancellation(ctx)
	defer stop()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), serverCfg.ShutdownTimeout)
	defer cancel()
	if err := a.shutdownHTTP(shutdownCtx, server); err != nil {
		return err
	}
	if err := a.shutdownManager(shutdownCtx, manager); err != nil {
		return err
	}
	if err := <-errCh; err != nil {
		return err
	}
	return nil
}

func (a serverApp) openRuntimeDeps(dbPath string, opts runtimebuilder.StoreOptions) (*store.SQLiteSessionStore, *store.SQLiteMemoryStore, func(), error) {
	sessionStore, err := a.newSession(dbPath, opts)
	if err != nil {
		return nil, nil, nil, err
	}
	memoryStore, err := a.newMemory(dbPath, opts)
	if err != nil {
		_ = sessionStore.Close()
		return nil, nil, nil, err
	}
	cleanup := func() {
		_ = memoryStore.Close()
		_ = sessionStore.Close()
	}
	return sessionStore, memoryStore, cleanup, nil
}

func (a serverApp) resolveJWTSecret(cfg runtimebuilder.ServerConfig) (string, error) {
	if cfg.JWTSecret != "" {
		return cfg.JWTSecret, nil
	}
	data, err := a.readFile(cfg.JWTSecretFile)
	if err != nil {
		return "", fmt.Errorf("read server JWT secret file: %w", err)
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", errors.New("server JWT secret file must not be empty")
	}
	return secret, nil
}

func (a serverApp) verifyWAL(dbPath string, cfg runtimebuilder.ServerConfig) error {
	if !cfg.EnableWAL {
		return errors.New("server startup requires sqlite WAL mode")
	}
	db, err := a.openSQL(dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite database for WAL verification: %w", err)
	}
	defer func() { _ = a.closeSQL(db) }()
	mode, err := a.checkWAL(db)
	if err != nil {
		return fmt.Errorf("verify sqlite WAL mode: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("server startup requires sqlite WAL mode, got %q", mode)
	}
	return nil
}

func (a serverApp) withSignalCancellation(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	a.notifySignal(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case <-sigCh:
			cancel()
		}
		a.stopSignal(sigCh)
	}()
	return ctx, cancel
}
