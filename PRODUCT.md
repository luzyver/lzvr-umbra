# Product

<!-- impeccable:product-schema 1 -->

> Disusun dari isi repo dan riwayat chat (Okt 2026), tanpa sesi wawancara. Bagian bertanda *(inferensi)* belum dikonfirmasi pengguna.

## Platform

web

## Users

Satu admin (pemilik VPS) yang mengelola server WireGuard pribadinya dan perangkat keluarga/teman/kantor kecil yang memakainya. Dibuka sesekali dari laptop atau HP untuk: menambah perangkat dan memindai QR, melihat siapa yang online, mengecek trafik, dan melihat aktivitas jaringan per perangkat. *(inferensi: skala puluhan perangkat, bukan ratusan)*

## Product Purpose

Umbra adalah panel web untuk WireGuard di satu VPS Linux: wizard setup, kelola perangkat (profil + QR), status dan trafik langsung (SSE), firewall nftables dengan uji 60 detik, log sesi VPN, aktivitas jaringan per perangkat (conntrack + riwayat DNS AdGuard), dan AdGuard Home yang dipasang serta dikelola dari panel. Sukses = admin bisa menjawab "siapa yang tersambung, sedang apa, dan aman tidak" dalam sekali lihat, dan menambah perangkat dalam kurang dari satu menit.

## Positioning

Satu binary Go tanpa dependensi di server (frontend di-embed), bahasa Indonesia, dan menggabungkan VPN + blokir iklan/DNS + aktivitas per perangkat dalam satu tempat, tidak seperti wg-easy atau panel WireGuard lain yang berhenti di daftar klien.

## Operating Context

Dijalankan sebagai service systemd di VPS (RAM bisa 1 GB). Diakses lewat browser, sering dari HP. Rilis lewat GitHub Actions. Mode uji `UMBRA_FAKE_WG` untuk pengembangan.

## Capabilities and Constraints

- SvelteKit (Svelte 5) + Tailwind 4, adapter-static, hasil build ke `internal/ui/dist` (go:embed). CSP ketat: aset font/ikon harus di-host sendiri, tanpa CDN.
- Rute: Ringkasan, Perangkat, Aktivitas (Sesi VPN, Aktivitas jaringan), AdGuard (beberapa tab), Pengaturan (server, firewall), Login, Setup.
- Istilah UI: "Perangkat", "Tambah perangkat" (bukan "klien"). Teks UI berbahasa Indonesia.
- Tema: Sistem → Terang → Gelap dalam satu tombol ikon.

## Brand Commitments

- Nama "umbra" (bayangan inti gerhana); logo bulan sabit gerhana di `web/src/lib/Logo.svelte`.
- Pengguna menolak aksen ungu/biru/iris ("AI slop"), menolak widget/hiasan tak berguna, label huruf besar, dan teks penjelas di judul halaman. Hindari hitam/putih murni.

## Evidence on Hand

Tidak ada data pengguna, testimoni, atau angka nyata. Data contoh hanya dari mode uji.

## Product Principles

1. Status dulu: online/offline dan trafik langsung harus terbaca tanpa klik.
2. Tenang dan ringkas: tidak ada elemen yang tidak menjawab pertanyaan admin.
3. Aman secara bawaan: aksi berisiko (hapus, firewall) selalu bisa dibatalkan atau diuji dulu.
4. Ramah HP: semua tugas utama bisa dikerjakan dari layar kecil.
