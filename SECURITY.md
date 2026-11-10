# Security policy

Umbra manages a VPN, a firewall, and DNS with root privileges, so security reports are handled first.

## Supported versions

Only the [`latest`](https://github.com/luzyver/lzvr-umbra/releases/tag/latest) release (the newest build
from `master`). Security fixes are not backported.

## Reporting a vulnerability

Do **not** open a public issue. Report it through
[GitHub private vulnerability reporting](https://github.com/luzyver/lzvr-umbra/security/advisories/new).

Include:

- the kind of vulnerability and its impact (e.g. login bypass, CSRF, command execution, key leak),
- steps to reproduce or a proof of concept,
- the version/commit and the server system.

Target: a first reply within 7 days, and a fix or mitigation plan within 30 days for confirmed
vulnerabilities. Reporters are credited in the advisory unless they ask to stay anonymous.

## Out of scope

- A panel deliberately exposed to the internet without HTTPS or a reverse proxy (see the README,
  *Opening the panel*).
- Vulnerabilities in WireGuard, AdGuard Home, or the kernel; report them to those projects.
- Attacks that need root access on the server.
