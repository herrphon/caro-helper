//go:build !windows

package main

import "fmt"

type cookie struct {
	Name  string
	Value string
}

// readEdgeCookies is Windows-only: it depends on DPAPI and Edge's on-disk
// cookie store. This stub lets the spike compile and vet on the dev Mac.
func readEdgeCookies(profile, realmHost string) ([]cookie, error) {
	return nil, fmt.Errorf("Edge cookie import is Windows-only; run ariba-probe on Caro's laptop")
}
