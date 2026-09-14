// Command explore dumps the Smartsheet structure (workspaces, folders,
// sheets, columns) visible to the token in .env as Markdown on stdout.
// No row data is ever printed.
//
//	go run ./cmd/explore                       # everything
//	go run ./cmd/explore -workspace "LEAN"     # workspaces whose name contains LEAN
//	go run ./cmd/explore -sheet 123456789      # one sheet's columns
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"carohelper/internal/smartsheet"
)

func main() {
	wsFilter := flag.String("workspace", "", "only workspaces whose name contains this (case-insensitive)")
	sheetID := flag.Int64("sheet", 0, "dump only this sheet's columns")
	reportID := flag.Int64("report", 0, "dump only this report's columns")
	reports := flag.Bool("reports", false, "also fetch report columns (slower)")
	noCols := flag.Bool("no-columns", false, "skip per-sheet column fetch (faster)")
	flag.Parse()

	loadDotEnv(".env")
	token := os.Getenv("SMARTSHEET_TOKEN")
	if token == "" {
		fatal("SMARTSHEET_TOKEN not set (put it in .env)")
	}
	c := smartsheet.New(token)
	ctx := context.Background()

	fmt.Printf("# Smartsheet layout\n\nGenerated %s. Structure only, no row data.\n\n", time.Now().Format("2006-01-02"))

	if *sheetID != 0 {
		printGrid(ctx, c, *sheetID, false, 0)
		return
	}
	if *reportID != 0 {
		printGrid(ctx, c, *reportID, true, 0)
		return
	}

	wss, err := c.ListWorkspaces(ctx)
	if err != nil {
		fatal(err.Error())
	}
	for _, ref := range wss {
		if *wsFilter != "" && !strings.Contains(strings.ToLower(ref.Name), strings.ToLower(*wsFilter)) {
			continue
		}
		ws, err := c.GetWorkspace(ctx, ref.ID)
		if err != nil {
			warn("workspace %q (%d): %v", ref.Name, ref.ID, err)
			continue
		}
		fmt.Printf("## Workspace: %s\n\n- id: `%d`\n\n", ws.Name, ws.ID)
		printFolder(ctx, c, &ws.Folder, 0, !*noCols, *reports)
	}
}

func printFolder(ctx context.Context, c *smartsheet.Client, f *smartsheet.Folder, depth int, withCols, withReports bool) {
	ind := strings.Repeat("  ", depth)
	for _, s := range f.Sheets {
		fmt.Printf("%s- Sheet **%s** (id `%d`)\n", ind, s.Name, s.ID)
		if withCols {
			printGrid(ctx, c, s.ID, false, depth+1)
		}
	}
	for _, r := range f.Reports {
		fmt.Printf("%s- Report *%s* (id `%d`)\n", ind, r.Name, r.ID)
		if withCols && withReports {
			printGrid(ctx, c, r.ID, true, depth+1)
		}
	}
	for _, d := range f.Sights {
		fmt.Printf("%s- Dashboard *%s* (id `%d`)\n", ind, d.Name, d.ID)
	}
	for i := range f.Folders {
		sub := &f.Folders[i]
		fmt.Printf("%s- Folder **%s/** (id `%d`)\n", ind, sub.Name, sub.ID)
		printFolder(ctx, c, sub, depth+1, withCols, withReports)
	}
	if depth == 0 {
		fmt.Println()
	}
}

func printGrid(ctx context.Context, c *smartsheet.Client, id int64, isReport bool, depth int) {
	ind := strings.Repeat("  ", depth)
	var s *smartsheet.Sheet
	var err error
	if isReport {
		s, err = c.GetReport(ctx, id, false)
	} else {
		s, err = c.GetSheet(ctx, id, false)
	}
	if err != nil {
		warn("grid %d: %v", id, err)
		fmt.Printf("%s- (error: %v)\n", ind, err)
		return
	}
	fmt.Printf("%s- rows: %d, modified: %s\n", ind, s.TotalRows, s.ModifiedAt)
	if len(s.SourceSheets) > 0 {
		names := make([]string, 0, len(s.SourceSheets))
		for _, ss := range s.SourceSheets {
			names = append(names, fmt.Sprintf("%s (`%d`)", ss.Name, ss.ID))
		}
		fmt.Printf("%s- source sheets: %s\n", ind, strings.Join(names, ", "))
	}
	fmt.Printf("%s- columns:\n", ind)
	for _, col := range s.Columns {
		flags := ""
		if col.Primary {
			flags += " primary"
		}
		if col.Hidden {
			flags += " hidden"
		}
		opts := ""
		if len(col.Options) > 0 {
			opts = " options: " + strings.Join(col.Options, ", ")
		}
		fmt.Printf("%s  - `%s` (%s%s)%s\n", ind, col.Title, col.Type, flags, opts)
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func warn(format string, a ...any) { fmt.Fprintf(os.Stderr, "warn: "+format+"\n", a...) }
func fatal(msg string)             { fmt.Fprintln(os.Stderr, "error:", msg); os.Exit(1) }
