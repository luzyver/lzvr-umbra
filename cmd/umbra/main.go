// Command umbra runs the WireGuard + AdGuard Home web panel.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
	"github.com/luzyver/lzvr-umbra/internal/api"
	"github.com/luzyver/lzvr-umbra/internal/auth"
	"github.com/luzyver/lzvr-umbra/internal/config"
	"github.com/luzyver/lzvr-umbra/internal/firewall"
	"github.com/luzyver/lzvr-umbra/internal/helper"
	"github.com/luzyver/lzvr-umbra/internal/store"
	"github.com/luzyver/lzvr-umbra/internal/ui"
	"github.com/luzyver/lzvr-umbra/internal/wg"
)

func main() {
	// "umbra helper" runs as root under umbra-helper.service (installs/removes AdGuard).
	if len(os.Args) > 1 && os.Args[1] == "helper" {
		os.Exit(helper.Main())
	}
	if err := run(); err != nil {
		slog.Error("umbra stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(cfg.DataDir, "umbra.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	var backend wg.Backend
	if cfg.FakeWG {
		slog.Warn("using fake WireGuard (UMBRA_FAKE_WG): no changes are made to the system")
		backend = wg.NewFake()
	} else if backend, err = wg.New(cfg.WGBackend); err != nil {
		return err
	}
	defer func() {
		if err := backend.Close(); err != nil {
			slog.Error("close WireGuard backend", "err", err)
		}
	}()
	fw := firewall.NewManager(cfg.FakeFirewall)
	ag := adguard.New(cfg.AdGuardURL, cfg.AdGuardUser, cfg.AdGuardPass)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	setupToken, err := prepareSetupToken(ctx, cfg, st)
	if err != nil {
		return err
	}

	srv := api.New(cfg, st, backend, fw, ag, setupToken)
	srv.Bootstrap(ctx)
	go srv.RunActivityMonitor(ctx)

	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := st.PurgeExpiredSessions(ctx); err != nil {
					slog.Warn("purge sessions", "err", err)
				}
			}
		}
	}()

	hs := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(ui.Handler(), ui.ScriptHashes()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second, // SSE disables this limit per request
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("panel running", "address", "http://"+cfg.Listen)
		errc <- hs.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down panel...")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := hs.Shutdown(sctx); err != nil {
			return err
		}
	}
	return nil
}

// prepareSetupToken creates a one-time token when no admin exists yet.
// The token is stored in a file (0600) and written to the log so it can be read via journalctl.
func prepareSetupToken(ctx context.Context, cfg config.Config, st *store.Store) (string, error) {
	n, err := st.AdminCount(ctx)
	if err != nil {
		return "", err
	}
	path := api.SetupTokenPath(cfg.DataDir)
	if n > 0 {
		_ = os.Remove(path)
		return "", nil
	}
	token := auth.NewToken()
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", err
	}
	slog.Info("no admin yet, open the panel and enter the setup token", "token", token, "file", path)
	return token, nil
}
