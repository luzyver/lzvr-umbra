# Design

Umbra memakai gambaran **papan kunci resepsionis**: rel kayu gelap tempat kunci digantung, label kuningan
bernomor, dan kertas linen hangat. Dunia ini hanya menyumbang palet, tipografi, kepadatan, dan satu gerak khas;
navigasi dan kontrol tetap standar web.

## Warna (token di `web/src/design.css`)

- **Rel** (`--rail*`): sidebar desktop, bilah atas dan tab bawah di HP, sisi kiri login. Gelap di kedua tema.
- **Dasar**: linen `--bg`, panel `--panel`/`--panel-alt`, garis `--border`/`--border-strong`, teks `--text`/`--muted`.
  Tidak ada hitam/putih murni. Skala `zinc` Tailwind di `app.css` dipetakan ke nada yang sama.
- **Kuningan** (`--accent`): hanya untuk tombol utama, menu aktif, dan label perangkat yang online.
  `--accent-ink`/`--focus` = versi gelap untuk teks dan cincin fokus di latar terang.
- **Status**: `--green` (online), `--red` (galat/hapus). Grafik: `--chart-rx` kuningan, `--chart-tx` teal.
- Aksen ungu/biru dilarang (keputusan pengguna).

## Tipografi

Manrope (di-host sendiri, 400 dan 600–800), angka tabular di seluruh halaman. Judul halaman 22px/700,
judul seksi 15px/700, isi 13–14px. Monospace hanya untuk IP, kunci, dan data.

## Komponen

- Kartu: radius 12px, garis 1px, tanpa bayangan. Dialog satu-satunya yang berbayangan.
- Tombol: `.btn-primary` kuningan dengan teks gelap, `.btn-ghost` bergaris, `.btn-danger` merah.
- Tab segmen `.filter-tabs`, tabel `.data-table`, status `.status` + titik.

## Gerak khas: papan kunci (`web/src/lib/KeyRack.svelte`)

Di Ringkasan tiap perangkat adalah label kunci bernomor oktet terakhir IP-nya. Online = label kuningan penuh,
offline = label kertas, nonaktif = garis putus-putus dan nama dicoret. Urutan: online dulu lalu nomor IP,
sehingga label hanya bergeser (animate:flip) saat status berubah. Label yang statusnya baru berubah berayun
dari lubangnya dan diberi cincin sampai disorot/difokus. Semua gerak mati saat `prefers-reduced-motion`.

## Responsif

Di bawah 768px sidebar diganti bilah atas + tab bawah lima menu (terjangkau jempol); toast naik di atas tab.
