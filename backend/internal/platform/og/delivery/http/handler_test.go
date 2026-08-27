package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStore struct {
	m   map[string]*metadata
	err error
}

func (f *fakeStore) getForSale(_ context.Context, _ string) (*metadata, error) {
	return f.m["for_sale"], f.err
}
func (f *fakeStore) getAuction(_ context.Context, _ string) (*metadata, error) {
	return f.m["auction"], f.err
}
func (f *fakeStore) getProfile(_ context.Context, _ string) (*metadata, error) {
	return f.m["profile"], f.err
}
func (f *fakeStore) getContent(_ context.Context, _ string) (*metadata, error) {
	return f.m["content"], f.err
}

func TestForSaleOGSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{store: &fakeStore{m: map[string]*metadata{
		"for_sale": {Title: "Kohaku <A>", Description: "Nice fish", ImageURL: "https://img.example/x.jpg"},
	}}}
	r.GET("/og/forSale/:id", h.GetForSale)

	req := httptest.NewRequest(http.MethodGet, "/og/forSale/123", nil)
	req.Host = "labuda-79de2.web.app"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want=200", w.Code)
	}
	if !strings.Contains(body, `property="og:title"`) || !strings.Contains(body, `name="twitter:card"`) {
		t.Fatalf("missing expected OG/Twitter tags: %s", body)
	}
	if !strings.Contains(body, `Kohaku &lt;A&gt;`) {
		t.Fatalf("expected escaped title, got: %s", body)
	}
}

func TestAliasRoutesSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{store: &fakeStore{m: map[string]*metadata{
		"for_sale": {Title: "ForSale 1", Description: "forSale desc"},
		"auction": {Title: "Auction 1", Description: "auction desc"},
		"profile": {Title: "@user", Description: "profile desc"},
		"content": {Title: "@author", Description: "content desc"},
	}}}
	r.GET("/forSale/:id", h.GetForSale)
	r.GET("/auction/:id", h.GetAuction)
	r.GET("/profile/:id", h.GetProfile)
	r.GET("/content/:id", h.GetContent)
	r.GET("/og/forSale/:id", h.GetForSale)

	for _, path := range []string{
		"/forSale/1",
		"/auction/1",
		"/profile/1",
		"/content/1",
		"/og/forSale/1",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d want=200", path, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, `property="og:title"`) || !strings.Contains(body, `name="twitter:card"`) {
			t.Fatalf("%s missing expected OG html tags", path)
		}
	}
}

func TestAuctionProfileContentSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{store: &fakeStore{m: map[string]*metadata{
		"auction": {Title: "Auction 1", Description: "desc"},
		"profile": {Title: "@user", Description: "bio"},
		"content": {Title: "@author", Description: "caption"},
	}}}
	r.GET("/og/auction/:id", h.GetAuction)
	r.GET("/og/profile/:id", h.GetProfile)
	r.GET("/og/content/:id", h.GetContent)

	for _, path := range []string{"/og/auction/1", "/og/profile/1", "/og/content/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d want=200", path, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, `property="og:description"`) {
			t.Fatalf("%s missing og:description", path)
		}
	}
}

func TestMissingReturnsGenericNot500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{store: &fakeStore{m: map[string]*metadata{}}}
	r.GET("/og/forSale/:id", h.GetForSale)

	req := httptest.NewRequest(http.MethodGet, "/og/forSale/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want=200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Labuda") {
		t.Fatalf("expected generic fallback page")
	}
}

func TestExtractFirstURL(t *testing.T) {
	got := extractFirstURL([]byte(`["https://a.example/1.jpg","https://a.example/2.jpg"]`))
	if got != "https://a.example/1.jpg" {
		t.Fatalf("got=%q", got)
	}
}


