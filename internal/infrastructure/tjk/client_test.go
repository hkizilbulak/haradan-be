package tjk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientFetchPageUsesLegacyBulkContract(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TR/YarisSever/Query/DataRows/Atlar" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("PageNumber") != "0" || q.Get("Sort") != "AtIsmi" || q.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Fatalf("query = %v", q)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Fatal("expected User-Agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_AtId=123">Thunder</a>
Arabian
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_BabaAdi=Sire">Sire</a>
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_AnneAdi=Dam">Dam</a>
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_AtId=bad"> </a>
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_AtId=456">Storm</a>
English
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_BabaAdi=S2">S2</a>
<a href="/TR/YarisSever/Query/Page/Atlar?QueryParameter_AnneAdi=D2">D2</a>
</body></html>`))
	}))
	defer s.Close()

	c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.FetchPage(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("horses = %#v", got)
	}
	if got[0] != (Horse{Number: "123", Name: "Thunder", Race: "Arabian", Sire: "Sire", Dam: "Dam"}) {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1] != (Horse{Number: "456", Name: "Storm", Race: "English", Sire: "S2", Dam: "D2"}) {
		t.Fatalf("second = %#v", got[1])
	}
}

func TestClientFetchPageTableFallback(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("PageNumber") != "2" {
			t.Fatalf("PageNumber = %q", r.URL.Query().Get("PageNumber"))
		}
		_, _ = w.Write([]byte(`<table><tr><th>header</th></tr><tr>
<td>1</td><td><a href="/Query/DataRows/Atlar?AtId=123">  Thunder  </a></td>
<td>Arabian</td><td>Sire</td><td>Dam</td></tr>
<tr><td>2</td><td>broken</td></tr></table>`))
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

func TestClientFetchPageEmptyMeansEnd(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div>Toplam 0</div></body></html>`))
	}))
	defer s.Close()
	c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.FetchPage(context.Background(), "99")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("horses = %#v", got)
	}
}

func TestClientFetchDetailPedigreeSiblings(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri"):
			if r.URL.Query().Get("1") != "1" || r.URL.Query().Get("QueryParameter_AtId") != "99" {
				t.Fatalf("detail query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`
<div class="grid_8">
<span>İsim</span><span>Thunder</span>
<span>Yaş</span><span>4y k a</span>
<span>Doğ. Trh</span><span>01.01.2020</span>
<span>Handikap P.</span><span>40</span>
<span>Baba</span><span>Sire</span>
<span>Anne</span><span>Dam / Maid</span>
<span>Gerçek Sahip</span><span>Owner</span>
<span>Yetiştirici</span><span>Grower</span>
</div>
<div class="grid_10">
<table class="tablesorter"><tbody>
<tr><td>2024 Yılı Kum</td><td>5</td><td>1</td><td>2</td><td>0</td><td>0</td><td>1</td><td>10.000</td></tr>
</tbody></table>
</div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Pedigri/Pedigri"):
			if r.URL.Query().Get("Atkodu") != "99" {
				t.Fatalf("pedigree query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`
<div class="grid_24">
<table class="tablesorter"><tbody>
<tr>
<td rowspan="4" style="background:#dbdbdb;">GF</td>
<td rowspan="2" style="background:#dbdbdb;">F</td>
<td style="background:#dbdbdb;">FF</td>
</tr>
<tr><td>FM</td></tr>
<tr>
<td rowspan="2">M</td>
<td style="background:#dbdbdb;">MF</td>
</tr>
<tr><td>MM</td></tr>
</tbody></table>
</div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Kardes/Kardes"):
			if r.URL.Query().Get("Atkodu") != "99" {
				t.Fatalf("sibling query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`
<div class="grid_24">
<table class="tablesorter"><tbody>
<tr><td>Bro</td><td>Sire</td><td>3</td><td>1</td><td>0</td><td>0</td><td>1</td><td>500</td></tr>
</tbody></table>
</div>`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer s.Close()

	c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.FetchDetail(context.Background(), "99")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Thunder" || d.Sire != "Sire" || d.Dam != "Dam" || d.Owner != "Owner" {
		t.Fatalf("detail = %#v", d)
	}
	if len(d.Statistics) != 1 || d.Statistics[0].RaceCount != "5" {
		t.Fatalf("stats = %#v", d.Statistics)
	}

	ped, err := c.FetchPedigree(context.Background(), "99")
	if err != nil {
		t.Fatal(err)
	}
	if len(ped) == 0 || ped[0].Father != "GF" {
		t.Fatalf("pedigree = %#v", ped)
	}

	sibs, err := c.FetchSiblings(context.Background(), "99")
	if err != nil {
		t.Fatal(err)
	}
	if len(sibs) != 1 || sibs[0].Name != "Bro" || sibs[0].FatherName != "Sire" {
		t.Fatalf("siblings = %#v", sibs)
	}

	doc := BuildDetailDocument(ped, sibs, d.Statistics)
	if len(doc.Pedigree) == 0 || len(doc.Siblings) != 1 || len(doc.Statistics) != 1 {
		t.Fatalf("detail doc = %#v", doc)
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
	_, err = c.FetchPage(context.Background(), "")
	if err == nil {
		t.Fatal("expected redirect error")
	}
	if !IsPermanent(err) {
		t.Fatalf("expected permanent, got %v", err)
	}
}

func TestClientClassifiesStatus(t *testing.T) {
	cases := []struct {
		status    int
		transient bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
	}
	for _, tc := range cases {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		c, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
		if err != nil {
			s.Close()
			t.Fatal(err)
		}
		_, err = c.FetchPage(context.Background(), "0")
		s.Close()
		if err == nil {
			t.Fatalf("status %d: expected error", tc.status)
		}
		if IsTransient(err) != tc.transient {
			t.Fatalf("status %d: transient=%v err=%v", tc.status, IsTransient(err), err)
		}
		if strings.Contains(err.Error(), "http") && strings.Contains(err.Error(), "://") {
			t.Fatalf("error must not contain URL: %v", err)
		}
	}
}

func TestClientRequiresConfiguration(t *testing.T) {
	if _, err := NewClient(Config{}); err != ErrUnconfigured {
		t.Fatalf("err = %v", err)
	}
}

func TestClientStripsBasePath(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TR/YarisSever/Query/DataRows/Atlar" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><body></body></html>`))
	}))
	defer s.Close()
	c, err := NewClient(Config{BaseURL: s.URL + "/ignored/prefix", HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.FetchPage(context.Background(), "0"); err != nil {
		t.Fatal(err)
	}
}
