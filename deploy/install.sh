#!/bin/sh
# Installer Umbra. Jalankan sebagai root di server Linux:
#   sh install.sh [path/ke/binary-umbra]
# Aman dijalankan ulang (untuk update binary).
set -eu

BIN_SRC="${1:-$(dirname "$0")/umbra}"
HERE="$(cd "$(dirname "$0")" && pwd)"

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mGagal:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "jalankan sebagai root (sudo sh install.sh)"
[ "$(uname -s)" = "Linux" ] || die "hanya untuk Linux"
[ -f "$BIN_SRC" ] || die "binary tidak ditemukan: $BIN_SRC (build dulu: lihat README)"
command -v systemctl >/dev/null || die "butuh systemd"

# WireGuard diatur langsung lewat netlink/wgctrl, jadi wireguard-tools tidak perlu.
say "Memasang paket (nftables)"
if command -v apt-get >/dev/null; then
	DEBIAN_FRONTEND=noninteractive apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nftables
elif command -v dnf >/dev/null; then
	dnf install -y -q nftables
elif command -v apk >/dev/null; then
	apk add -q nftables
else
	command -v nft >/dev/null || die "pasang paket nftables secara manual lalu ulangi"
fi

say "Memuat modul kernel WireGuard"
modprobe wireguard 2>/dev/null || die "modul wireguard tidak ada di kernel ini (butuh kernel 5.6+)"
echo wireguard > /etc/modules-load.d/umbra.conf

say "Mengaktifkan IP forwarding"
cat > /etc/sysctl.d/90-umbra.conf <<'SYSCTL'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
SYSCTL
sysctl -q -p /etc/sysctl.d/90-umbra.conf

if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q '^Status: active'; then
	# Tabel nftables panel tidak bisa mengalahkan DROP milik UFW, jadi izinkan lewat UFW juga.
	say "UFW aktif: mengizinkan forward dari wg0 dan port UDP ${UMBRA_WG_PORT:-51820}"
	ufw route allow in on wg0 >/dev/null
	ufw allow "${UMBRA_WG_PORT:-51820}/udp" >/dev/null
fi

say "Membuat user sistem umbra"
id umbra >/dev/null 2>&1 || useradd --system --no-create-home --home-dir /var/lib/umbra --shell /usr/sbin/nologin umbra

say "Memasang binary dan service"
systemctl stop umbra 2>/dev/null || true
install -m 0755 "$BIN_SRC" /usr/local/bin/umbra
install -m 0644 "$HERE/umbra.service" /etc/systemd/system/umbra.service
install -d -m 0750 -o root -g umbra /etc/umbra
if [ ! -f /etc/umbra/umbra.env ]; then
	install -m 0640 -o root -g umbra "$HERE/umbra.env" /etc/umbra/umbra.env
else
	say "  /etc/umbra/umbra.env sudah ada, tidak ditimpa"
fi

systemctl daemon-reload
systemctl enable --now umbra
sleep 2
systemctl is-active --quiet umbra || { journalctl -u umbra -n 30 --no-pager; die "service tidak mau jalan"; }

LISTEN="$(sed -n 's/^UMBRA_LISTEN=//p' /etc/umbra/umbra.env | tail -n1)"
TOKEN_FILE=/var/lib/umbra/setup-token
echo
say "Selesai. Panel berjalan di http://${LISTEN:-127.0.0.1:8967}"
if [ -f "$TOKEN_FILE" ]; then
	echo "    Token setup (dipakai sekali untuk membuat admin):"
	echo "    $(cat "$TOKEN_FILE")"
fi
echo "    Dari laptop: ssh -L 8967:${LISTEN:-127.0.0.1:8967} user@server  lalu buka http://localhost:8967"
echo "    Jangan lupa buka port UDP WireGuard (bawaan 51820) di firewall penyedia VPS."
