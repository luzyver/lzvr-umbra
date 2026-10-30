<div align="center">

# Umbra

Panel web untuk WireGuard dan AdGuard Home di satu VPS Linux.

[![release](https://github.com/luzyver/lzvr-umbra/actions/workflows/release.yml/badge.svg)](https://github.com/luzyver/lzvr-umbra/actions/workflows/release.yml)
[![ci](https://github.com/luzyver/lzvr-umbra/actions/workflows/ci.yml/badge.svg)](https://github.com/luzyver/lzvr-umbra/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/luzyver/lzvr-umbra)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

</div>

Umbra adalah satu binary Go (frontend sudah di-embed) yang mengurus server WireGuard dari browser:
wizard setup, perangkat dengan QR, status dan trafik langsung, firewall nftables, log sesi, aktivitas
jaringan per perangkat, dan AdGuard Home yang dipasang serta dikelola dari panel. Server tidak butuh
Go, Node, atau source code.

## Daftar isi

- [Fitur](#fitur)
- [Syarat](#syarat)
- [Pasang di server](#pasang-di-server)
- [Membuka panel](#membuka-panel)
- [Pengaturan](#pengaturan-etcumbraumbraenv)
- [AdGuard Home](#adguard-home)
- [Pindah dari WireGuard lama](#pindah-dari-wireguard-lama)
- [Aktivitas jaringan & log sesi](#aktivitas-jaringan--log-sesi)
- [Pengembangan](#pengembangan)
- [Kontribusi](#kontribusi)
- [Lisensi](#lisensi)

## Fitur

- **Wizard setup**: token setup sekali pakai, akun admin, endpoint, interface, subnet, dan DNS.
- **Perangkat**: tambah, nonaktifkan, unduh profil, atau pindai QR dari HP.
- **Impor** dari `wg0.conf` atau database wireguard-ui, dengan pratinjau sebelum disimpan.
- **Ringkasan langsung** (SSE): siapa yang online, kecepatan 60 detik terakhir, total data.
- **Firewall nftables** dengan uji 60 detik: aturan baru batal otomatis kalau tidak dikonfirmasi.
- **Log sesi VPN**: terhubung, terputus, dan ganti IP publik, disimpan 90 hari.
- **Aktivitas jaringan** per perangkat: koneksi saat ini (conntrack) dan situs yang dibuka (riwayat DNS AdGuard).
- **AdGuard Home**: pasang, atur filter, DNS, layanan, klien, dan log, langsung dari panel.
- **Aman secara bawaan**: hanya mendengar di `127.0.0.1`, sesi + CSRF, CSP ketat, aset di-host sendiri.
- **Verifikasi dua langkah** (TOTP) untuk login admin, dengan 8 kode pemulihan sekali pakai.
- **Tanpa modul kernel pun jalan**: bila kernel tidak mendukung WireGuard, Umbra memakai `wireguard-go`
  (userspace) yang tertanam di binary.
- Antarmuka berbahasa Indonesia, tema terang/gelap, ramah HP.

## Syarat

- Linux dengan systemd dan akses root. Kernel 5.6+ (modul WireGuard bawaan) disarankan; tanpa modul itu
  Umbra memakai `wireguard-go` dan hanya butuh `/dev/net/tun` (umumnya ada di VPS KVM/LXC yang mengizinkan TUN).
- Port UDP 51820 terbuka di firewall penyedia VPS.
- amd64 atau arm64. RAM 1 GB cukup.

Kalau ada WireGuard lain yang memakai `wg0` atau port 51820, matikan dulu.

## Pasang di server

Setiap push ke `master`, GitHub Actions membuild frontend dan binary (amd64 dan arm64) lalu
menerbitkannya ke rilis [`latest`](https://github.com/luzyver/lzvr-umbra/releases/tag/latest) sebagai
`umbra-linux-<arch>.tar.gz` (+ `.sha256`).

### Cara 1: skrip update (disarankan, sekalian untuk update)

`update.sh` mengambil rilis `latest`, mengecek sha256, lalu menjalankan `install.sh`:

```sh
curl -fsSL https://raw.githubusercontent.com/luzyver/lzvr-umbra/master/deploy/update.sh | sudo sh
```

**Update** nanti: jalankan perintah yang sama. Pengaturan dan data tidak ditimpa.

<details>
<summary>Repo privat atau fork privat</summary>

Server butuh token untuk mengunduh rilis.

1. Buat token **fine-grained** di GitHub (Settings → Developer settings → Fine-grained tokens), hanya untuk
   repo itu, dengan izin **Contents: Read-only**.
2. Simpan di server:

   ```sh
   sudo install -d -m 0750 /etc/umbra
   sudo sh -c 'umask 077; cat > /etc/umbra/github-token'   # tempel token, Enter, lalu Ctrl+D
   ```

3. Jalankan (ganti `OWNER/REPO`):

   ```sh
   curl -fsSL -H "Authorization: Bearer $(sudo cat /etc/umbra/github-token)" \
     -H "Accept: application/vnd.github.raw" \
     https://api.github.com/repos/OWNER/REPO/contents/deploy/update.sh | sudo UMBRA_REPO=OWNER/REPO sh
   ```

</details>

### Cara 2: manual

1. Unduh `umbra-linux-amd64.tar.gz` dan `.sha256` dari rilis `latest` (`arm64` untuk VPS ARM):

   ```sh
   gh release download latest -R luzyver/lzvr-umbra -p 'umbra-linux-amd64.tar.gz*'
   ```

2. Salin ke server, lalu:

   ```sh
   sha256sum -c umbra-linux-amd64.tar.gz.sha256
   tar -xzf umbra-linux-amd64.tar.gz
   sudo sh umbra-linux-amd64/install.sh umbra-linux-amd64/umbra
   ```

   Tambahkan `--with-adguard` (`install.sh --with-adguard umbra-linux-amd64/umbra`) supaya AdGuard Home
   ikut dipasang begitu server WireGuard selesai diatur di panel.

3. File unduhan dan folder hasil ekstrak boleh dihapus.

### Yang dilakukan installer

Memasang nftables (wireguard-tools tidak perlu), memuat modul WireGuard dan TUN (gagal memuat modul tidak
menghentikan instalasi), menyalakan IP forwarding,
mengizinkan `wg0` dan 51820/udp di UFW bila aktif, membuat user `umbra`, memasang service, memasang helper
root kecil (`umbra-helper.path`/`.service`) untuk memasang dan mencopot AdGuard Home, lalu menampilkan
**token setup** (juga ada di `/var/lib/umbra/setup-token`). Aman dijalankan ulang.

Setelah terpasang, Umbra hanya memakai:

| Lokasi | Isi |
|---|---|
| `/usr/local/bin/umbra` | program |
| `/etc/systemd/system/umbra.service` | service systemd |
| `/etc/umbra/umbra.env` | pengaturan |
| `/var/lib/umbra` | database, token setup, `wg0.conf` cadangan |

### Cek dan uninstall

```sh
systemctl status umbra
journalctl -u umbra -f
```

Uninstall: `sudo systemctl disable --now umbra`, lalu hapus `/usr/local/bin/umbra`,
`/etc/systemd/system/umbra.service`, `/etc/umbra`, `/var/lib/umbra`, `/etc/sysctl.d/90-umbra.conf`,
`/etc/modules-load.d/umbra.conf`, dan jalankan `sudo nft delete table inet umbra`.

## Membuka panel

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
subnet VPN (mis. `10.8.0.1/24`), dan DNS klien. Setelah itu tambah perangkat dan pindai QR-nya.

Disarankan langsung menyalakan **verifikasi dua langkah** di Pengaturan → Keamanan: pindai QR dengan aplikasi
autentikator (Google Authenticator, Aegis, 2FAS, dll.), masukkan kode 6 digit, lalu simpan 8 kode pemulihan
yang muncul sekali saja. Tiap kode pemulihan hanya bisa dipakai sekali, sebagai ganti kode autentikator saat
masuk atau saat mematikan 2FA (yang juga meminta kata sandi).

## Pengaturan (`/etc/umbra/umbra.env`)

| Variabel | Bawaan | Arti |
|---|---|---|
| `UMBRA_LISTEN` | `127.0.0.1:8967` | alamat web panel |
| `UMBRA_DATA_DIR` | `/var/lib/umbra` | database, token setup, `wg0.conf` cadangan |
| `UMBRA_SECURE_COOKIE` | `true` | cookie hanya lewat HTTPS (localhost tetap bisa) |
| `UMBRA_TRUST_PROXY` | `false` | percayai `X-Real-IP` dari nginx |
| `UMBRA_ADGUARD_URL` | kosong | cadangan bila koneksi AdGuard belum disimpan dari panel |
| `UMBRA_ADGUARD_USER` / `UMBRA_ADGUARD_PASS` | kosong | login AdGuard Home (cadangan, sama seperti di atas) |
| `UMBRA_WG_BACKEND` | `auto` | `kernel`, `userspace` (`wireguard-go`), atau `auto`: kernel bila didukung, selain itu userspace. Backend aktif tampil di Pengaturan |
| `UMBRA_ADGUARD_AUTOINSTALL` | `false` | pasang AdGuard otomatis setelah WireGuard diatur (diisi `install.sh --with-adguard`) |

Setelah mengubah: `sudo systemctl restart umbra`.

## AdGuard Home

Semuanya bisa dari menu **AdGuard** di panel:

- **Pasang otomatis**: tekan *Pasang AdGuard* (atau pakai `install.sh --with-adguard`). Panel mengunduh
  AdGuard Home, menjalankan setup awal, membuat akun admin, lalu mengatur DNS supaya hanya mendengar di
  IP server VPN (mis. `10.8.0.1:53`, bukan `0.0.0.0`, supaya tidak jadi open resolver), web UI di
  `127.0.0.1:3000`, dan DNS bawaan perangkat diubah ke IP itu.
- **Sudah punya AdGuard**: isi alamat web dan akun admin di panel. Koneksi disimpan di database dan
  langsung berlaku tanpa restart; nilai `UMBRA_ADGUARD_*` di `umbra.env` hanya cadangan.
- **Pengaturan**: tab Ringkasan (proteksi, statistik, safe browsing, kontrol orang tua, safe search),
  Filter (blocklist, allowlist, aturan sendiri), DNS (upstream, bootstrap), Layanan (blokir
  YouTube/TikTok/dll.), Klien (aturan khusus per perangkat), Log (query log), dan Koneksi.
- **Copot**: di tab Koneksi, untuk AdGuard yang dipasang panel. DNS perangkat dikembalikan ke DNS bawaan server.

Panel mendaftarkan setiap perangkat di AdGuard dengan nama `Nama [wg#id]` dan mengatur blokir iklan per perangkat.

## Pindah dari WireGuard lama

Buka **Perangkat → Impor** untuk memindahkan perangkat dari `wg0.conf` server atau dari database
wireguard-ui (folder `db` dalam ZIP/TAR.GZ, atau beberapa berkas JSON perangkat). Periksa pratinjau sebelum
menyimpan. Yang dilewati: perangkat dengan kunci publik atau alamat yang sudah ada, alamat di luar subnet
server, dan kunci yang tidak valid.

- **`wg0.conf`**: centang "Gunakan kunci dan port server dari sumber" agar profil lama di perangkat tetap
  tersambung tanpa diubah. Berkas server tidak memuat kunci privat perangkat, jadi profil/QR-nya tidak bisa
  diunduh dari Umbra; bila perlu, pakai **Buat ulang kunci** lalu pindai profil baru di perangkat itu.
- **wireguard-ui**: cadangan memuat kunci privat perangkat, jadi profilnya langsung bisa diunduh lagi. Rute
  tambahan (`extra_allowed_ips`) tidak ikut diimpor.

Matikan WireGuard lama (`systemctl disable --now wg-quick@wg0`) sebelum Umbra memakai `wg0` dan port yang sama.

## Aktivitas jaringan & log sesi

Menu **Aktivitas** → tab **Aktivitas jaringan** (pilih perangkat) menampilkan apa yang dibuka perangkat itu:

- **Koneksi saat ini**: dibaca dari tabel koneksi kernel (conntrack): IP/domain tujuan, layanan
  (HTTPS, DNS, SSH, …), status, dan data terkirim/diterima per koneksi. Bisa disegarkan otomatis tiap 5 detik.
  Hitungan byte butuh `net.netfilter.nf_conntrack_acct=1` (dipasang otomatis oleh `install.sh`).
- **Situs yang dibuka (riwayat DNS)**: log query AdGuard untuk IP perangkat: domain apa saja, diblokir atau
  tidak, plus daftar domain tersering. Butuh AdGuard aktif dan perangkat memakai DNS AdGuard (opsi blokir iklan).

Hanya metadata yang terlihat (domain, IP, port, jumlah data); isi halaman tidak bisa dilihat karena HTTPS
terenkripsi.

Tab **Sesi VPN** menampilkan kapan perangkat terhubung, terputus, dan berganti IP publik (mis. pindah
dari Wi-Fi ke data seluler), lengkap dengan endpoint, durasi sesi, dan trafik sesi. WireGuard tidak punya
login/logout, jadi panel mengecek handshake tiap ±15 detik: perangkat dianggap terputus bila tidak
handshake selama 3 menit. Log tersimpan di database dan otomatis dihapus setelah 90 hari; bisa disaring
per perangkat/jenis kejadian dan dihapus manual.

## Pengembangan

Butuh Go 1.26+ dan Node 22+. Bisa di Windows/macOS juga, karena WireGuard dan firewall bisa diganti tiruan.

```sh
# terminal 1: backend dengan WireGuard & firewall tiruan (make dev)
UMBRA_DEV=1 UMBRA_FAKE_WG=1 UMBRA_FAKE_FIREWALL=1 go run ./cmd/umbra
# terminal 2: frontend dengan hot reload (proxy /api ke :8967)
cd web && npm ci && npm run dev
```

Token setup tercetak di log dan tersimpan di `./data/setup-token`.

| Perintah | Isi |
|---|---|
| `make build` | build frontend lalu binary Linux (`GOARCH=arm64 make build` untuk ARM) |
| `make check` | cek tipe frontend (svelte-check) |
| `make test` | `go test` dan test frontend |
| `make vet` | `go vet` |

Build manual tanpa `make`:

```sh
cd web && npm ci && npm run build && cd ..        # hasil ke internal/ui/dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o deploy/umbra ./cmd/umbra
```

### Struktur

```
cmd/umbra            main program
internal/api         HTTP API, sesi, CSRF, CSP
internal/auth        argon2id, token sesi, batas percobaan login, TOTP, kode pemulihan
internal/config      baca pengaturan UMBRA_*
internal/helper      helper root untuk pasang/copot AdGuard
internal/wg          WireGuard: kernel (wgctrl), userspace (wireguard-go), tiruan untuk dev
internal/firewall    nftables + uji 60 detik
internal/importer    baca wg0.conf dan database wireguard-ui
internal/adguard     klien REST AdGuard Home
internal/conntrack   koneksi aktif per perangkat
internal/store       SQLite (modernc, tanpa CGO)
internal/system      cek sistem untuk wizard
internal/ui          frontend hasil build (go:embed)
web/                 SvelteKit (Svelte 5, Tailwind 4)
deploy/              install.sh, update.sh, service systemd, file env
```

Catatan SvelteKit 3: alias `$lib` sudah tidak ada; pakai `#lib/...` (diatur di `imports` pada `web/package.json`).

## Kontribusi

Issue dan pull request diterima. Baca [CONTRIBUTING.md](CONTRIBUTING.md) dulu. Celah keamanan dilaporkan
secara privat sesuai [SECURITY.md](SECURITY.md).

## Lisensi

[MIT](LICENSE) © 2026 𝙻𝚄𝚉𝚈𝚅𝙴𝚁. Font Manrope memakai [SIL Open Font License](web/static/fonts/OFL-Manrope.txt).
WireGuard adalah merek dagang terdaftar milik Jason A. Donenfeld; Umbra tidak berafiliasi dengan proyek WireGuard
maupun AdGuard.
