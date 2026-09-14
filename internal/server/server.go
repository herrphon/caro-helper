// Package server exposes carohelper's local HTTP API and serves the SPA.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"carohelper/internal/config"
	"carohelper/internal/smartsheet"
)

type Server struct {
	cfg    *config.Store
	static fs.FS
	mux    *http.ServeMux
}

func New(cfg *config.Store, static fs.FS) *Server {
	s := &Server{cfg: cfg, static: static, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mux.ServeHTTP(w, r)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("PUT /api/token", s.handleSetToken)
	s.mux.HandleFunc("GET /api/sheets", s.handleListSheets)
	s.mux.HandleFunc("PUT /api/sheets", s.handleSetSheets)
	s.mux.HandleFunc("GET /api/grid/{kind}/{id}", s.handleGetGrid)
	s.mux.HandleFunc("GET /api/browse", s.handleBrowse)
	s.mux.Handle("/", spaHandler(s.static))
}

// --- helpers ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var apiErr *smartsheet.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 401, 403:
			status = http.StatusUnauthorized
		case 404:
			status = http.StatusNotFound
		}
	}
	log.Printf("error: %v", err)
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) client() (*smartsheet.Client, error) {
	tok, err := s.cfg.Token()
	if err != nil {
		return nil, err
	}
	if tok == "" {
		return nil, errors.New("no Smartsheet token configured")
	}
	return smartsheet.New(tok), nil
}

// --- handlers -----------------------------------------------------------

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"hasToken":   s.cfg.HasToken(),
		"configPath": s.cfg.Path(),
		"tokenValid": false,
		"user":       "",
	}
	if s.cfg.HasToken() {
		if c, err := s.client(); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			if me, err := c.Me(ctx); err == nil {
				out["tokenValid"] = true
				out["user"] = me.Email
			} else {
				out["tokenError"] = err.Error()
			}
		} else {
			out["tokenError"] = err.Error()
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSetToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}
	tok := strings.TrimSpace(body.Token)
	// verify before storing
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	me, err := smartsheet.New(tok).Me(ctx)
	if err != nil {
		writeErr(w, fmt.Errorf("token rejected by Smartsheet: %w", err))
		return
	}
	if err := s.cfg.SetToken(tok); err != nil {
		writeErr(w, err)
		return
	}
	log.Printf("token stored for %s", me.Email)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": me.Email})
}

func (s *Server) handleListSheets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Sheets())
}

func (s *Server) handleSetSheets(w http.ResponseWriter, r *http.Request) {
	var sheets []config.SheetAlias
	if err := json.NewDecoder(r.Body).Decode(&sheets); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	for i, sh := range sheets {
		if strings.TrimSpace(sh.Alias) == "" || sh.SheetID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each sheet needs alias and sheetId"})
			return
		}
		if sh.Kind == "" {
			sheets[i].Kind = "sheet"
		} else if sh.Kind != "sheet" && sh.Kind != "report" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be sheet or report"})
			return
		}
	}
	if err := s.cfg.SetSheets(sheets); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sheets)
}

// SheetView is the flattened shape the UI consumes: columns + rows as
// display strings keyed by column id.
type SheetView struct {
	ID         int64               `json:"id"`
	Name       string              `json:"name"`
	Permalink  string              `json:"permalink"`
	ModifiedAt string              `json:"modifiedAt"`
	TotalRows  int                 `json:"totalRows"`
	Columns    []smartsheet.Column `json:"columns"`
	Rows       []RowView           `json:"rows"`
}

type RowView struct {
	ID        int64             `json:"id"`
	RowNumber int               `json:"rowNumber"`
	Cells     map[string]string `json:"cells"` // columnId -> display value
}

func (s *Server) handleGetGrid(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	kind := r.PathValue("kind")
	if kind != "sheet" && kind != "report" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be sheet or report"})
		return
	}
	c, err := s.client()
	if err != nil {
		writeErr(w, err)
		return
	}
	var sheet *smartsheet.Sheet
	if kind == "report" {
		sheet, err = c.GetReport(r.Context(), id, true)
	} else {
		sheet, err = c.GetSheet(r.Context(), id, true)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toView(sheet))
}

func toView(sh *smartsheet.Sheet) SheetView {
	v := SheetView{
		ID: sh.ID, Name: sh.Name, Permalink: sh.Permalink,
		ModifiedAt: sh.ModifiedAt, TotalRows: sh.TotalRows,
		Columns: make([]smartsheet.Column, 0, len(sh.Columns)),
		Rows:    make([]RowView, 0, len(sh.Rows)),
	}
	for _, col := range sh.Columns {
		if !col.Hidden {
			v.Columns = append(v.Columns, col)
		}
	}
	for _, row := range sh.Rows {
		rv := RowView{ID: row.ID, RowNumber: row.RowNumber, Cells: make(map[string]string, len(row.Cells))}
		for _, cell := range row.Cells {
			val := cell.DisplayValue
			if val == "" && cell.Value != nil {
				val = fmt.Sprint(cell.Value)
			}
			if val != "" {
				rv.Cells[strconv.FormatInt(cell.ColumnID, 10)] = val
			}
		}
		v.Rows = append(v.Rows, rv)
	}
	return v
}

// handleBrowse returns the workspace tree so the user can pick sheets to alias.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	c, err := s.client()
	if err != nil {
		writeErr(w, err)
		return
	}
	refs, err := c.ListWorkspaces(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]*smartsheet.Workspace, len(refs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // stay well under Smartsheet's rate limit
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref smartsheet.ItemRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ws, err := c.GetWorkspace(r.Context(), ref.ID)
			if err != nil {
				log.Printf("warn: workspace %q: %v", ref.Name, err)
				return
			}
			out[i] = ws
		}(i, ref)
	}
	wg.Wait()
	result := make([]*smartsheet.Workspace, 0, len(out))
	for _, ws := range out {
		if ws != nil {
			result = append(result, ws)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// spaHandler serves static files and falls back to index.html.
func spaHandler(static fs.FS) http.Handler {
	fileServer := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(static, p); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
