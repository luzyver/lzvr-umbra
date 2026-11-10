# Changelog

Semua perubahan penting dicatat di sini. Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/).
Umbra dirilis bergulir: setiap push ke `master` menerbitkan ulang rilis `latest`.

## [Belum dirilis]

### Ditambahkan

- Verifikasi dua langkah (TOTP) untuk login admin: QR untuk aplikasi autentikator, 8 kode pemulihan sekali
  pakai (disimpan ter-hash), kode yang sudah dipakai ditolak, dan mematikannya butuh kata sandi + kode.
- Impor perangkat dari `wg0.conf` dan database wireguard-ui (ZIP/TAR.GZ/JSON): pratinjau dulu, duplikat,
  alamat di luar subnet, dan kunci tidak valid dilewati, lalu disimpan dalam satu transaksi. Perangkat tanpa
  kunci privat bisa dibuatkan kunci baru ("Buat ulang kunci").
- Fallback `wireguard-go` (userspace) bila kernel tidak mendukung WireGuard, diatur dengan
  `UMBRA_WG_BACKEND` (`auto`/`kernel`/`userspace`). Backend aktif tampil di Pengaturan.
- Lisensi MIT, panduan kontribusi, kebijakan keamanan, template issue/PR, CI untuk pull request,
  Dependabot, `Makefile`, `.editorconfig`, dan `.gitattributes`.
- `npm test` untuk test frontend.
- Konfigurasi ruleset cabang `master` siap pasang (`.github/rulesets/master.json`): wajib pull request dan
  CI lulus, tanpa push langsung atau force push. Belum aktif di GitHub; butuh repo publik atau paket berbayar.

### Diubah

- `install.sh` tetap lanjut bila modul WireGuard tidak bisa dimuat, dan memuat modul TUN; cek sistem
  menampilkan modul yang hilang sebagai peringatan, bukan kegagalan. Service systemd mengizinkan
  `/dev/net/tun`.
- `update.sh` tidak lagi mewajibkan token GitHub; token hanya dipakai bila ada (repo privat/fork).
- Rilis `latest` baru diterbitkan setelah check dan semua test lulus; arsip rilis kini memuat `LICENSE`
  dan lisensi font.
- Ringkasan menampilkan status layanan, grafik trafik, dan daftar perangkat dengan filter status.
  Detail koneksi, QR, dan pengaturan perangkat dibuka dalam panel samping; di HP, panel memenuhi layar.
- Tampilan dirombak menjadi "label rak": dasar abu-abu, nama perangkat dan pilihan aktif berupa pita label,
  serta font Hanken Grotesk dan Sometype Mono (keduanya tertanam, lisensinya ikut di arsip rilis).
  Ringkasan kini dibuka dengan panel port yang memetakan alamat subnet: lampu menyala untuk perangkat
  online, kuning saat ada trafik, dan alamat kosong berikutnya tertulis di atasnya.
  Daftar perangkat menampilkan data masuk dan keluar per perangkat.
- Pembaruan yang terputus atau berhenti mengirim data ditampilkan sebagai status belum diketahui,
  agar perangkat tidak keliru terlihat offline. Urutan perangkat tetap berdasarkan alamat IP.

## 2026-10-28

### Ditambahkan

- Aktivitas jaringan per perangkat: koneksi aktif (conntrack) dan riwayat DNS AdGuard.
- AdGuard Home dipasang dan dikelola dari panel, lewat helper root.
- Log sesi VPN (terhubung, terputus, ganti IP) dengan retensi 90 hari.
- Tautan Dokumentasi dan GitHub di sidebar.

### Diubah

- Desain ulang UI yang lebih minimalis dengan palet netral hangat.

## 2026-10-27

### Ditambahkan

- Rilis pertama: wizard setup, kelola perangkat dengan QR, status langsung (SSE), firewall nftables
  dengan uji 60 detik, klien AdGuard Home, tema terang/gelap.
- Workflow rilis GitHub Actions (amd64, arm64) dan skrip `update.sh`.

### Diubah

- Port bawaan panel dari 8080 ke 8967.

### Diperbaiki

- Lalu lintas WireGuard diizinkan saat UFW aktif.
- Efek `ServerForm` yang memicu dirinya sendiri berulang.
