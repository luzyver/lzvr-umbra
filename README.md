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

Server **tidak perlu** source code, Go, atau Node. Yang dipasang hanya binary hasil CI. Setelah terpasang,
Umbra hanya memakai:

| Lokasi | Isi |
|---|---|
| `/usr/local/bin/umbra` | program |
| `/etc/systemd/system/umbra.service` | service systemd |
| `/etc/umbra/umbra.env` | pengaturan |
| `/var/lib/umbra` | database, token setup, `wg0.conf` cadangan |

Syarat: Linux dengan systemd, kernel 5.6+ (modul WireGuard bawaan), akses root, port UDP 51820 terbuka
di firewall penyedia VPS. Kalau ada WireGuard lain yang memakai `wg0` atau port 51820, matikan dulu.

### Cara 1: pakai token GitHub (disarankan, sekalian untuk update)

Repo ini privat, jadi server butuh token untuk mengunduh rilis.

1. Di GitHub buat token **fine-grained**: Settings → Developer settings → Fine-grained tokens.
   Repository access: hanya `luzyver/lzvr-umbra`. Permissions: **Contents: Read-only**.
2. Simpan token di server:

   ```sh
   sudo install -d -m 0750 /etc/umbra
   sudo sh -c 'umask 077; cat > /etc/umbra/github-token'   # tempel token, Enter, lalu Ctrl+D
   ```

3. Unduh dan jalankan `update.sh`. Skrip ini mengambil rilis `latest`, mengecek sha256, lalu menjalankan
   `install.sh`:

   ```sh
   curl -fsSL -H "Authorization: Bearer $(sudo cat /etc/umbra/github-token)" \
     -H "Accept: application/vnd.github.raw" \
     https://api.github.com/repos/luzyver/lzvr-umbra/contents/deploy/update.sh | sudo sh
   ```

**Update** nanti: jalankan perintah langkah 3 lagi. Pengaturan dan data tidak ditimpa.

### Cara 2: manual (tanpa token di server)

1. Di komputer sendiri unduh `umbra-linux-amd64.tar.gz` dan `.sha256` dari rilis `latest`
   (pakai `umbra-linux-arm64` untuk VPS ARM), misalnya:

   ```sh
   gh release download latest -R luzyver/lzvr-umbra -p 'umbra-linux-amd64.tar.gz*'
   ```

2. Salin kedua file ke server (mis. `scp`), lalu di server:

   ```sh
   sha256sum -c umbra-linux-amd64.tar.gz.sha256
   tar -xzf umbra-linux-amd64.tar.gz
   sudo sh umbra-linux-amd64/install.sh umbra-linux-amd64/umbra
   ```

3. File unduhan dan folder hasil ekstrak boleh dihapus setelahnya.

### Yang dilakukan installer

Memasang nftables (wireguard-tools tidak perlu), memuat modul WireGuard, menyalakan IP forwarding,
mengizinkan `wg0` dan port 51820/udp di UFW bila UFW aktif, membuat user `umbra`, memasang service,
lalu menampilkan **token setup**. Aman dijalankan ulang. Token setup juga ada di `/var/lib/umbra/setup-token`.

### Membuka panel

Panel bawaannya hanya mendengar di `127.0.0.1:8967`. Pilih salah satu:

- **SSH tunnel** (paling mudah): `ssh -L 8967:127.0.0.1:8967 user@server`, lalu buka `http://localhost:8967`.
- **Nginx + HTTPS** (domain sendiri): set `UMBRA_TRUST_PROXY=true` di `/etc/umbra/umbra.env`,
  `sudo systemctl restart umbra`, lalu pakai contoh berikut (sertifikat dari certbot):

  ```nginx
  server {
      server_name vpn.example.com;

      # opsional: hanya dari klien VPN
      # allow 10.8.0.0/24;
      # deny all;

      location /api/events {            # SSE, jangan di-buffer
          proxy_pass http://127.0.0.1:8967;
          proxy_http_version 1.1;
          proxy_set_header Connection "";
          proxy_set_header Host $host;
          proxy_set_header X-Real-IP $remote_addr;
          proxy_set_header X-Forwarded-Proto $scheme;
          proxy_buffering off;
          proxy_read_timeout 1h;
      }

      location / {
          proxy_pass http://127.0.0.1:8967;
          proxy_set_header Host $host;
          proxy_set_header X-Real-IP $remote_addr;
          proxy_set_header X-Forwarded-Proto $scheme;
      }

      listen 443 ssl;
      ssl_certificate     /etc/letsencrypt/live/vpn.example.com/fullchain.pem;
      ssl_certificate_key /etc/letsencrypt/live/vpn.example.com/privkey.pem;
  }
  ```

### Setup pertama

Buka panel, masukkan token setup, buat akun admin, lalu isi wizard: endpoint publik
(mis. `vpn.example.com:51820`), interface internet server (cek dengan `ip route get 1.1.1.1`),
subnet VPN (mis. `10.8.0.1/24`), dan DNS klien. Setelah itu tambah klien dan scan QR-nya.

### Cek dan uninstall

```sh
systemctl status umbra
journalctl -u umbra -f
```

Uninstall: `sudo systemctl disable --now umbra`, lalu hapus `/usr/local/bin/umbra`,
`/etc/systemd/system/umbra.service`, `/etc/umbra`, `/var/lib/umbra`, `/etc/sysctl.d/90-umbra.conf`,
`/etc/modules-load.d/umbra.conf`, dan `sudo nft delete table inet umbra`.

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
