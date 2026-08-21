package tjk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkerAdapterEnrichesDetailDocument(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/DataRows/Atlar"):
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<a href="/x?QueryParameter_AtId=99">Thunder</a>
Arabian
<a href="/x?QueryParameter_BabaAdi=Sire">Sire</a>
<a href="/x?QueryParameter_AnneAdi=Dam">Dam</a>
</body></html>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri"):
			_, _ = w.Write([]byte(`
<div class="grid_8">
<span>İsim</span><span>Thunder</span>
<span>Yaş</span><span>4y k a</span>
<span>Doğ. Trh</span><span>01.01.2020</span>
<span>Baba</span><span>Sire</span>
<span>Anne</span><span>Dam / Maid</span>
</div>
<div class="grid_10"><table class="tablesorter"><tbody>
<tr><td>2024 Yılı</td><td>5</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>100</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Pedigri/Pedigri"):
			_, _ = w.Write([]byte(`
<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td rowspan="4" style="background:#dbdbdb;">GF</td><td>GM</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Kardes/Kardes"):
			_, _ = w.Write([]byte(`
<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>Bro</td><td>Sire</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>10</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/AsimRaporu/AsimRaporu"):
			_, _ = w.Write([]byte(`
<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>2023</td><td>Stallion</td><td>Mare</td><td>10</td><td>8</td><td>Gebe</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Yavru/Yavru"):
			_, _ = w.Write([]byte(`
<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>Foal</td><td>2022</td><td>Sire</td><td>Dam</td><td>5</td><td>1</td><td>0</td><td>0</td><td>0</td><td>100</td></tr>
</tbody></table></div>`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer s.Close()

	client, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (WorkerAdapter{Client: client}).FetchPage(context.Background(), "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Horses) != 1 {
		t.Fatalf("horses=%#v", got)
	}
	h := got.Horses[0]
	if h.Number != "99" || h.Name != "Thunder" || h.Race != "Arabian" {
		t.Fatalf("summary=%#v", h)
	}
	if h.BirthYear == nil || *h.BirthYear != 2020 {
		t.Fatalf("birthYear=%v", h.BirthYear)
	}
	if h.Gender == nil || *h.Gender != "a" {
		t.Fatalf("gender=%v", h.Gender)
	}
	if h.Coat == nil || *h.Coat != "k" {
		t.Fatalf("coat=%v", h.Coat)
	}
	var doc DetailDocument
	if err := json.Unmarshal(h.Detail, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Statistics == nil || len(*doc.Statistics) != 1 || doc.Siblings == nil || len(*doc.Siblings) != 1 || doc.Pedigree == nil || len(*doc.Pedigree) == 0 || doc.Mating == nil || len(*doc.Mating) != 1 || doc.Offspring == nil || len(*doc.Offspring) != 1 {
		t.Fatalf("detail=%#v", doc)
	}
	if doc.Profile == nil || doc.Profile.BirthDate != "01.01.2020" || doc.Profile.MaidenSire != "Maid" {
		t.Fatalf("profile=%#v", doc.Profile)
	}
}

func TestWorkerAdapterSkipsBrokenDetailWithoutFailingPage(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/DataRows/Atlar"):
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<a href="/x?QueryParameter_AtId=1">One</a>
Arabian
<a href="/x?QueryParameter_BabaAdi=S1">S1</a>
<a href="/x?QueryParameter_AnneAdi=D1">D1</a>
<a href="/x?QueryParameter_AtId=2">Two</a>
English
<a href="/x?QueryParameter_BabaAdi=S2">S2</a>
<a href="/x?QueryParameter_AnneAdi=D2">D2</a>
</body></html>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/ConnectedPage/AtKosuBilgileri"):
			if r.URL.Query().Get("QueryParameter_AtId") == "1" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`<div class="grid_8"><span>İsim</span><span>Two</span></div>
<div class="grid_10"><table class="tablesorter"><tbody>
<tr><td>2024</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Pedigri/Pedigri"):
			w.WriteHeader(http.StatusBadRequest)
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Kardes/Kardes"):
			if r.URL.Query().Get("Atkodu") == "1" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>Sib</td><td>S2</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/AsimRaporu/AsimRaporu"):
			if r.URL.Query().Get("Atkodu") == "1" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>2023</td><td>S2</td><td>Mare</td><td>1</td><td>1</td><td>Gebe</td></tr>
</tbody></table></div>`))
		case strings.HasPrefix(r.URL.Path, "/TR/YarisSever/Query/Yavru/Yavru"):
			if r.URL.Query().Get("Atkodu") == "1" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`<div class="grid_24"><table class="tablesorter"><tbody>
<tr><td>Foal2</td><td>2022</td><td>S2</td><td>Dam2</td><td>1</td><td>0</td><td>0</td><td>0</td><td>0</td><td>0</td></tr>
</tbody></table></div>`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer s.Close()

	client, err := NewClient(Config{BaseURL: s.URL, HTTPTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (WorkerAdapter{Client: client}).FetchPage(context.Background(), "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Horses) != 2 {
		t.Fatalf("horses=%#v", got)
	}
	if len(got.Horses[0].Detail) != 0 || len(got.Horses[0].EnrichmentIssues) != 0 {
		t.Fatalf("horse 1 should have missing detail without enrichment issues, got %#v", got.Horses[0])
	}
	var doc DetailDocument
	if err := json.Unmarshal(got.Horses[1].Detail, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Statistics == nil || len(*doc.Statistics) != 1 || doc.Siblings == nil || len(*doc.Siblings) != 1 || doc.Mating == nil || len(*doc.Mating) != 1 || doc.Offspring == nil || len(*doc.Offspring) != 1 {
		t.Fatalf("horse 2 detail=%#v", doc)
	}
}
