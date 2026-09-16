// Command ariba-probe is a throwaway spike (like cmd/explore) that proves we
// can replay Caro's authenticated SAP Ariba browser session from Go on the same
// machine, without any official API, OAuth app, or IT involvement.
//
// It reads the cookies Edge already holds for the Ariba realm (they are refreshed
// whenever she uses Ariba normally), replays one request she captured from
// DevTools, and prints enough to tell whether the session replay worked:
//
//   - master-key decrypt result (or ABE detected -> loud failure)
//   - how many cookies matched the realm host
//   - the probe request's HTTP status and FINAL url (an IdP redirect means the
//     session was not accepted)
//   - the first ~200 bytes of the body
//
// No secrets are printed: cookie values, headers and full bodies are never logged.
//
// Usage (Windows, after capturing values into ariba-probe.json):
//
//	go run ./cmd/ariba-probe            # reads ./ariba-probe.json
//	go run ./cmd/ariba-probe -c foo.json
//
// See docs/adr/0001-ariba-session-replay.md for the capture recipe and rationale.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

// Probe is the git-ignored input you fill in after capturing a request from
// Edge DevTools (Network tab -> pick an authenticated XHR -> read off these bits).
type Probe struct {
	// RealmHost is the Ariba origin host, e.g. "prod3.procurement.ariba.com".
	// Cookies are matched against this host; the probe URL should be on it too.
	RealmHost string `json:"realmHost"`

	// ProbeURL is the full URL of an authenticated request the SPA makes, ideally
	// a lightweight bootstrap/whoami-ish GET. Guessy: that's the point of probing.
	ProbeURL string `json:"probeURL"`

	// Method defaults to GET.
	Method string `json:"method,omitempty"`

	// Headers are non-cookie request headers to mirror (Accept, X-CSRF-Token,
	// any Ariba-specific ones). Do NOT put Cookie here; cookies come from Edge.
	Headers map[string]string `json:"headers,omitempty"`

	// CookieNameFilter, if non-empty, restricts which cookie names are sent
	// (substring match). Empty = send every cookie Edge holds for the realm host.
	CookieNameFilter []string `json:"cookieNameFilter,omitempty"`

	// UserAgent mirrors Caro's browser UA so the request looks like her traffic.
	// If empty a current Edge-on-Windows UA is used.
	UserAgent string `json:"userAgent,omitempty"`

	// EdgeProfile is the Edge profile directory name (default "Default").
	EdgeProfile string `json:"edgeProfile,omitempty"`
}

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.0.0"

func main() {
	cfgPath := flag.String("c", "ariba-probe.json", "path to probe config json")
	flag.Parse()

	p, err := loadProbe(*cfgPath)
	if err != nil {
		fatal("load %s: %v", *cfgPath, err)
	}
	if p.Method == "" {
		p.Method = http.MethodGet
	}
	if p.UserAgent == "" {
		p.UserAgent = defaultUA
	}
	if p.EdgeProfile == "" {
		p.EdgeProfile = "Default"
	}
	if p.RealmHost == "" || p.ProbeURL == "" {
		fatal("realmHost and probeURL are required in %s", *cfgPath)
	}

	fmt.Printf("realm host : %s\n", p.RealmHost)
	fmt.Printf("probe      : %s %s\n", p.Method, p.ProbeURL)

	// 1. Pull cookies out of Edge for the realm host.
	cookies, err := readEdgeCookies(p.EdgeProfile, p.RealmHost)
	if err != nil {
		fatal("read Edge cookies: %v", err)
	}
	cookies = filterCookies(cookies, p.CookieNameFilter)
	fmt.Printf("cookies    : %d matched realm host\n", len(cookies))
	if len(cookies) == 0 {
		fatal("no cookies for %s - is Caro logged into Ariba in Edge (profile %q)?", p.RealmHost, p.EdgeProfile)
	}
	printCookieNames(cookies)

	// 2. Replay the captured request with those cookies.
	status, finalURL, body, err := replay(p, cookies)
	if err != nil {
		fatal("replay: %v", err)
	}

	fmt.Printf("http status: %d\n", status)
	fmt.Printf("final url  : %s\n", finalURL)
	if redirectedToIdP(p.ProbeURL, finalURL) {
		fmt.Println("!! final url differs from probe origin - session likely NOT accepted (IdP/login redirect).")
	}
	fmt.Printf("body[:200] : %s\n", preview(body, 200))

	fmt.Println("\ndone. 200 + JSON on the realm origin = session replay works.")
}

func loadProbe(path string) (Probe, error) {
	var p Probe
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	return p, nil
}

func filterCookies(in []cookie, names []string) []cookie {
	if len(names) == 0 {
		return in
	}
	var out []cookie
	for _, c := range in {
		for _, n := range names {
			if strings.Contains(c.Name, n) {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// printCookieNames prints only names + value lengths, never the secret values.
func printCookieNames(cookies []cookie) {
	for _, c := range cookies {
		fmt.Printf("  - %s (len %d)\n", c.Name, len(c.Value))
	}
}

func replay(p Probe, cookies []cookie) (status int, finalURL string, body []byte, err error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return 0, "", nil, err
	}
	u, err := url.Parse(p.ProbeURL)
	if err != nil {
		return 0, "", nil, fmt.Errorf("probeURL: %w", err)
	}
	hc := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		hc = append(hc, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	jar.SetCookies(u, hc)

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest(p.Method, p.ProbeURL, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("User-Agent", p.UserAgent)
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, resp.Request.URL.String(), body, nil
}

func redirectedToIdP(probeURL, finalURL string) bool {
	a, err1 := url.Parse(probeURL)
	b, err2 := url.Parse(finalURL)
	if err1 != nil || err2 != nil {
		return false
	}
	return a.Host != b.Host
}

func preview(b []byte, n int) string {
	s := strings.ReplaceAll(string(b), "\n", " ")
	if len(s) > n {
		s = s[:n] + "..."
	}
	return s
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ariba-probe: "+format+"\n", args...)
	os.Exit(1)
}
