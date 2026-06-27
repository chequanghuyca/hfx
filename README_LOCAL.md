# NOFX Local Source

This folder is a trimmed local-only copy of `/Volumes/USB/nofx-bot`.

## What is included

- Go backend runtime source and tests.
- Web frontend source, public assets, and Vite/Tailwind/TypeScript config.
- Local SQLite `.env` with fresh local encryption keys.
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

Runtime files are created under `data/` and ignored by git.
