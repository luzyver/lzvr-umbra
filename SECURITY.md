# Kebijakan keamanan

Umbra mengelola VPN, firewall, dan DNS dengan hak root, jadi laporan keamanan kami tangani lebih dulu.

## Versi yang didukung

Hanya rilis [`latest`](https://github.com/luzyver/lzvr-umbra/releases/tag/latest) (build terbaru dari
`master`). Perbaikan keamanan tidak di-backport.

## Melaporkan celah

**Jangan** buka issue publik. Laporkan lewat
[GitHub private vulnerability reporting](https://github.com/luzyver/lzvr-umbra/security/advisories/new).

Sertakan:

- jenis celah dan dampaknya (mis. bypass login, CSRF, eksekusi perintah, kebocoran kunci),
- langkah untuk mengulang atau proof of concept,
- versi/commit dan sistem server.

Target: balasan pertama dalam 7 hari, dan perbaikan atau rencana mitigasi dalam 30 hari untuk celah yang
terkonfirmasi. Pelapor akan disebut di advisory kecuali minta anonim.

## Di luar cakupan

- Panel yang sengaja dibuka ke internet tanpa HTTPS atau reverse proxy (lihat README, *Membuka panel*).
- Celah di WireGuard, AdGuard Home, atau kernel; laporkan ke proyek masing-masing.
- Serangan yang butuh akses root di server.
