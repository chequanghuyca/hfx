# NOFX Local Source

This folder is a trimmed local-only copy of `/Volumes/USB/nofx-bot`.

## What is included

- Go backend runtime source and tests.
- Web frontend source, public assets, and Vite/Tailwind/TypeScript config.
- Local `.env` is ignored by git and should point to PostgreSQL for runtime.
- Original `README.md`, `LICENSE`, and encryption notes.

## What was intentionally left out

- `.git`, `.github`, `.husky`, `.vscode`
- `node_modules`, built frontend output, generated binaries, `build/`
- Docker, Kubernetes, Railway, nginx, deploy/install scripts
- screenshots, old logs, old `data/`, backtests, desktop/Wails build entrypoint

## Run locally

Prerequisites:

- Go matching `go.mod`
- Node.js 18+
- TA-Lib installed on the machine

Install dependencies:

```bash
go mod download
cd web && npm install
```

Start backend:

```bash
go run main.go
```

Start frontend in another terminal:

```bash
cd web && npm run dev
```

Open http://127.0.0.1:3000.

Runtime logs are created under `data/` and ignored by git.

## Run through Cloudflare Tunnel

This local copy includes a Windows launcher matching the previous `nofx-bot`
setup. In VSCode Integrated Terminal, run the PowerShell script directly:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-bot-cloudflare.ps1
```

Use `.\start-bot-cloudflare.bat` only when launching from Explorer/cmd, because
the batch wrapper pauses for input.

The launcher starts:

- backend on `http://127.0.0.1:8080`
- frontend on `http://127.0.0.1:3000`
- the existing `cloudflared` Windows service, if installed
- logs under `data/backend.*.log` and `data/frontend.*.log`

The local `.env` uses PostgreSQL credentials from the previous `nofx-bot`
environment with a separate `DB_SCHEMA` for this clean copy, so it does not
attach to the old live schema or auto-start old traders.
