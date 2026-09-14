# Caro Helper

Single-exe Windows helper that reads Smartsheet sheets and reports through the REST API and
shows it in a local web UI (search, sort, copy to Excel).

## Using it (Windows laptop)

1. Copy `carohelper.exe` anywhere (e.g. Desktop). Double-click it.
   A console window opens with log output and the browser opens
   `http://127.0.0.1:8765`. Close the console to stop.
2. First run: go to **Settings**, paste a Smartsheet token
   (Smartsheet: Account -> Personal Settings -> API Access -> Generate).
   The token is stored DPAPI-encrypted in `%APPDATA%\CaroHelper\config.json`.
3. **Browse workspaces**, add the sheets or reports you need, give them short names,
   **Save sheets**. They appear in the sidebar.

Flags: `-port 9000`, `-no-browser`, `-config path\to\config.json`.

## Developing (Mac)

Requires Go and Node.

```sh
cp .env.example .env            # put SMARTSHEET_TOKEN in it (dev only)
make explore                    # docs/smartsheet-layout.md, structure only
make build-win                  # dist/carohelper.exe
make build                      # native binary for local testing
cd frontend && npm run dev      # UI with hot reload; run `go run ./cmd/carohelper -no-browser` alongside
```

Layout:

- `cmd/carohelper` - the app; embeds `cmd/carohelper/dist` (built by Vite)
- `cmd/explore` - dumps the Smartsheet workspace/sheet/column structure
- `internal/smartsheet` - minimal API client
- `internal/config` - `%APPDATA%` config, DPAPI token protection
- `internal/server` - JSON API + SPA serving
- `frontend` - React + Vite + Tailwind + shadcn/ui
