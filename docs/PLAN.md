# Caro Helper - Vision and Plan

Written 2026-09-16 from the design conversation. This is the "why and where to",
complementing `ARCHITECTURE.md` (the "how, today").

## The problem

Caro's daily work runs through several tools that do not talk to each other:

- **Smartsheet** - service scheduling data per customer/lab area, mostly as
  Reports (Forecast, VERS, UFP, ready to schedule, awaiting customer
  confirmation, status pipeline 03..13).
- **SAP Ariba** - purchase orders and procurement.
- **Two in-house web applications** (names to be confirmed).
- **Excel sheets** - working files she maintains or receives.

Routine questions ("which PO belongs to this asset", "what is open for lab X",
"which items still need a price release") mean opening several tools, clicking
through filters, and copying values by hand. Ten minutes of navigation for a
ten-second answer, several times a day.

## The idea

One small Windows program, started by double-click, that pulls the needed data
from these tools and answers such questions in one place, in a UI that feels
like the tools she already knows (Smartsheet first). No installation, no IT
involvement beyond what is already allowed on the company laptop.

Guiding principles agreed on:

1. **Start small, widen incrementally.** Each iteration must be useful on its
   own and be shown to Caro before the next one is planned.
2. **Caro's real questions drive the features.** The prototype exists to make
   her articulate them; the developer (her husband) does setup and maintenance.
3. **Self-contained and boring to deploy.** One exe, config in `%APPDATA%`,
   transfer by USB stick, no runtime dependencies.
4. **Read first, write later (if ever).** Pulling data has no risk; writing
   into company systems needs a separate decision.
5. **Secrets stay on the laptop.** Tokens are user-bound encrypted; the dev
   machine only ever holds a revocable token in a gitignored `.env`.

## Iterations

### Iteration 1 - Smartsheet reader (done)

Goal: convince Caro that this is faster than the browser.

- Local web UI, Smartsheet-like look, source rail / sheet panel / avatar menu.
- Token entry with verification, DPAPI storage.
- Browse workspaces, alias sheets and reports.
- Grid with live AND-search over all columns, sort, copy for Excel.
- Verified on the company laptop.

### Iteration 2 - Answer real questions (next)

Goal: turn Caro's recurring lookups into one-click answers.

- Sit with Caro, list her top 5-10 questions and where the answer lives today.
- **Predefined queries**: named filters on a grid (column / operator / value),
  initially defined in config, later editable in the UI. Example: "MedChem,
  price release missing" = `Preisfreigabe = false` on `VERS`.
- **Cross-grid lookup**: type a PO / asset tag / contract ID once, search all
  configured reports at the same time.
- Column chooser and remembered column order per grid; remember last search.
- Possibly: refresh all configured grids at startup and show counts per query in
  the sheet panel (a "dashboard" without building one).

### Iteration 3 - Excel

Goal: bring her working files next to the Smartsheet data.

- Read `.xlsx` files locally (Go library, no Excel automation needed).
- Register files like sheets: alias, path, sheet name, header row.
- Same grid, same search, same queries.
- First join: match Excel rows to Smartsheet rows on a shared key (PO, asset
  tag) and flag differences.

### Iteration 4 - SAP Ariba and the in-house web apps

Goal: cover the remaining sources. The approach is open until we know what
each system offers, evaluated in this order:

1. Official API or export endpoint (as with Smartsheet).
2. Scheduled export files (CSV/XLSX) the tool can read like Excel.
3. Browser automation as the last resort (brittle, may violate policies).

Ariba in particular has APIs but access is usually gated by the company's
procurement admin; this needs a conversation with IT before building anything.

### Ongoing - Quality of life

- Tray icon / hidden console once the tool is trusted.
- Version display and self-check page (token valid, sources reachable).
- Simple update path (copy new exe; config untouched).
- Optional: German UI, since the data and Caro's colleagues are German.

## Non-goals (for now)

- Multi-user or server deployment.
- Writing back into Smartsheet, Ariba, or the web apps.
- Replacing Smartsheet dashboards or reports; the tool reads them.
- Any form of automation that acts on Caro's behalf without a click.

## Open questions

- Which reports and questions matter most (Iteration 2 kick-off with Caro).
- Names and nature of the two in-house web apps.
- Whether the shared-mailbox Smartsheet account is the right identity to use
  long-term, or whether Caro should use a personal token.
- Whether IT has an opinion on a browser-like User-Agent (currently used) versus
  an honest `CaroHelper/x.y`.
