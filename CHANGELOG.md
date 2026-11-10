# Changelog

All notable changes are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Umbra is a rolling release: every push to `master` republishes the `latest` release.

## [Unreleased]

### Added

- Two-factor authentication (TOTP) for the admin login: a QR code for authenticator apps, 8 one-time
  recovery codes (stored hashed), used codes are rejected, and turning it off needs the password + a code.
- Device import from `wg0.conf` and wireguard-ui databases (ZIP/TAR.GZ/JSON): preview first; duplicates,
  addresses outside the subnet, and invalid keys are skipped, then everything is saved in one transaction.
  Devices without a private key can get new keys ("Regenerate keys").
- `wireguard-go` (userspace) fallback when the kernel lacks WireGuard, controlled by
  `UMBRA_WG_BACKEND` (`auto`/`kernel`/`userspace`). The active backend is shown in Settings.
- MIT license, contributing guide, security policy, issue/PR templates, CI for pull requests,
  Dependabot, `Makefile`, `.editorconfig`, and `.gitattributes`.
- `npm test` for the frontend tests.
- A footer with the copyright and license in the panel and on the sign-in and setup pages.
- The README shows app previews (`docs/screenshots/`, regenerated with `web/tests/screenshots.py`) and
  architecture and usage-flow diagrams (`docs/diagrams/`).
- Code of conduct (`CODE_OF_CONDUCT.md`) and `.github/CODEOWNERS`.
- `master` branch ruleset (`.github/rulesets/master.json`), active on GitHub: pull requests and passing
  CI are required, with no direct or force pushes.

### Changed

- The interface, messages, documentation, and code comments are now in English (previously Indonesian).
- `install.sh` carries on when the WireGuard module cannot be loaded, and loads the TUN module; the system
  check shows a missing module as a warning, not a failure. The systemd service allows `/dev/net/tun`.
- `update.sh` no longer requires a GitHub token; a token is only used when present (private repo/fork).
- The `latest` release is published only after the check and all tests pass; the release archive now
  includes `LICENSE` and the font licenses.
- Overview shows service status, a traffic chart, and the device list with a status filter.
  Connection details, QR code, and device settings open in a side panel; on phones the panel fills the screen.
- The look was reworked into "rack label": a gray base, device names and active choices as label tape,
  and the Hanken Grotesk and Sometype Mono fonts (both embedded, their licenses ship in the release archive).
  Overview now opens with a port panel that maps the subnet addresses: a light is on for online devices,
  amber while there is traffic, and the next free address is printed above it.
  The device list shows data in and out per device.
- An update stream that drops or stops sending data is shown as an unknown status, so devices are not
  mistakenly shown as offline. Devices stay ordered by IP address.

## 2026-10-28

### Added

- Per-device network activity: active connections (conntrack) and AdGuard DNS history.
- AdGuard Home installed and managed from the panel, through a root helper.
- VPN session log (connected, disconnected, IP change) with 90-day retention.
- Docs and GitHub links in the sidebar.

### Changed

- A more minimal UI redesign with a warm neutral palette.

## 2026-10-27

### Added

- First release: setup wizard, device management with QR codes, live status (SSE), nftables firewall
  with a 60-second trial, AdGuard Home client, light/dark themes.
- GitHub Actions release workflow (amd64, arm64) and the `update.sh` script.

### Changed

- Default panel port from 8080 to 8967.

### Fixed

- WireGuard traffic is allowed when UFW is active.
- A `ServerForm` effect that kept retriggering itself.
