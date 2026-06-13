package dlmf_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/dlmf-cli/dlmf"
)

// minimalHomePage is a trimmed DLMF home page fragment with chapter links.
const minimalHomePage = `<!DOCTYPE html><html><head>
<link rel="chapter" href="./1" title="Chapter 1 Algebraic and Analytic Methods">
<link rel="chapter" href="./2" title="Chapter 2 Asymptotic Approximations">
<link rel="chapter" href="./10" title="Chapter 10 Bessel Functions">
</head><body></body></html>`

// minimalSearchPage is a trimmed DLMF search results fragment.
const minimalSearchPage = `<!DOCTYPE html><html><body>
<h5 class="ltx_hittag">1: <a href="../10.1" title="" class="ltx_ref"><span class="ltx_tag ltx_tag_ref">10.1 </span>Special Notation</a>
</h5>
<h5 class="ltx_hittag">2: <a href="../35.5" title="" class="ltx_ref"><span class="ltx_tag ltx_tag_ref">35.5 </span>Bessel Functions of Matrix Argument</a>
</h5>
</body></html>`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*dlmf.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := dlmf.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	return dlmf.NewClient(cfg), srv
}

func TestGet_SetsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := dlmf.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := dlmf.NewClient(cfg)

	_, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("request carried no User-Agent")
	}
}

func TestGet_RetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	cfg := dlmf.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := dlmf.NewClient(cfg)

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestChapters_Parse(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(minimalHomePage))
	})
	defer srv.Close()

	chapters, err := c.Chapters(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 3 {
		t.Fatalf("got %d chapters, want 3", len(chapters))
	}

	ch := chapters[0]
	if ch.Number != 1 {
		t.Errorf("chapter[0].Number = %d, want 1", ch.Number)
	}
	if ch.Title != "Algebraic and Analytic Methods" {
		t.Errorf("chapter[0].Title = %q", ch.Title)
	}
	if !strings.HasSuffix(ch.URL, "/1") {
		t.Errorf("chapter[0].URL = %q, want suffix /1", ch.URL)
	}

	ch10 := chapters[2]
	if ch10.Number != 10 {
		t.Errorf("chapter[2].Number = %d, want 10", ch10.Number)
	}
	if ch10.Title != "Bessel Functions" {
		t.Errorf("chapter[2].Title = %q, want Bessel Functions", ch10.Title)
	}
}

func TestChapters_Limit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(minimalHomePage))
	})
	defer srv.Close()

	chapters, err := c.Chapters(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2 (limit applied)", len(chapters))
	}
}

func TestSearch_Parse(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/search/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(minimalSearchPage))
	})
	defer srv.Close()

	results, err := c.Search(context.Background(), "bessel function", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r0 := results[0]
	if r0.Rank != 1 {
		t.Errorf("results[0].Rank = %d, want 1", r0.Rank)
	}
	if r0.Section != "10.1" {
		t.Errorf("results[0].Section = %q, want 10.1", r0.Section)
	}
	if !strings.Contains(r0.Title, "Special Notation") {
		t.Errorf("results[0].Title = %q, want to contain Special Notation", r0.Title)
	}
	if !strings.HasSuffix(r0.URL, "/10.1") {
		t.Errorf("results[0].URL = %q, want suffix /10.1", r0.URL)
	}

	r1 := results[1]
	if r1.Section != "35.5" {
		t.Errorf("results[1].Section = %q, want 35.5", r1.Section)
	}
}

func TestSearch_Limit(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(minimalSearchPage))
	})
	defer srv.Close()

	results, err := c.Search(context.Background(), "bessel", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (limit applied)", len(results))
	}
}
