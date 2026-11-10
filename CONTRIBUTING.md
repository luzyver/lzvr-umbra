# Kontribusi

Terima kasih sudah mau membantu Umbra.

## Sebelum mulai

- Untuk perubahan besar, buka issue dulu supaya arahnya disepakati.
- Celah keamanan jangan lewat issue publik; lihat [SECURITY.md](SECURITY.md).
- Semua yang ikut serta diharapkan mengikuti [kode etik](CODE_OF_CONDUCT.md).

## Menyiapkan lingkungan

Butuh Go 1.26+ dan Node 22+. WireGuard dan nftables tidak perlu: pakai mode uji.

```sh
make dev                       # backend tiruan di 127.0.0.1:8967
cd web && npm ci && npm run dev  # frontend dengan hot reload
```

## Sebelum membuka pull request

```sh
make check   # svelte-check
make test    # go test + test frontend
make vet     # go vet
```

CI menjalankan hal yang sama di setiap pull request.

Untuk memeriksa UI Ringkasan dan Perangkat, jalankan frontend dengan `npm run dev`, lalu dari root repo:

```sh
python web/tests/device-ui.py
```

Tes browser ini membutuhkan Python, Playwright, dan Chromium yang sudah terpasang. Semua permintaan API
diganti dengan data uji, termasuk tambah, ubah, dan hapus perangkat. Backend tidak perlu dijalankan.
Tes mencakup filter, panel detail, navigasi keyboard, status langsung/terputus, dan layar HP;
tangkapan layar disimpan lokal di `.impeccable/review/`. Gunakan `--url` jika port frontend berbeda.

## Tangkapan layar README

Kalau perubahanmu mengubah tampilan Ringkasan, detail perangkat, Aktivitas, atau halaman masuk, buat ulang
gambar di `docs/screenshots/` (frontend berjalan dengan `npm run dev`, syaratnya sama dengan tes browser):

```sh
python web/tests/screenshots.py
```

Gambar memakai data contoh, jadi tidak ada alamat atau nama perangkat asli yang ikut terekam.

## Aturan cabang `master`

> **Status: belum aktif.** [`.github/rulesets/master.json`](.github/rulesets/master.json) baru berupa
> konfigurasi siap pasang; file di repo tidak mengaktifkan apa pun, dan push langsung ke `master` masih
> bisa sampai ruleset dipasang lewat API (lihat di bawah). Ruleset hanya bisa dipasang bila repo **publik**
> atau pemiliknya memakai paket berbayar (GitHub Pro/Team); repo privat di akun gratis ditolak (HTTP 403).
> Cek apakah sudah aktif: `gh api repos/luzyver/lzvr-umbra/rulesets`.

Setelah dipasang, aturan ini berlaku untuk semua orang, termasuk admin:

- tidak bisa push langsung; semua perubahan lewat pull request,
- cek CI `test` harus lulus dan cabang harus terbaru dengan `master`,
- semua diskusi review harus selesai,
- merge hanya *squash* atau *rebase* (riwayat lurus), tanpa force push dan tanpa hapus cabang.

Alur kerja:

```sh
git switch -c fix/nama-perbaikan
git push -u origin fix/nama-perbaikan
gh pr create --fill
gh pr merge --squash --delete-branch   # setelah CI hijau
```

Memasang atau memperbarui ruleset (admin repo, butuh `gh`):

```sh
gh api -X POST repos/luzyver/lzvr-umbra/rulesets --input .github/rulesets/master.json
# perbarui: cari id dengan `gh api repos/luzyver/lzvr-umbra/rulesets`, lalu
gh api -X PUT repos/luzyver/lzvr-umbra/rulesets/<id> --input .github/rulesets/master.json
```

## Gaya

- **Go**: `gofmt`, error dikembalikan dengan konteks, tanpa CGO (binary harus statis).
- **Frontend**: Svelte 5 runes, satu komponen per tanggung jawab di `web/src/lib`. Warna dan ukuran
  memakai token di `web/src/design.css`, bukan nilai mentah.
- **Teks UI** berbahasa Indonesia. Pakai istilah "perangkat" (bukan "klien") dan "Tambah perangkat".
- Aset (font, ikon) di-host sendiri; CSP panel melarang CDN.
- Cek perubahan UI di layar HP dan tema gelap.

## Pesan commit

Format [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `refactor:`, `docs:`,
`ci:`, `chore:`. Kalimat pertama menjelaskan apa yang berubah, mis.
`fix: Allow WireGuard traffic through active UFW rules`.

## Lisensi

Dengan mengirim kontribusi, kamu setuju kontribusi itu dirilis di bawah [lisensi MIT](LICENSE).
