// Package smartsheet is a minimal client for the Smartsheet REST API v2.
// It covers only what carohelper needs: browsing the workspace/folder tree
// and reading whole sheets.
package smartsheet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.smartsheet.com/2.0"

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(token string) *Client {
	base := DefaultBaseURL
	if v := os.Getenv("SMARTSHEET_BASE_URL"); v != "" { // for tests against a fake server
		base = v
	}
	return &Client{
		BaseURL: base,
		Token:   token,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status    int
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("smartsheet: HTTP %d (code %d): %s", e.Status, e.ErrorCode, e.Message)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("smartsheet: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		apiErr := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(body, apiErr)
		if apiErr.Message == "" {
			apiErr.Message = string(body)
		}
		return apiErr
	}
	return json.Unmarshal(body, out)
}

// paged fetches every page of a list endpoint and appends into collect.
func (c *Client) paged(ctx context.Context, path string, query url.Values, collect func(json.RawMessage) error) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("pageSize", "100")
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		var resp struct {
			TotalPages int             `json:"totalPages"`
			Data       json.RawMessage `json:"data"`
		}
		if err := c.get(ctx, path, query, &resp); err != nil {
			return err
		}
		if err := collect(resp.Data); err != nil {
			return err
		}
		if page >= resp.TotalPages {
			return nil
		}
	}
}

// --- Navigation ---------------------------------------------------------

type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// Me returns the current user; the cheapest way to validate a token.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	err := c.get(ctx, "/users/me", nil, &u)
	return &u, err
}

type ItemRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Permalink string `json:"permalink,omitempty"`
}

type Folder struct {
	ItemRef
	Sheets  []ItemRef `json:"sheets"`
	Folders []Folder  `json:"folders"`
	Reports []ItemRef `json:"reports"`
	Sights  []ItemRef `json:"sights"`
}

type Workspace struct {
	Folder
	AccessLevel string `json:"accessLevel"`
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]ItemRef, error) {
	var all []ItemRef
	err := c.paged(ctx, "/workspaces", nil, func(raw json.RawMessage) error {
		var page []ItemRef
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		all = append(all, page...)
		return nil
	})
	return all, err
}

// GetWorkspace returns the workspace with its full nested folder tree.
func (c *Client) GetWorkspace(ctx context.Context, id int64) (*Workspace, error) {
	var ws Workspace
	q := url.Values{"loadAll": {"true"}}
	err := c.get(ctx, "/workspaces/"+strconv.FormatInt(id, 10), q, &ws)
	return &ws, err
}

// --- Sheets -------------------------------------------------------------

type Column struct {
	ID      int64    `json:"id"`
	Index   int      `json:"index"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Primary bool     `json:"primary"`
	Hidden  bool     `json:"hidden"`
	Options []string `json:"options,omitempty"`
}

type Cell struct {
	ColumnID     int64  `json:"columnId"`
	Value        any    `json:"value,omitempty"`
	DisplayValue string `json:"displayValue,omitempty"`
}

type Row struct {
	ID        int64  `json:"id"`
	RowNumber int    `json:"rowNumber"`
	Cells     []Cell `json:"cells"`
}

type Sheet struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Permalink  string   `json:"permalink"`
	TotalRows  int      `json:"totalRowCount"`
	ModifiedAt string   `json:"modifiedAt"`
	Columns    []Column `json:"columns"`
	Rows       []Row    `json:"rows"`
	Version    int      `json:"version"`
}

// GetSheet fetches the whole sheet. Set withRows=false for structure only.
func (c *Client) GetSheet(ctx context.Context, id int64, withRows bool) (*Sheet, error) {
	q := url.Values{"pageSize": {"10000"}}
	if !withRows {
		q.Set("pageSize", "1")
	}
	var s Sheet
	err := c.get(ctx, "/sheets/"+strconv.FormatInt(id, 10), q, &s)
	if err != nil {
		return nil, err
	}
	if !withRows {
		s.Rows = nil
	}
	return &s, nil
}
