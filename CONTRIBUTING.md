# Contributing

Thanks for wanting to help with Umbra.

## Before you start

- For large changes, open an issue first so the direction is agreed.
- Do not report vulnerabilities in public issues; see [SECURITY.md](SECURITY.md).
- Everyone taking part is expected to follow the [code of conduct](CODE_OF_CONDUCT.md).

## Setting up

You need Go 1.26+ and Node 22+. WireGuard and nftables are not needed: use the test mode.

```sh
make dev                       # fake backend on 127.0.0.1:8967
cd web && npm ci && npm run dev  # frontend with hot reload
```

## Before opening a pull request

```sh
make check   # svelte-check
make test    # go test + frontend tests
make vet     # go vet
```

CI runs the same on every pull request.

To check the Overview and Devices UI, run the frontend with `npm run dev`, then from the repo root:

```sh
python web/tests/device-ui.py
```

This browser test needs Python, Playwright, and Chromium installed. Every API request is replaced with
test data, including adding, editing, and deleting devices. The backend does not need to run.
It covers filters, the details panel, keyboard navigation, live/disconnected status, and phone screens;
screenshots are saved locally in `.impeccable/review/`. Use `--url` if the frontend port differs.

## README screenshots

If your change alters how Overview, device details, Activity, or the sign-in page look, regenerate the
images in `docs/screenshots/` (frontend running with `npm run dev`, same requirements as the browser test):

```sh
python web/tests/screenshots.py
```

The images use sample data, so no real addresses or device names are captured.

The README diagrams are drawn as SVG in `docs/diagrams/*.html` (colors and fonts follow the panel).
After changing them, export the light and dark PNGs again:

```sh
python docs/diagrams/export.py
```

## `master` branch rules

The `master` ruleset has been active on GitHub since 10 November 2026, installed from
[`.github/rulesets/master.json`](.github/rulesets/master.json). The file in the repo is only the source;
changes to it take effect once the ruleset is updated through the API (see below).
Check what is in force: `gh api repos/luzyver/lzvr-umbra/rules/branches/master`.

These rules apply to everyone, admins included:

- no direct pushes; every change goes through a pull request,
- the `test` CI check must pass and the branch must be up to date with `master`,
- all review conversations must be resolved,
- merges are *squash* or *rebase* only (linear history), with no force pushes and no branch deletion.

Workflow:

```sh
git switch -c fix/short-name
git push -u origin fix/short-name
gh pr create --fill
gh pr merge --squash --delete-branch   # once CI is green
```

Installing or updating the ruleset (repo admin, needs `gh`):

```sh
gh api -X POST repos/luzyver/lzvr-umbra/rulesets --input .github/rulesets/master.json
# update: find the id with `gh api repos/luzyver/lzvr-umbra/rulesets`, then
gh api -X PUT repos/luzyver/lzvr-umbra/rulesets/<id> --input .github/rulesets/master.json
```

## Style

- **Go**: `gofmt`, errors returned with context, no CGO (the binary must be static).
- **Frontend**: Svelte 5 runes, one component per responsibility in `web/src/lib`. Colors and sizes
  use the tokens in `web/src/design.css`, not raw values.
- **UI text** is in English, in sentence case. Say "device" (not "client") and "Add device".
- Assets (fonts, icons) are self-hosted; the panel's CSP forbids CDNs.
- Check UI changes on a phone screen and in the dark theme.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/) format: `feat:`, `fix:`, `refactor:`, `docs:`,
`ci:`, `chore:`. The first sentence says what changed, e.g.
`fix: Allow WireGuard traffic through active UFW rules`.

## License

By submitting a contribution, you agree that it is released under the [MIT license](LICENSE).
