#!/bin/sh
# Downloads the newest Umbra build from GitHub (the "latest" release, made by CI) and installs it.
# Run as root:
#   sudo sh update.sh
# A public repo needs no token. A private repo (e.g. a fork) needs a GitHub token (fine-grained,
# read-only Contents for that repo) in /etc/umbra/github-token (chmod 600) or the GITHUB_TOKEN variable.
set -eu

REPO="${UMBRA_REPO:-luzyver/lzvr-umbra}"
TOKEN_FILE=/etc/umbra/github-token

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mFailed:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo sh update.sh)"
command -v curl >/dev/null || die "curl is required"

case "$(uname -m)" in
	x86_64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	*) die "architecture $(uname -m) is not supported yet" ;;
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

# Finds an asset id by name in the "latest" release (without jq).
asset_id() {
	printf '%s' "$RELEASE" | tr -d '\n' |
		grep -o "\"url\": *\"[^\"]*/releases/assets/[0-9]*\"[^}]*\"name\": *\"$1\"" |
		grep -o 'assets/[0-9]*' | head -n1 | cut -d/ -f2
}

say "Fetching the latest release info from $REPO"
RELEASE="$(api "https://api.github.com/repos/$REPO/releases/tags/latest")" || die "latest release not found (has CI run? a private repo needs a token)"
TAR_ID="$(asset_id "$NAME.tar.gz")"
SUM_ID="$(asset_id "$NAME.tar.gz.sha256")"
[ -n "$TAR_ID" ] && [ -n "$SUM_ID" ] || die "asset $NAME is missing from the latest release"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cd "$TMP"

say "Downloading $NAME.tar.gz"
api -H "Accept: application/octet-stream" -o "$NAME.tar.gz" "https://api.github.com/repos/$REPO/releases/assets/$TAR_ID"
api -H "Accept: application/octet-stream" -o "$NAME.tar.gz.sha256" "https://api.github.com/repos/$REPO/releases/assets/$SUM_ID"
sha256sum -c "$NAME.tar.gz.sha256" >/dev/null || die "checksum mismatch"

tar -xzf "$NAME.tar.gz"
sh "$NAME/install.sh" "$NAME/umbra"
