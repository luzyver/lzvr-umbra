// Package helper is a small component that runs as root (via
// umbra-helper.service) for jobs the panel must not do itself:
// installing and removing AdGuard Home.
//
// The panel writes a request to <data>/helper/request.json; a systemd path unit
// runs "umbra helper", which writes the result to status.json.
package helper

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/luzyver/lzvr-umbra/internal/adguard"
	"github.com/luzyver/lzvr-umbra/internal/config"
)

const (
	InstallDir  = "/opt/AdGuardHome"
	ServiceName = "AdGuardHome"
	// UnitPath is used by the panel to tell whether the installer has set up the helper.
	UnitPath  = "/etc/systemd/system/umbra-helper.path"
	dropInDir = "/etc/systemd/system/AdGuardHome.service.d"
)

type Request struct {
	Action      string `json:"action"` // install | uninstall
	WGInterface string `json:"wg_interface"`
}

type Status struct {
	Action  string    `json:"action"`
	State   string    `json:"state"` // running | done | error
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

var ifaceRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`)

func Dir(dataDir string) string         { return filepath.Join(dataDir, "helper") }
func RequestPath(dataDir string) string { return filepath.Join(Dir(dataDir), "request.json") }
func StatusPath(dataDir string) string  { return filepath.Join(Dir(dataDir), "status.json") }

// Available reports whether the helper is installed (the systemd unit exists).
func Available() bool {
	_, err := os.Stat(UnitPath)
	return err == nil
}

// ReadStatus is used by the panel to read the helper's latest result.
func ReadStatus(dataDir string) (Status, bool) {
	var st Status
	b, err := os.ReadFile(StatusPath(dataDir))
	if err != nil || json.Unmarshal(b, &st) != nil {
		return st, false
	}
	return st, true
}

// Main is the body of the "umbra helper" command.
func Main() int {
	cfg := config.Load()
	dir := Dir(cfg.DataDir)
	req, err := readRequest(RequestPath(cfg.DataDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0
		}
		writeStatus(dir, Status{State: "error", Message: err.Error()})
		return 1
	}
	writeStatus(dir, Status{Action: req.Action, State: "running", Message: "processing"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var msg string
	switch req.Action {
	case "install":
		msg, err = install(ctx, req.WGInterface)
	case "uninstall":
		msg, err = uninstall(ctx, req.WGInterface)
	default:
		err = fmt.Errorf("unknown action: %q", req.Action)
	}
	if err != nil {
		slog.Error("helper failed", "action", req.Action, "err", err)
		writeStatus(dir, Status{Action: req.Action, State: "error", Message: err.Error()})
		return 1
	}
	writeStatus(dir, Status{Action: req.Action, State: "done", Message: msg})
	return 0
}

// readRequest reads and then deletes the request. The folder is owned by the
// panel user, so only regular files are accepted (not symlinks).
func readRequest(path string) (Request, error) {
	var req Request
	fi, err := os.Lstat(path)
	if err != nil {
		return req, err
	}
	defer os.Remove(path)
	if !fi.Mode().IsRegular() || fi.Size() > 4096 {
		return req, errors.New("invalid request.json")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return req, fmt.Errorf("invalid request.json: %w", err)
	}
	if req.WGInterface != "" && !ifaceRe.MatchString(req.WGInterface) {
		return req, errors.New("invalid interface name")
	}
	return req, nil
}

// writeStatus writes via a temp file + rename so a symlink in the folder
// owned by the panel user cannot be used to overwrite other files.
func writeStatus(dir string, st Status) {
	st.Time = time.Now().UTC()
	b, _ := json.Marshal(st)
	tmp, err := os.CreateTemp(dir, ".status-*")
	if err != nil {
		slog.Error("write helper status", "err", err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return
	}
	_ = tmp.Chmod(0o644)
	if tmp.Close() != nil {
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, "status.json")); err != nil {
		slog.Error("write helper status", "err", err)
	}
}

func run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func archName() (string, error) {
	switch runtime.GOARCH {
	case "amd64", "arm64", "386":
		return runtime.GOARCH, nil
	case "arm":
		return "armv7", nil
	}
	return "", fmt.Errorf("architecture %s is not supported by AdGuard Home", runtime.GOARCH)
}

func install(ctx context.Context, wgIface string) (string, error) {
	bin := filepath.Join(InstallDir, "AdGuardHome")
	if _, err := os.Stat(bin); err != nil {
		arch, err := archName()
		if err != nil {
			return "", err
		}
		url := "https://static.adguard.com/adguardhome/release/AdGuardHome_linux_" + arch + ".tar.gz"
		if err := downloadExtract(ctx, url, filepath.Dir(InstallDir)); err != nil {
			return "", fmt.Errorf("download AdGuard Home: %w", err)
		}
	}
	if _, err := os.Stat("/etc/systemd/system/" + ServiceName + ".service"); err != nil {
		if err := run(ctx, bin, "-s", "install",
			"--web-addr", fmt.Sprintf("%s:%d", adguard.WizardIP, adguard.WizardPort),
			"--no-check-update"); err != nil {
			return "", err
		}
	}
	// AdGuard listens for DNS on the tunnel IP, so it must start after the
	// WireGuard interface is created by the panel; Restart=always retries if it starts too early.
	if err := os.MkdirAll(dropInDir, 0o755); err != nil {
		return "", err
	}
	dropIn := "[Unit]\nAfter=umbra.service\n\n[Service]\nRestart=always\nRestartSec=5\n"
	if err := os.WriteFile(filepath.Join(dropInDir, "umbra.conf"), []byte(dropIn), 0o644); err != nil {
		return "", err
	}
	if err := run(ctx, "systemctl", "daemon-reload"); err != nil {
		return "", err
	}
	if err := run(ctx, "systemctl", "enable", ServiceName); err != nil {
		return "", err
	}
	if err := run(ctx, "systemctl", "restart", ServiceName); err != nil {
		return "", err
	}
	msg := "AdGuard Home installed"
	if wgIface != "" && ufwActive(ctx) {
		if err := run(ctx, "ufw", "allow", "in", "on", wgIface, "to", "any", "port", "53"); err != nil {
			msg += " (warning: failed to open port 53 in UFW: " + err.Error() + ")"
		}
	}
	return msg, nil
}

func uninstall(ctx context.Context, wgIface string) (string, error) {
	bin := filepath.Join(InstallDir, "AdGuardHome")
	if _, err := os.Stat(bin); err == nil {
		if err := run(ctx, bin, "-s", "uninstall"); err != nil {
			slog.Warn("uninstall AdGuard service", "err", err)
		}
	}
	_ = os.RemoveAll(dropInDir)
	_ = run(ctx, "systemctl", "daemon-reload")
	if err := os.RemoveAll(InstallDir); err != nil {
		return "", err
	}
	if wgIface != "" && ufwActive(ctx) {
		_ = run(ctx, "ufw", "delete", "allow", "in", "on", wgIface, "to", "any", "port", "53")
	}
	return "AdGuard Home removed", nil
}

func ufwActive(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "ufw", "status").Output()
	return err == nil && strings.Contains(string(out), "Status: active")
}

// downloadExtract downloads a tar.gz and extracts it into dest
// (the AdGuard archive contains an AdGuardHome/ folder).
func downloadExtract(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	root := filepath.Clean(dest) + string(os.PathSeparator)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(strings.TrimPrefix(h.Name, "./"))
		if !strings.HasPrefix(name, "AdGuardHome") {
			continue
		}
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, root) {
			return fmt.Errorf("suspicious archive entry: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode)&0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, 200<<20)); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}
