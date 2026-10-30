#!/bin/sh
# Unduh build terbaru Umbra dari GitHub (rilis "latest", dibuat oleh CI) lalu pasang.
# Jalankan sebagai root:
#   sudo sh update.sh
# Repo publik tidak butuh token. Repo privat (mis. fork) butuh token GitHub (fine-grained,
# hanya baca Contents repo itu) di /etc/umbra/github-token (chmod 600) atau variabel GITHUB_TOKEN.
set -eu

REPO="${UMBRA_REPO:-luzyver/lzvr-umbra}"
TOKEN_FILE=/etc/umbra/github-token

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mGagal:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "jalankan sebagai root (sudo sh update.sh)"
command -v curl >/dev/null || die "butuh curl"

case "$(uname -m)" in
	x86_64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	*) die "arsitektur $(uname -m) belum didukung" ;;
esac
NAME="umbra-linux-$ARCH"

TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ] && [ -f "$TOKEN_FILE" ]; then TOKEN="$(tr -d ' \r\n' < "$TOKEN_FILE")"; fi

api() {
	if [ -n "$TOKEN" ]; then
		curl -fsSL -H "Authorization: Bearer $TOKEN" -H "X-GitHub-Api-Version: 2022-11-28" "$@"
	else
		curl -fsSL -H "X-GitHub-Api-Version: 2022-11-28" "$@"
	fi
}

# Cari id aset berdasarkan nama di rilis "latest" (tanpa jq).
asset_id() {
	printf '%s' "$RELEASE" | tr -d '\n' |
		grep -o "\"url\": *\"[^\"]*/releases/assets/[0-9]*\"[^}]*\"name\": *\"$1\"" |
		grep -o 'assets/[0-9]*' | head -n1 | cut -d/ -f2
}

say "Mengambil info rilis latest dari $REPO"
RELEASE="$(api "https://api.github.com/repos/$REPO/releases/tags/latest")" || die "rilis latest tidak ditemukan (CI sudah jalan? repo privat butuh token)"
TAR_ID="$(asset_id "$NAME.tar.gz")"
SUM_ID="$(asset_id "$NAME.tar.gz.sha256")"
[ -n "$TAR_ID" ] && [ -n "$SUM_ID" ] || die "aset $NAME tidak ada di rilis latest"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"

say "Mengunduh $NAME.tar.gz"
api -H "Accept: application/octet-stream" -o "$NAME.tar.gz" "https://api.github.com/repos/$REPO/releases/assets/$TAR_ID"
api -H "Accept: application/octet-stream" -o "$NAME.tar.gz.sha256" "https://api.github.com/repos/$REPO/releases/assets/$SUM_ID"
sha256sum -c "$NAME.tar.gz.sha256" >/dev/null || die "checksum tidak cocok"

tar -xzf "$NAME.tar.gz"
sh "$NAME/install.sh" "$NAME/umbra"
