package storage

import (
	"context"
	"testing"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

func TestRepeaterTabsCRUDAndHistory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	tab := &models.RepeaterTab{
		ID: NewID(), Name: "login", Scheme: "https", Host: "example.com:443",
		Raw: []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"), HTTPVersion: "HTTP/1.1",
	}
	if err := db.CreateRepeaterTab(ctx, tab); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetRepeaterTab(ctx, tab.ID)
	if err != nil || got.Name != "login" || string(got.Raw) != string(tab.Raw) {
		t.Fatalf("get: %v %+v", err, got)
	}

	got.Name = "renamed"
	got.FollowRedirects = true
	if err := db.UpdateRepeaterTab(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reget, _ := db.GetRepeaterTab(ctx, tab.ID)
	if reget.Name != "renamed" || !reget.FollowRedirects {
		t.Errorf("update not persisted: %+v", reget)
	}

	if err := db.AddRepeaterHistory(ctx, &models.RepeaterHistoryEntry{
		ID: NewID(), TabID: tab.ID, RequestRaw: tab.Raw,
		ResponseRaw: []byte("HTTP/1.1 200 OK\r\n\r\nhi"), StatusCode: 200, DurationMs: 5,
	}); err != nil {
		t.Fatalf("add history: %v", err)
	}
	hist, err := db.ListRepeaterHistory(ctx, tab.ID)
	if err != nil || len(hist) != 1 || hist[0].StatusCode != 200 {
		t.Fatalf("history: %v %+v", err, hist)
	}

	tabs, _ := db.ListRepeaterTabs(ctx)
	if len(tabs) != 1 {
		t.Errorf("expected 1 tab, got %d", len(tabs))
	}

	if err := db.DeleteRepeaterTab(ctx, tab.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetRepeaterTab(ctx, tab.ID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
	// History was removed by cascade.
	if hist, _ := db.ListRepeaterHistory(ctx, tab.ID); len(hist) != 0 {
		t.Errorf("history not cascaded: %d", len(hist))
	}
}

func TestSitemap(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, ex := range []*models.Exchange{
		sampleExchange(NewID(), "GET", "a.example.com", "/", "x", 200),
		sampleExchange(NewID(), "POST", "a.example.com", "/login", "x", 200),
		sampleExchange(NewID(), "GET", "a.example.com", "/login", "x", 200),
		sampleExchange(NewID(), "GET", "b.example.com", "/", "x", 200),
	} {
		if err := db.StoreExchange(ctx, ex); err != nil {
			t.Fatalf("store: %v", err)
		}
	}

	if err := db.UpsertSitemapNote(ctx, "a.example.com", "/login", "auth endpoint", "auth,important"); err != nil {
		t.Fatalf("note: %v", err)
	}
	// Upsert again to confirm ON CONFLICT update works.
	if err := db.UpsertSitemapNote(ctx, "a.example.com", "/login", "login endpoint", "auth"); err != nil {
		t.Fatalf("note update: %v", err)
	}

	tree, err := db.Sitemap(ctx)
	if err != nil {
		t.Fatalf("sitemap: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(tree))
	}

	var a *SitemapHost
	for i := range tree {
		if tree[i].Host == "a.example.com" {
			a = &tree[i]
		}
	}
	if a == nil || len(a.Paths) != 2 {
		t.Fatalf("host a: %+v", a)
	}
	for _, p := range a.Paths {
		if p.Path == "/login" {
			if len(p.Methods) != 2 { // GET + POST
				t.Errorf("/login methods = %v, want GET+POST", p.Methods)
			}
			if p.Note != "login endpoint" || p.Tags != "auth" {
				t.Errorf("/login note/tags = %q / %q", p.Note, p.Tags)
			}
		}
	}
}
