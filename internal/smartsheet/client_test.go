package smartsheet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserAgentIsSent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
		fmt.Fprint(w, `{"id":1,"email":"x"}`)
	}))
	defer srv.Close()

	c := New("t")
	c.BaseURL = srv.URL
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != DefaultUserAgent {
		t.Fatalf("got UA %q", got)
	}

	c.UserAgent = "Custom/1.0"
	c.Me(context.Background())
	if got != "Custom/1.0" {
		t.Fatalf("override not applied, got %q", got)
	}
}
