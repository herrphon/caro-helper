# ADR 0001 - SAP Ariba access via browser-session replay

Status: Accepted (spike stage)
Date: 2026-09-17
Deciders: project owner (internal; no IT involvement)

## Context

Caro uses SAP Ariba (Guided Buying / Ariba Buying, buyer realm) daily for
purchase orders and procurement. We want Caro Helper to show the same PO data in
its existing search/sort/copy-to-Excel grid, mirroring the Smartsheet flow
(look up POs/requisitions by number, supplier, cost centre; pull a filtered list
into Excel).

Constraints that shaped this decision:

- **No official API.** SAP Ariba's documented APIs (Operational/Analytical
  Reporting, Ariba Open APIs on the Business Accelerator Hub) require an
  application registered on SAP's developer portal and **approved by the
  customer realm's Ariba admin**, who issues OAuth client credentials. Caro's
  personal login does not grant API access.
- **No IT contact.** We will not raise a request with procurement/IT. That rules
  out the entire registered-app / client-credentials path.
- **SSO.** The realm is fronted by corporate SAML SSO (Azure AD / Entra). We do
  not want to re-implement login.
- **Single Go exe, pure Go.** `make build-win` builds with `CGO_ENABLED=0`; any
  solution must keep that (no cgo).
- **Should look like her own traffic.** The tool should issue the same requests
  her browser already makes, from the same machine, so it is indistinguishable
  from normal use.

Research (`docs/research-ariba-browser-session-auth.md`) established that the
buyer UI is a same-origin SPA whose XHRs are authenticated by a **server-side
session keyed to a session-ID cookie** on the realm origin
(`*.procurement.ariba.com`). After SSO there is a single reusable realm session
cookie. **Exact cookie names, any CSRF header, and useful endpoints are NOT
documented by SAP** and must be captured live from Caro's realm. Session idle TTL
is ~30 min.

## Decision

Ride Caro's already-authenticated Edge session:

1. Caro logs into Ariba in Edge as normal (SSO handled by the browser).
2. The helper reads Edge's own cookie jar for the realm host and replays the same
   requests from Go's `http.Client`, on the same machine, with her User-Agent.
3. Cookies stay fresh automatically because she uses Ariba normally; nothing is
   stored long-term and no second login secret is needed.

This is **option (a) cookie-jar import** from the design discussion. Rejected
alternatives: driving/embedding a live browser (option b - heavier dependency,
kept as fallback if replay from a separate client is blocked, e.g. by App-Bound
Encryption or IP binding); manual cURL paste (option c - poor UX). The official
OAuth API path is rejected outright per the "no IT" constraint.

### First step (this spike): capture-and-probe

Because endpoints are unknown until captured, the honest first milestone is not a
coded API call but a **capture-and-probe** tool that proves session replay works:

- `cmd/ariba-probe` (throwaway, like `cmd/explore`) reads a git-ignored
  `ariba-probe.json` describing one captured authenticated request.
- On Windows it decrypts Edge's cookies for the realm host (see below) and
  replays that request, printing: master-key decrypt result (or **ABE detected ->
  loud failure**), count of realm cookies, HTTP status, final URL (an IdP
  redirect ⇒ session not accepted), and the first ~200 bytes of the body.
- No secrets are printed (only cookie names + value lengths).

### Edge cookie decryption (Windows)

Edge (Chromium) stores cookies in a SQLite DB
(`%LOCALAPPDATA%\Microsoft\Edge\User Data\<profile>\Network\Cookies`), each value
AES-256-GCM encrypted (`v10`/`v11` prefix | 12-byte nonce | ciphertext | 16-byte
tag). The AES key is in `...\User Data\Local State` as
`base64("DPAPI" + dpapi_blob)`, decryptable with `CryptUnprotectData` under the
current user. The DB is copied to a temp file and opened read-only to dodge the
lock while Edge runs. Pure Go: `modernc.org/sqlite` (no cgo) + `golang.org/x/sys/windows`.

**App-Bound Encryption (ABE):** newer Edge builds may wrap the key with an
`APPB` prefix instead of `DPAPI`. We detect this and fail loudly rather than
return garbage; if it turns up, we switch to the live-browser fallback (option b).

### Storage

Realm host and probe request live in git-ignored `ariba-probe.json` (may reveal
internal URLs). Cookies are read live from Edge and never persisted. If we later
cache anything sensitive, reuse the existing generic DPAPI `protect`/`unprotect`
in `internal/config`.

## How to capture `ariba-probe.json`

On Caro's laptop, in Edge:

1. Log into Ariba (Guided Buying / Buying) as normal.
2. Press **F12** → **Network** tab. Filter to **Fetch/XHR**.
3. Click around so the SPA issues requests (open a PO list, a search). Pick a
   lightweight authenticated GET that returns JSON (a bootstrap / user-profile /
   list call is ideal).
4. Right-click it → **Copy** → **Copy as cURL** (or read the request line
   manually). From it, fill in:
   - `realmHost` - the request's host, e.g. `prod3.procurement.ariba.com`.
   - `probeURL` - the full request URL.
   - `headers` - non-cookie headers worth mirroring (`Accept`, and any
     `X-CSRF-Token` / Ariba-specific header). **Do not** copy the `Cookie`
     header; cookies come from Edge.
   - `edgeProfile` - the Edge profile dir, usually `Default` (check
     `edge://version` → "Profile Path").
5. Save as `ariba-probe.json` (git-ignored) next to the repo root. Start from
   `ariba-probe.example.json`.
6. Run on the laptop: `go run ./cmd/ariba-probe`.

Interpreting output: **HTTP 200 with JSON on the realm origin = session replay
works.** A redirect to the IdP host, or an HTML login page, means the cookies
were not accepted (expired session, missing cookie, or CSRF/UA/IP check).

## Consequences

- Positive: no IT, no OAuth app, indistinguishable from her own traffic, stays in
  the single pure-Go exe, cookies auto-refresh.
- Negative / risks: undocumented and unsupported by SAP (may break on Ariba UI
  changes); ~30 min idle TTL means she must have used Ariba recently; ABE or
  IP-binding could block pure-Go replay and force the live-browser fallback;
  reading the browser cookie store is a grey-area capability and must remain
  strictly read-only and same-user.
- This ADR covers only the spike. Wiring Ariba into the backend source
  abstraction, per-source config, the grid contract, and the frontend rail entry
  are deferred until the probe confirms replay works.

## Dependency note: modernc.org/sqlite Dependabot alerts

Adding `modernc.org/sqlite` (pure-Go, no cgo - needed to read Edge's cookie DB)
pulls in a large build/codegen dependency tree (`modernc.org/cc`, `ccgo`,
`libc`, `golang.org/x/tools`). GitHub Dependabot flags advisories against
packages in that tree.

`govulncheck ./...` reports **no vulnerabilities reachable from our code**: the
flagged packages are the codegen toolchain behind `libc`, not runtime code any of
our call paths execute. We accept the alerts for the spike rather than hand-roll a
SQLite reader. Re-run `govulncheck ./...` before promoting the spike into the
shipped app; if a reachable vuln appears, either bump the dependency or replace it
with a minimal Chromium-Cookies-table reader.
