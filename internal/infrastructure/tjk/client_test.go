package tjk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFetchPageParsesAtIDRows(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Fatalf("page = %q", got)
		}
		_, _ = w.Write([]byte(`<table><tr><th>header</th></tr><tr>
<td>1</td><td><a href="/Query/DataRows/Atlar?AtId=123">  Thunder  </a></td>
<td>Arabian</td><td>Sire</td><td>Dam</td></tr></table>`))
	}))
	defer s.Close()
	c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.FetchPage(context.Background(), "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Horse{Number: "123", Name: "Thunder", Race: "Arabian", Sire: "Sire", Dam: "Dam"}) {
		t.Fatalf("horses = %#v", got)
	}
}

func TestClientBlocksRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("redirect target must not be requested")
	}))
	defer target.Close()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer s.Close()
	c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FetchPage(context.Background(), ""); err == nil {
		t.Fatal("expected redirect error")
	}
}

func TestClientRequiresConfiguration(t *testing.T) {
	if _, err := NewClient(Config{}); err != ErrUnconfigured {
		t.Fatalf("err = %v", err)
	}
}
