package storage

import (
	"context"
	"sync"
	"testing"
)

func TestListRequests_LikeWildcardsAreLiteral(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// Two responses with distinct mime types; neither contains the literal
	// substring "t%t". Before escaping, "%" acted as a LIKE wildcard and matched
	// both. After escaping it should match neither.
	for _, mt := range []string{"text/html", "text/xml"} {
		ex := sampleExchange(NewID(), "GET", "h.example", "/"+mt, "body", 200)
		ex.Response.MimeType = mt
		if err := db.StoreExchange(ctx, ex); err != nil {
			t.Fatalf("StoreExchange: %v", err)
		}
	}

	_, total, err := db.ListRequests(ctx, RequestFilter{MimeType: "t%t"})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if total != 0 {
		t.Errorf("MimeType %q matched %d rows, want 0 (%% must be literal)", "t%t", total)
	}

	// A real substring still matches.
	_, total, err = db.ListRequests(ctx, RequestFilter{MimeType: "text/x"})
	if err != nil {
		t.Fatalf("ListRequests: %v", err)
	}
	if total != 1 {
		t.Errorf("MimeType %q matched %d rows, want 1", "text/x", total)
	}
}

func TestInMemoryDB_ConcurrentAccess(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	defer func() { _ = db.Close() }()

	// With a single pooled connection the migrated schema is visible to every
	// query; previously concurrent access hit "no such table".
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := db.ListRequests(context.Background(), RequestFilter{}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent ListRequests failed: %v", err)
	}
}
