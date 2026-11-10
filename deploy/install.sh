#!/bin/sh
# Umbra installer. Run as root on a Linux server:
#   sh install.sh [--with-adguard] [path/to/umbra-binary]
# --with-adguard: install AdGuard Home automatically once WireGuard is set up in the panel.
# Safe to run again (to update the binary).
set -eu

WITH_ADGUARD=false
BIN_SRC=""
for arg in "$@"; do
	case "$arg" in
		--with-adguard) WITH_ADGUARD=true ;;
		-*) printf 'Unknown option: %s
' "$arg" >&2; exit 1 ;;
		*) BIN_SRC="$arg" ;;
	esac
done
BIN_SRC="${BIN_SRC:-$(dirname "$0")/umbra}"
HERE="$(cd "$(dirname "$0")" && pwd)"

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mFailed:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo sh install.sh)"
[ "$(uname -s)" = "Linux" ] || die "Linux only"
[ -f "$BIN_SRC" ] || die "binary not found: $BIN_SRC (build it first: see the README)"
command -v systemctl >/dev/null || die "systemd is required"

# WireGuard is controlled directly through netlink/wgctrl, so wireguard-tools is not needed.
say "Installing packages (nftables)"
if command -v apt-get >/dev/null; then
	DEBIAN_FRONTEND=noninteractive apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq nftables
elif command -v dnf >/dev/null; then
	dnf install -y -q nftables
elif command -v apk >/dev/null; then
	apk add -q nftables
else
	command -v nft >/dev/null || die "install the nftables package manually, then try again"
fi

say "Loading the WireGuard kernel module"
if modprobe wireguard 2>/dev/null; then
	printf 'wireguard\ntun\nnf_conntrack\n' > /etc/modules-load.d/umbra.conf
else
	say "The WireGuard module could not be loaded. Umbra will use userspace wireguard-go if the kernel lacks WireGuard (auto mode)."
	printf 'tun\nnf_conntrack\n' > /etc/modules-load.d/umbra.conf
fi
modprobe tun 2>/dev/null || true
modprobe nf_conntrack 2>/dev/null || true

say "Enabling IP forwarding"
cat > /etc/sysctl.d/90-umbra.conf <<'SYSCTL'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
# Count bytes & start time per connection for the Network activity page.
net.netfilter.nf_conntrack_acct = 1
net.netfilter.nf_conntrack_timestamp = 1
SYSCTL
sysctl -q -e -p /etc/sysctl.d/90-umbra.conf

if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q '^Status: active'; then
	# The panel's nftables table cannot override UFW's DROP, so allow it through UFW as well.
	say "UFW is active: allowing forwarding from wg0 and UDP port ${UMBRA_WG_PORT:-51820}"
	ufw route allow in on wg0 >/dev/null
	ufw allow "${UMBRA_WG_PORT:-51820}/udp" >/dev/null
fi

say "Creating the umbra system user"
id umbra >/dev/null 2>&1 || useradd --system --no-create-home --home-dir /var/lib/umbra --shell /usr/sbin/nologin umbra

say "Installing the binary and service"
systemctl stop umbra 2>/dev/null || true
install -m 0755 "$BIN_SRC" /usr/local/bin/umbra
install -m 0644 "$HERE/umbra.service" /etc/systemd/system/umbra.service
install -d -m 0750 -o root -g umbra /etc/umbra
if [ ! -f /etc/umbra/umbra.env ]; then
	install -m 0640 -o root -g umbra "$HERE/umbra.env" /etc/umbra/umbra.env
else
	say "  /etc/umbra/umbra.env already exists, not overwritten"
fi
if [ "$WITH_ADGUARD" = true ]; then
	say "AdGuard Home will be installed automatically once WireGuard is set up in the panel"
	sed -i '/^#* *UMBRA_ADGUARD_AUTOINSTALL=/d' /etc/umbra/umbra.env
	echo "UMBRA_ADGUARD_AUTOINSTALL=true" >> /etc/umbra/umbra.env
fi

# Small root helper that installs/removes AdGuard Home from the panel. The panel
# (user umbra) writes request.json; the path unit runs "umbra helper".
say "Installing the AdGuard helper"
install -d -m 0700 -o umbra -g umbra /var/lib/umbra /var/lib/umbra/helper
cat > /etc/systemd/system/umbra-helper.path <<'UNIT'
[Unit]
Description=Umbra - watch for helper requests

[Path]
PathExists=/var/lib/umbra/helper/request.json

[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/umbra-helper.service <<'UNIT'
[Unit]
Description=Umbra - root helper (install/remove AdGuard Home)

[Service]
Type=oneshot
User=root
EnvironmentFile=-/etc/umbra/umbra.env
ExecStart=/usr/local/bin/umbra helper
UNIT

systemctl daemon-reload
systemctl enable --now umbra-helper.path
systemctl enable --now umbra
sleep 2
systemctl is-active --quiet umbra || { journalctl -u umbra -n 30 --no-pager; die "the service did not start"; }

LISTEN="$(sed -n 's/^UMBRA_LISTEN=//p' /etc/umbra/umbra.env | tail -n1)"
TOKEN_FILE=/var/lib/umbra/setup-token
echo
say "Done. The panel is running at http://${LISTEN:-127.0.0.1:8967}"
if [ -f "$TOKEN_FILE" ]; then
	echo "    Setup token (used once to create the admin):"
	echo "    $(cat "$TOKEN_FILE")"
fi
echo "    From your laptop: ssh -L 8967:${LISTEN:-127.0.0.1:8967} user@server  then open http://localhost:8967"
echo "    Remember to open the WireGuard UDP port (default 51820) in the VPS provider's firewall."
