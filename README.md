<p align="center">
  <img src="broffice.svg" width="96" height="96" alt="Branch Office Logo" />
</p>

<h1 align="center">Branch Office</h1>

<p align="center">
  <strong>A hyper-lightweight, mobile-friendly remote Git control center running locally on your host machine.</strong><br>
  <em>Your agents write it. You still commit it.</em>
</p>

<p align="center">
  <a href="https://branch-office.ishifishi.work">Website</a> •
  <a href="#screenshots">Screenshots</a> •
  <a href="#installation">Installation</a> •
  <a href="#features">Features</a> •
  <a href="#ssh--biometrics">SSH & Biometrics</a>
</p>

---

Designed to be securely exposed over private networks like Tailscale, Branch Office lets you review diffs, stage files
or individual hunks, safely discard changes, draft commits, push (with optional `--force-with-lease` safeguards), and
manage GitHub pull requests directly from your phone.

This project was born from a personal need / desire. I was doing a lot of programming from my phone with agents, but as
a git purist, I don't allow my agents to commit on my behalf and use [Secretive](https://github.com/maxgoedjen/secretive)
(for managed SSH keys within Secure Enclave) which requires biometrics, but I still wanted to be able to review diffs,
stage hunks/files, review changes, write commit messages from my phone, and do all of the other lifecycle things like
pushing and opening PRs from my phone as well.

Doing that via SSH is a bit of a pain, and I also wanted an interface that saved on typing.

---

## Screenshots

| Status & Staging | Diff & Hunk Staging | Mobile Commit Drawer | Push to Remote |
| :---: | :---: | :---: | :---: |
| <img src="screenshots/shot-status.png" width="220" alt="Status & Staging" /> | <img src="screenshots/shot-diff.png" width="220" alt="Diff & Hunks" /> | <img src="screenshots/shot-commit.png" width="220" alt="Commit Drawer" /> | <img src="screenshots/shot-push.png" width="220" alt="Push to Remote" /> |

---

## Features

- **Single Binary**: The entire mobile Vue 3 SPA is embedded into a compiled Go binary (`~9MB`). No external node runtime, python, or server dependencies needed in production.
- **Multi-Repository Workspace**: Seamlessly switch between multiple local Git projects from the mobile top bar. Register new directory paths or remove projects directly from the UI.
- **First-Class Git Worktrees**: Switch between linked worktrees, create new worktrees (with automatic branch checkout), and remove worktrees directly from mobile with dedicated live change detection.
- **Git Stash Interface**: Save uncommitted changes (with untracked files by default), inspect stash diffs, apply or pop stashes, and switch branches cleanly.
- **Git Tags & Release Management**: Create lightweight or annotated tags with version messages, and push tags directly to remote (`origin`) in one tap.
- **Full File & Hunk-Level Staging**: Stage whole files or inspect color-coded unified diffs and stage/unstage individual diff hunks with a tap.
- **Safe Discard Workflow**: Revert changes in modified files, untracked files, or specific diff hunks with deliberate confirmation safeguards.
- **Mobile-Optimized Commits**: Fixed bottom drawer with auto-sizing commit message input and amend toggles within thumb reach.
- **GitHub Copilot Commit Messages**: One-tap AI commit message generation from your staged diff via GitHub CLI (`gh copilot`) with author hint support.
- **Remote Synchronization & Safety**: Check ahead/behind commits and push upstream. Includes a guarded `--force-with-lease` button with explicit two-step confirmation dialogs.
- **GitHub Pull Requests**: Direct integration with the GitHub CLI (`gh`) to view existing PR status and open new PRs or draft PRs from your device.

---

## Installation

### Homebrew

```bash
brew install jejacks0n/tap/broffice
```

or

```bash
brew tap jejacks0n/tap
brew install broffice
```

### Build from Source

Requirements: Go 1.22+ and Node 18+ (for compiling the frontend).

```bash
# Build the embedded frontend and compile the Go binary
make build
```

The output executable is created at `bin/broffice`.

### Run Locally

```bash
# Automatically registers and opens the current Git repo
./bin/broffice

# Or specify a target repository path
./bin/broffice -dir /path/to/my-repo

# Or accept connections from this machine only
./bin/broffice -local
```

By default the server listens on all interfaces (`0.0.0.0:8080`), so your phone can reach it over your LAN or tailnet.
Every API request needs the token printed in the startup banner (see [Security & Remote Access](#security--remote-access)).
Pass `-local` (or set `BROFFICE_LOCAL=1`) to bind `127.0.0.1` only. It keeps the port from `-addr`, so
`-addr 0.0.0.0:9090 -local` listens on `127.0.0.1:9090`.

### Access over Tailscale

To access Branch Office from your phone or tablet on your Tailscale tailnet:

```bash
# Option A: Expose via Tailscale Serve (recommended: adds HTTPS and keeps Branch Office off your LAN)
./bin/broffice -local
tailscale serve --bg 8080

# Option B: Access directly via your machine's Tailscale IP or MagicDNS (the default binding)
./bin/broffice
# http://<machine-name>:8080 or http://100.x.y.z:8080
```

Both options work with token authentication: on first visit, open the URL with the `#token=...` fragment from the
startup banner appended (e.g. `http://<machine-name>.ts.net/#token=...`). When listening on all interfaces (the default), the banner
also prints a QR code of the Bonjour URL with the token embedded — scan it with your phone to sign in instantly.

---

## Security & Remote Access

Branch Office has shell-equivalent power over your repositories — it stages, discards, commits, and pushes — so the
server locks itself down by default while staying reachable from wherever you work (localhost, phone, or Tailscale):

* **Token authentication (on by default)**: A 256-bit token is generated on first launch and stored in
  `~/.config/broffice/config.json` (written with mode `0600`). The startup banner prints the token; append it to the
  URL as a fragment (`http://<host>:8080/#token=...`). Fragments are never sent to the server, and the
  app strips it from the URL immediately and remembers the token in `localStorage`. All API requests then carry it via
  the `X-Broffice-Token` header.
* **Scannable sign-in**: When listening on the LAN (the default), the banner prints a QR code encoding the Bonjour
  (`.local`) URL with the token embedded as a `?token=` query parameter — scan it with your phone's camera to sign in
  instantly. The SPA accepts `?token=` the same way as the `#token=` fragment.
* **Local-only is one flag away**: The server listens on all interfaces by default, protected by the token. Pass
  `-local` (or `BROFFICE_LOCAL=1`) to accept connections from this machine only, and reach it from elsewhere through a
  proxy such as `tailscale serve`.
* **No cross-origin API access**: The API no longer returns CORS headers and rejects cross-origin mutations and
  non-JSON bodies, so a malicious website cannot trigger git actions from your browser.
* **Clickjacking protection**: Responses deny being framed (`frame-ancestors 'none'`, `X-Frame-Options: DENY`).

### Using a token with curl

```bash
TOKEN=...   # from the startup banner
curl -H "X-Broffice-Token: $TOKEN" http://127.0.0.1:8080/api/repos
# or: curl -H "Authorization: Bearer $TOKEN" ...
```

### Disabling authentication

```bash
./bin/broffice -no-auth        # or: export BROFFICE_NO_AUTH=true
```

> [!WARNING]
> `-no-auth` removes the only thing standing between the network and your repositories. Branch Office has
> shell-equivalent power, so with authentication off, anyone who can reach the port can stage, discard, commit, and
> push in every registered repository.
>
> The server listens on all interfaces by default, which means `-no-auth` on its own exposes it to everyone on your
> network. Pair it with `-local`, or only use it behind an access layer you trust (Tailscale ACLs, a VPN).
>
> Cross-origin requests from web pages are still rejected in this mode, but that does not stop another machine or a
> local process from calling the API directly. The startup banner prints a warning when authentication is off and the
> server is reachable beyond loopback.

### Regenerating the token

Delete the `token` field from `~/.config/broffice/config.json` and restart; a new token is generated and printed in
the banner.

### A note on TLS

Branch Office does not serve TLS itself. Requests over plain HTTP on an untrusted LAN can be sniffed, token included.
For remote access prefer `tailscale serve` (HTTPS, tailnet-only) or `tailscale funnel` (public HTTPS) so the token is
never sent in cleartext.

---

## SSH & Biometrics

If you use **SecretAgent**, **1Password SSH Agent**, or hardware security keys on your host machine, Git operations
normally require a physical fingerprint or password prompt. When accessing Branch Office remotely, you cannot provide
biometric authorization, which causes pushes or commits to block or fail.

Branch Office solves this with **process-level isolation**: your host machine continues using SecretAgent for regular
terminal and IDE work, while Branch Office uses a dedicated key for remote Git commands.

### Generate a Dedicated Remote Key

Create an unencrypted SSH key specifically for Branch Office:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/broffice_ed25519 -C "branch-office-remote" -N ""
```

### Add Key to GitHub

Register the key twice: once to authenticate pushes, and once as a signing key so GitHub can verify the commits
Branch Office signs with it.

```bash
gh ssh-key add ~/.ssh/broffice_ed25519.pub --title "Branch Office Host"
gh ssh-key add ~/.ssh/broffice_ed25519.pub --title "Branch Office Host (signing)" --type signing
```

### Start Branch Office with the Key

```bash
./bin/broffice -ssh-key ~/.ssh/broffice_ed25519
```

* **`-ssh-key <path>`**: Injects `GIT_SSH_COMMAND="ssh -i <path> -o IdentitiesOnly=yes"` and clears `SSH_AUTH_SOCK` for Git commands executed by Branch Office, completely bypassing SecretAgent. If your Git config signs commits, Branch Office signs them with this same key, so nothing prompts for a fingerprint and GitHub can verify them.
* **`-no-sign`** (optional): Disables commit and tag signing entirely, so commits made from Branch Office are unsigned. It is not needed to avoid biometric prompts when `-ssh-key` is set. Use it if you would rather not sign from your phone, or if you did not register the key as a signing key. Without `-ssh-key`, it is what stops SecretAgent or GPG from prompting for Touch ID on commit.

### Optional Environment Variables & Persistence

You can also set these options via environment variables:

```bash
export BROFFICE_SSH_KEY="~/.ssh/broffice_ed25519"
# export BROFFICE_NO_SIGN="true"   # only to disable signing
./bin/broffice
```

Settings passed via flags or environment variables are automatically saved to `~/.config/broffice/config.json` for
subsequent launches.

---

## Development

Run the Go backend and Vite dev server concurrently:

```bash
# Terminal 1: Backend
go run ./cmd/broffice -addr 127.0.0.1:8080

# Terminal 2: Frontend with hot-reloading
cd web && npm run dev
```

Visit `http://localhost:5173` in your browser. Requests to `/api/*` are automatically proxied to the Go backend.

Token authentication applies in development too. On first visit, append the token from the Go server's startup banner
to the Vite URL: `http://localhost:5173/#token=<token>` (a `?token=` query parameter works too).

---

## Tests

Run the backend test suite:

```bash
make test
# or
go test -v ./...
```

---

## License

Branch Office is released under the MIT license:

* https://opensource.org/licenses/MIT

Copyright 2026 &copy; [jejacks0n](https://github.com/jejacks0n)

## Make Code Not War ♥
