# Research: How SAP Ariba (Guided Buying / Buyer UI) authenticates its own in-browser XHR

**Scope:** the buyer realm procurement web UI at `*.ariba.com` and the session it uses for
same-origin XHR — **not** the OAuth "Ariba Open APIs / Business Accelerator Hub" developer program.

**Goal:** understand what a helper tool would need to replay the requests a logged-in browser makes.

**Date:** 2026-09-16

---

## Honesty preface

SAP publishes almost nothing about the *internal* wire format of the guided buying / buyer
UI (cookie names, CSRF header names, private JSON endpoints). That surface is an
implementation detail, not a documented contract. Everything below is separated into:

- **Documented** — stated in a primary SAP / Microsoft source (cited).
- **Inferred** — reasoned from SAP platform conventions or the SAML flow docs.
- **Unknown** — must be confirmed by inspecting real browser DevTools traffic on the
  target realm; do not assume.

Cookie/header names in particular are **Unknown** from public docs and MUST be captured
live per-realm. Do not hardcode guesses.

---

## 1. Is the buyer UI Fiori/UI5 or a custom SPA? What backend does the browser call?

**Findings:**

- **Guided buying is a custom SAP SPA**, not classic SAPUI5/Fiori-elements. It presents a
  "Fiori-compliant" look but is a separate front end from the older SAP Ariba Buying &
  Invoicing (B&I) UI. SAP treats them as two distinct apps with cross-navigation between
  them ("Action Tiles" / "View in SAP Ariba Procurement" links jump from guided buying to
  B&I and back).
  - Source: SAP KBA 3324069, *"Users are facing redirecting and navigation issues in
    Guided Buying"* — explicitly distinguishes guided buying from "SAP Ariba Buying (B&I)"
    and describes cross-navigation between the two.
    <https://userapps.support.sap.com/sap/support/knowledge/en/3324069>
  - Confidence: **High** that they are two distinct front ends; **Medium** on the exact
    JS framework (SAP has publicly described guided buying as React-based in conference
    material, but I did not find a citable primary doc naming the framework — treat framework
    identity as **Inferred**).

- **Realm host patterns (Documented).** The buyer/procurement realms live on
  `*.procurement.ariba.com` / `*.procurement-2.ariba.com` / `*.procurement-eu.ariba.com`
  (EU data center), with sourcing/supplier on `*.sourcing.ariba.com` /
  `*.supplier.ariba.com`. Microsoft's Entra SSO tutorial lists these exact SP URL patterns:
  - `http://<subdomain>.procurement-2.ariba.com` (SAML Entity ID)
  - Reply URLs: `https://<subdomain>.ariba.com/CUSTOM_URL`,
    `https://<subdomain>.procurement-eu.ariba.com`, `https://<subdomain>.procurement-2.ariba.com`, ...
  - Sign-on: `https://<subdomain>.sourcing.ariba.com`
  - Source: Microsoft Learn, *Configure Ariba for single sign-on with Microsoft Entra ID*
    <https://learn.microsoft.com/en-us/entra/identity/saas-apps/ariba-tutorial>
  - Confidence: **High** for these host families. Guided buying itself is typically reached
    via a deep path on the buyer realm host; the exact path prefix is **Unknown** from docs.

- **Data-center naming (Documented, partial).** Realms are pinned to a data center
  (e.g. "Prod3"). Source: SAP KBA 3324069 mentions "Prod3 Data center". The `s1.ariba.com`
  / `service.ariba.com` names in the question are older/legacy or Ariba Network fronting
  hosts; I found no primary doc confirming they front the current guided buying XHR API, so
  treat those specific hostnames as **Unknown/legacy**.

**The XHR backend is same-origin** with the realm host the SPA is served from (classic SPA
pattern). No public doc contradicts this; the SAML flow (below) sets the session cookie on
that same realm origin.

---

## 2. How is the browser session authenticated to those XHR endpoints?

**Documented (session model):**

- On first contact, **Ariba creates a web session and sets a session-ID cookie in the
  browser**, storing the intended destination URL server-side against that session. After
  SAML completes, the web server "maps the connection to the correct session based on the
  session ID in the browser cookie."
  - Quote (SAP support note KB0397704, *SAML Authentication for Ariba Buyer*):
    > "Ariba creates a web session, sets the session ID cookie in the user's browser and
    > saves the location of the final destination URL in the session."
    > "Ariba web server maps the connection to the correct session based on the session ID
    > in the browser cookie."
  - Source: <https://support.ariba.com/item/view/176057> (a.k.a. KB0397704)
  - Confidence: **High**. This means subsequent same-origin XHRs are authenticated **by that
    session cookie alone** — a classic server-side session, not a bearer token in JS.

- **The `awr` / `realm` URL parameters are real and load-bearing (Documented).** Ariba
  buyer URLs carry `realm=<RealmName>` and the SAML consumption URL carries `awr=1`:
  - Sample SPID URL: `https://mycompany.com/Buyer/Main?realm=System&passwordadapter=PasswordAdapter1`
  - Sample SAML response consumption URL: `https://mycompany.com/Buyer/Main/ad/samlAuth/SSOActions?awr=1&realm=System`
  - Source: KB0397704 §2.1 (same URL as above). This is the origin of the "awr_..." pattern
    in the question. `awr` = "Ariba Web Request" style routing flag; `realm` selects the
    tenant. Confidence: **High** these params exist; **Medium** on precise semantics.

**Unknown / must capture live:**

- **Exact session cookie name(s).** SAP docs say "the session ID cookie" without naming it.
  Common observed possibilities on Ariba (unverified here): `AWSELB`/`AWSALB` (AWS ELB
  stickiness, *not* the auth session), plus an Ariba-app session cookie. **Do not assume a
  name** — inspect `document.cookie` / the `Set-Cookie` on the realm origin.
- **CSRF/anti-CSRF header.** SAP NetWeaver/Gateway apps use the `X-CSRF-Token` fetch pattern
  (GET with `X-CSRF-Token: Fetch` → token echoed back → sent on writes). **Guided buying is
  not NetWeaver Gateway**, so this pattern is **not confirmed** for the Ariba realm. Whether
  the buyer SPA uses `X-CSRF-Token`, a custom header, or a per-request nonce in the body is
  **Unknown from docs** and must be observed. Confidence that some anti-CSRF mechanism exists
  on state-changing calls: **Medium-High** (modern SAP apps generally have one); the concrete
  header name: **Unknown**.
- SAML assertion cookies (from the IdP) live on the **IdP** origin, not the Ariba realm, and
  are not needed for replay once the Ariba session cookie exists.

---

## 3. SSO: after SAML completes, is there a single reusable Ariba session cookie?

**Documented — yes, effectively.** The flow (KB0397704 §1):

1. Browser hits the buyer/sourcing realm URL.
2. Ariba looks up the auth method (SAML "Corporate Authentication"), creates a session,
   sets the session-ID cookie, redirects the browser to the corporate IdP relay page.
3. IdP (e.g. Azure AD / Entra ID, or SAP IAS) authenticates the user and POSTs a signed
   SAML Response back to the Ariba return URL (`.../SSOActions?awr=1&realm=...`).
4. Ariba verifies the signature, binds the authenticated NameID to the **existing session**,
   and lands the user. "The user can continue using Ariba as an authenticated user."

- Ariba supports **SP-initiated SSO** (Documented, Microsoft Learn tutorial: "Ariba supports
  **SP** initiated SSO").
- Source: <https://support.ariba.com/item/view/176057>,
  <https://learn.microsoft.com/en-us/entra/identity/saas-apps/ariba-tutorial>
- Confidence: **High**.

**Implication for a helper tool:** once the user's browser has completed SSO, there is a
single server-side session on the Ariba realm origin, keyed by the session cookie. Replaying
that cookie on same-origin XHRs should authenticate — *subject to §5 caveats*. The tool does
**not** need to understand SAML at all; it needs the post-SSO realm cookie jar. The cleanest,
most robust design is to ride the user's actual browser session (e.g. via the browser, an
extension, or reading its cookie jar for the realm origin) rather than re-implementing login.

---

## 4. Is there a documented lightweight "authenticated 200" endpoint (health / whoami / version)?

**Unknown / none documented.** I found **no** primary SAP source documenting a public
"whoami", user-profile, version, or health endpoint on the guided buying / buyer realm that
is intended for client use and returns 200 only when authenticated.

- The guided buying **Administration** area and its public *Open APIs* (Business Accelerator
  Hub) are OAuth-based and out of scope per the question.
- Practical approach (Inferred, must verify live): pick any small same-origin JSON XHR the
  SPA itself issues on load (e.g. a user-context / bootstrap / preferences call observed in
  DevTools) and use it as the "am I still authenticated?" probe. Expect **302/redirect to the
  IdP** or a 401/403 when the session is invalid, and 200 when valid. Confirm the actual
  unauth behavior empirically — Ariba tends to **redirect** unauthenticated navigations rather
  than return a clean 401, which matters for how the tool detects logged-out state.
- Confidence: **High** that nothing is publicly documented; **Medium** that a suitable
  bootstrap XHR exists to repurpose.

---

## 5. What might break session replay from a different HTTP client?

**Documented:**

- **Short idle TTL.** "The web session is configured to be expired after **30 minutes of idle
  time**." (KB0397704 §1.4). So a replayed cookie is only good for ~30 min of inactivity;
  keep-alive activity extends it. Confidence: **High** (note: this text is from the Ariba
  Buyer on-premise SAML doc; the exact cloud value may differ but 30 min idle is the stated
  norm). Treat the number as **Documented-ish / verify per realm**.

**Inferred / Unknown (must test):**

- **CSRF token on writes.** If the buyer SPA requires an anti-CSRF header/token, a raw HTTP
  client that only replays cookies will get GETs but fail POST/PUT/DELETE until it also
  fetches and echoes the token. **Likely** for state-changing calls; header name **Unknown**.
  Confidence: **Medium-High** that writes are protected; **Unknown** on mechanism.
- **User-Agent / header fingerprinting.** No SAP doc confirms UA checks, but WAF/Akamai-style
  fronting (common for `*.ariba.com`) can reject clients with missing/odd `User-Agent`,
  `Origin`, `Referer`, or `Sec-Fetch-*` headers. Safest to **mirror the browser's headers**.
  Confidence: **Medium** that header hygiene matters; **Unknown** on specifics.
- **IP binding.** Not documented. Some SAP session configs bind a session to the originating
  IP; if enabled, replaying from a different host/IP than the browser breaks. Running the
  helper **on the same machine as the browser** avoids this risk. Confidence: **Low-Medium**;
  **Unknown** whether Ariba enables it — assume it might.
- **Load-balancer stickiness.** AWS ELB/ALB stickiness cookies may be required to land on the
  node holding the session; capture and replay the full cookie set, not just the app cookie.
  Confidence: **Medium**; **Inferred** from AWS-hosted architecture.
- **Per-request nonces.** Not documented; possible in some flows. **Unknown** — observe.

---

## Consolidated findings (with confidence)

| # | Finding | Confidence |
|---|---------|------------|
| 1 | Guided buying is a custom SPA, distinct from Ariba Buying (B&I); "Fiori-compliant" look, cross-navigation between the two | High (two apps) / Medium (framework = React, inferred) |
| 1 | Realm hosts: `*.procurement(-2/-eu).ariba.com` (buyer), `*.sourcing/supplier.ariba.com`; DC-pinned (e.g. Prod3) | High |
| 1 | `s1.ariba.com` / `service.ariba.com` as the GB XHR backend | Unknown / likely legacy |
| 2 | Auth is a **server-side session keyed by a session-ID cookie** set on the realm origin; same-origin XHR rides that cookie | High |
| 2 | URLs carry `realm=<Name>` and SAML return uses `awr=1` (`.../ad/samlAuth/SSOActions?awr=1&realm=...`) | High |
| 2 | Exact cookie name(s) and CSRF header name | Unknown — capture live |
| 3 | SP-initiated SAML SSO to corporate IdP (Entra ID / SAP IAS); after completion a single reusable realm session cookie exists | High |
| 4 | A documented authenticated whoami/health/version endpoint | None found; repurpose a bootstrap XHR (verify) |
| 4 | Unauth requests tend to **redirect to IdP** rather than clean 401 | Medium — verify |
| 5 | ~30 min idle session TTL | High (per SAML doc) / verify per realm |
| 5 | Anti-CSRF token likely required on writes | Medium-High / mechanism Unknown |
| 5 | UA/header fingerprinting, IP binding, LB stickiness may break replay | Low-Medium / Unknown — mitigate by running on same machine + mirroring browser headers + full cookie jar |

## Design takeaways for the helper tool

- Don't re-implement SAML. Ride the **already-authenticated browser session** on the realm
  origin (browser automation / extension / cookie-jar reuse). This sidesteps IdP, IP binding,
  and header-fingerprint risks.
- Replay the **entire** cookie set for the realm origin (app session + any LB stickiness),
  not a single guessed cookie.
- Mirror the browser's `User-Agent`, `Origin`, `Referer`, `Sec-Fetch-*` headers.
- Discover the real endpoints, cookie names, and CSRF mechanism by **observing DevTools /
  the network tab on the actual target realm** — none of these are contractually documented.
- Expect session death after ~30 min idle and redirect-to-IdP on expiry; detect that and
  prompt the user to re-auth in the browser.

## Sources

- SAP support note KB0397704 / item 176057, *SAML Authentication for Ariba Buyer On-Premise*
  — session cookie creation, SAML flow, `realm`/`awr` params, 30-min idle TTL.
  <https://support.ariba.com/item/view/176057>
- Microsoft Learn, *Configure Ariba for single sign-on with Microsoft Entra ID* — realm host
  patterns, SP-initiated SSO. <https://learn.microsoft.com/en-us/entra/identity/saas-apps/ariba-tutorial>
- SAP KBA 3324069, *Redirecting and navigation issues in Guided Buying* — guided buying vs
  B&I as distinct apps, DC pinning. <https://userapps.support.sap.com/sap/support/knowledge/en/3324069>
- SAP Help Portal, *Guided Buying Administration* (landing; JS-rendered, not fully machine-readable):
  <https://help.sap.com/docs/buying-invoicing/guided-buying-administration/b063fb3cc0dc49359bb984afcbd4920a.html>
