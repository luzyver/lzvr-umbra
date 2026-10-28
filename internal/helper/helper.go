// Package helper adalah bagian kecil yang berjalan sebagai root (lewat
// umbra-helper.service) untuk pekerjaan yang tidak boleh dilakukan panel
// sendiri: memasang dan melepas AdGuard Home.
//
// Panel menulis permintaan ke <data>/helper/request.json; systemd path unit
// menjalankan "umbra helper", yang menulis hasilnya ke status.json.
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
	// UnitPath dipakai panel untuk tahu apakah helper sudah dipasang installer.
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

// Available: helper terpasang (unit systemd ada).
func Available() bool {
	_, err := os.Stat(UnitPath)
	return err == nil
}

// ReadStatus dipakai panel untuk membaca hasil terakhir helper.
func ReadStatus(dataDir string) (Status, bool) {
	var st Status
	b, err := os.ReadFile(StatusPath(dataDir))
	if err != nil || json.Unmarshal(b, &st) != nil {
		return st, false
	}
	return st, true
}

// Main adalah isi perintah "umbra helper".
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
	writeStatus(dir, Status{Action: req.Action, State: "running", Message: "sedang diproses"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var msg string
	switch req.Action {
	case "install":
		msg, err = install(ctx, req.WGInterface)
	case "uninstall":
		msg, err = uninstall(ctx, req.WGInterface)
	default:
		err = fmt.Errorf("aksi tidak dikenal: %q", req.Action)
	}
	if err != nil {
		slog.Error("helper gagal", "action", req.Action, "err", err)
		writeStatus(dir, Status{Action: req.Action, State: "error", Message: err.Error()})
		return 1
	}
	writeStatus(dir, Status{Action: req.Action, State: "done", Message: msg})
	return 0
}

// readRequest membaca lalu menghapus permintaan. Folder milik user panel,
// jadi hanya file biasa yang diterima (bukan symlink).
func readRequest(path string) (Request, error) {
	var req Request
	fi, err := os.Lstat(path)
	if err != nil {
		return req, err
	}
	defer os.Remove(path)
	if !fi.Mode().IsRegular() || fi.Size() > 4096 {
		return req, errors.New("request.json tidak valid")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return req, fmt.Errorf("request.json tidak valid: %w", err)
	}
	if req.WGInterface != "" && !ifaceRe.MatchString(req.WGInterface) {
		return req, errors.New("nama interface tidak valid")
	}
	return req, nil
}

// writeStatus menulis lewat file sementara + rename supaya symlink di folder
// milik user panel tidak bisa dipakai untuk menimpa file lain.
func writeStatus(dir string, st Status) {
	st.Time = time.Now().UTC()
	b, _ := json.Marshal(st)
	tmp, err := os.CreateTemp(dir, ".status-*")
	if err != nil {
		slog.Error("tulis status helper", "err", err)
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
		slog.Error("tulis status helper", "err", err)
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
	return "", fmt.Errorf("arsitektur %s tidak didukung AdGuard Home", runtime.GOARCH)
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
			return "", fmt.Errorf("unduh AdGuard Home: %w", err)
		}
	}
	if _, err := os.Stat("/etc/systemd/system/" + ServiceName + ".service"); err != nil {
		if err := run(ctx, bin, "-s", "install",
			"--web-addr", fmt.Sprintf("%s:%d", adguard.WizardIP, adguard.WizardPort),
			"--no-check-update"); err != nil {
			return "", err
		}
	}
	// AdGuard mendengar DNS di IP tunnel, jadi harus jalan setelah interface
	// WireGuard dibuat panel; Restart=always mengulang bila start terlalu cepat.
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
	msg := "AdGuard Home terpasang"
	if wgIface != "" && ufwActive(ctx) {
		if err := run(ctx, "ufw", "allow", "in", "on", wgIface, "to", "any", "port", "53"); err != nil {
			msg += " (peringatan: gagal membuka port 53 di UFW: " + err.Error() + ")"
		}
	}
	return msg, nil
}

func uninstall(ctx context.Context, wgIface string) (string, error) {
	bin := filepath.Join(InstallDir, "AdGuardHome")
	if _, err := os.Stat(bin); err == nil {
		if err := run(ctx, bin, "-s", "uninstall"); err != nil {
			slog.Warn("uninstall service AdGuard", "err", err)
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
	return "AdGuard Home dilepas", nil
}

func ufwActive(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "ufw", "status").Output()
	return err == nil && strings.Contains(string(out), "Status: active")
}

// downloadExtract mengunduh tar.gz dan mengekstraknya ke dest
// (arsip AdGuard berisi folder AdGuardHome/).
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
			return fmt.Errorf("isi arsip mencurigakan: %s", h.Name)
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
