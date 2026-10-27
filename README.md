# Umbra

## Struktur

```
cmd/umbra            main program
internal/api         HTTP API, sesi, CSRF, CSP
internal/wg          WireGuard (wgctrl) + backend tiruan untuk dev
internal/firewall    nftables + uji 60 detik
internal/adguard     klien REST AdGuard Home
internal/store       SQLite (modernc, tanpa CGO)
internal/system      cek sistem untuk wizard
internal/ui          frontend hasil build (go:embed)
web/                 SvelteKit (Svelte 5, Tailwind 4)
deploy/              install.sh, service systemd, file env
```

## Build

Butuh Go 1.26+ dan Node 22+.

```sh
cd web && npm ci && npm run build && cd ..        # hasil ke internal/ui/dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o deploy/umbra ./cmd/umbra
```

Untuk VPS ARM ganti `GOARCH=arm64`.

### Build otomatis (GitHub Actions)

Tiap push ke `master`, workflow `.github/workflows/release.yml` membuild frontend + binary (amd64 dan arm64)
dan menerbitkannya ke rilis `latest` sebagai `umbra-linux-<arch>.tar.gz` (+ `.sha256`). Server kecil
(RAM 1 GB) tidak perlu build sendiri.

## Pasang di server

Cara termudah: pakai build dari CI. Karena repo privat, buat token GitHub fine-grained (hanya repo ini,
izin **Contents: Read-only**), simpan di server, lalu jalankan `update.sh`:

```sh
sudo install -d -m 0750 /etc/umbra
sudo sh -c 'umask 077; cat > /etc/umbra/github-token'   # tempel token, lalu Ctrl+D
curl -fsSL -H "Authorization: Bearer $(sudo cat /etc/umbra/github-token)" \
  -H "Accept: application/vnd.github.raw" \
  https://api.github.com/repos/luzyver/lzvr-umbra/contents/deploy/update.sh | sudo sh
```

Perintah yang sama dipakai untuk update. Kalau sudah ada salinannya: `sudo sh update.sh`.

Atau manual: salin folder `deploy/` (berisi binary `umbra` hasil build) ke server, lalu:

```sh
sudo sh install.sh
```

Installer akan memasang nftables (wireguard-tools tidak perlu), memuat modul WireGuard, menyalakan IP forwarding,
membuat user `umbra`, memasang service, lalu menampilkan **token setup**.

Panel bawaannya hanya mendengar di `127.0.0.1:8967`. Cara membukanya:

- **SSH tunnel** (paling mudah): `ssh -L 8967:127.0.0.1:8967 user@server`, lalu buka `http://localhost:8967`.
- **Nginx + HTTPS**: proxy ke `127.0.0.1:8967`, set `UMBRA_TRUST_PROXY=true`. Untuk SSE tambahkan
  `proxy_buffering off;` pada `location /api/events`.

Jangan lupa membuka port UDP WireGuard (bawaan 51820) di firewall penyedia VPS.

## Pengaturan (`/etc/umbra/umbra.env`)

| Variabel | Bawaan | Arti |
|---|---|---|
| `UMBRA_LISTEN` | `127.0.0.1:8967` | alamat web panel |
| `UMBRA_DATA_DIR` | `/var/lib/umbra` | database, token setup, `wg0.conf` cadangan |
| `UMBRA_SECURE_COOKIE` | `true` | cookie hanya lewat HTTPS (localhost tetap bisa) |
| `UMBRA_TRUST_PROXY` | `false` | percayai `X-Real-IP` dari nginx |
| `UMBRA_ADGUARD_URL` | kosong | mis. `http://127.0.0.1:3000`; kosong = AdGuard mati |
| `UMBRA_ADGUARD_USER` / `UMBRA_ADGUARD_PASS` | kosong | login AdGuard Home |

Setelah mengubah: `sudo systemctl restart umbra`.

### AdGuard Home

Pasang AdGuard Home terpisah, lalu atur supaya:

- DNS hanya mendengar di IP server VPN, mis. `10.8.0.1:53` (jangan `0.0.0.0`, supaya tidak jadi open resolver);
- web UI di `127.0.0.1:3000`;
- isi `UMBRA_ADGUARD_*`, dan di Pengaturan panel ubah DNS bawaan klien ke `10.8.0.1`.

Panel mendaftarkan setiap klien di AdGuard dengan nama `Nama [wg#id]` dan mengatur blokir iklan per klien.

## Pengembangan (Windows/macOS juga bisa)

```sh
# terminal 1: backend dengan WireGuard & firewall tiruan
UMBRA_DEV=1 UMBRA_FAKE_WG=1 UMBRA_FAKE_FIREWALL=1 go run ./cmd/umbra
# terminal 2: frontend dengan hot reload (proxy /api ke :8967)
cd web && npm run dev
```

Token setup tercetak di log dan tersimpan di `./data/setup-token`.

Catatan SvelteKit 3: alias `$lib` sudah tidak ada; pakai `#lib/...` (diatur di `imports` pada `web/package.json`).

## Belum ada

- 2FA (TOTP) untuk login admin
- fallback `wireguard-go` untuk kernel tanpa modul WireGuard
- impor dari `wg0.conf` / database wireguard-ui lama
- pemasangan AdGuard Home otomatis dari installer
