# Caro Helper

Single-exe Windows helper that reads Smartsheet sheets and reports through the REST API and
shows it in a local web UI (search, sort, copy to Excel).

Docs: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (decisions, components, API, security, roadmap),
[docs/smartsheet-layout.md](docs/smartsheet-layout.md) (Smartsheet structure, generated),
[docs/architecture.html](docs/architecture.html) (interactive diagram; source `docs/architecture.diagram.json`).

## Using it (Windows laptop)

1. Copy `carohelper.exe` anywhere (e.g. Desktop). Double-click it.
   A console window opens with log output and the browser opens
   `http://127.0.0.1:8765`. Close the console to stop.
2. First run: you land on **Settings** (later: avatar menu, top right). Paste a Smartsheet token
   (Smartsheet: Account -> Personal Settings -> API Access -> Generate).
   The token is checked against Smartsheet, then stored DPAPI-encrypted in
   `%APPDATA%\CaroHelper\config.json`.
3. **Browse workspaces**, add the sheets or reports you need, give them short names,
   **Save sheets**. They appear in the sheet panel.
4. Navigation: the narrow left rail selects the data source (Smartsheet active;
   SAP Ariba, Excel and two web apps are placeholders), the panel next to it
   lists that source's sheets/reports. The burger button (top left) hides both.
   Click a sheet: all rows load; the search box filters live across all columns
   (several words = AND), click a header to sort, **Copy for Excel** puts the
   visible rows on the clipboard as tab-separated text. **Light/Dark mode** is in
   the avatar menu.

Starting point for Caro: `2026 ESC Workspace - BI BC` -> `LEAN MedChem` ->
add `VERS`, `UFP`, `Forecast`.

Flags: `-port 9000`, `-no-browser`, `-config path\to\config.json`.

Optional `config.json` keys: `"port"`, `"userAgent"` (defaults to a current Edge-on-Windows string; bump the version there without rebuilding).

## Developing (Mac)

Requires Go and Node.

```sh
cp .env.example .env            # put SMARTSHEET_TOKEN in it (dev only)
make explore                    # docs/smartsheet-layout.md, structure only
make build-win                  # dist/carohelper.exe
make build                      # native binary for local testing
go test ./...                   # unit tests
cd frontend && npm run dev      # UI with hot reload; run `go run ./cmd/carohelper -no-browser` alongside
```

Layout:

- `cmd/carohelper` - the app; embeds `cmd/carohelper/dist` (built by Vite)
- `cmd/explore` - dumps the Smartsheet workspace/sheet/column structure
- `internal/smartsheet` - minimal API client
- `internal/config` - `%APPDATA%` config, DPAPI token protection
- `internal/server` - JSON API + SPA serving
- `frontend` - React + Vite + Tailwind + shadcn/ui
