package storage

import (
	"context"
	"testing"

	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

func TestSequencerTaskLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	task := &models.SequencerTask{
		ID: NewID(), Name: "sid", Scheme: "http", Host: "h:80",
		Template: []byte("GET / HTTP/1.1\r\n\r\n"), Source: "cookie", Selector: "sid", Target: 10,
	}
	// Regression: a fresh task has a nil Report; create must not violate the
	// NOT NULL report column.
	if err := db.CreateSequencerTask(ctx, task); err != nil {
		t.Fatalf("CreateSequencerTask: %v", err)
	}
	if err := db.PrepareSequencerRun(ctx, task.ID); err != nil {
		t.Fatalf("PrepareSequencerRun: %v", err)
	}
	if err := db.UpdateSequencerProgress(ctx, task.ID, 2, []byte(`["a","b"]`)); err != nil {
		t.Fatalf("UpdateSequencerProgress: %v", err)
	}
	if err := db.SetSequencerResult(ctx, task.ID, "completed", []byte(`{"sampleCount":2}`)); err != nil {
		t.Fatalf("SetSequencerResult: %v", err)
	}
	got, err := db.GetSequencerTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetSequencerTask: %v", err)
	}
	if got.Status != "completed" || got.Collected != 2 || string(got.Report) != `{"sampleCount":2}` {
		t.Errorf("task = %+v, want completed/2 with report", got)
	}
}

func TestCrawlTaskAndURLs(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	task := &models.CrawlTask{
		ID: NewID(), Name: "c", Seed: "http://h/", Scheme: "http", Host: "h", MaxDepth: 2, MaxPages: 50,
	}
	if err := db.CreateCrawlTask(ctx, task); err != nil {
		t.Fatalf("CreateCrawlTask: %v", err)
	}
	if err := db.PrepareCrawlRun(ctx, task.ID); err != nil {
		t.Fatalf("PrepareCrawlRun: %v", err)
	}
	ins, err := db.AddCrawlURL(ctx, &models.CrawlURL{
		ID: NewID(), TaskID: task.ID, URL: "http://h/a", Method: "GET", StatusCode: 200, Depth: 1,
	})
	if err != nil || !ins {
		t.Fatalf("AddCrawlURL = (%v, %v), want (true, nil)", ins, err)
	}
	// Per-task URL dedup.
	dup, err := db.AddCrawlURL(ctx, &models.CrawlURL{ID: NewID(), TaskID: task.ID, URL: "http://h/a"})
	if err != nil {
		t.Fatalf("AddCrawlURL dup: %v", err)
	}
	if dup {
		t.Error("duplicate URL should not be inserted")
	}
	urls, err := db.ListCrawlURLs(ctx, task.ID)
	if err != nil || len(urls) != 1 {
		t.Fatalf("ListCrawlURLs = %d (err %v), want 1", len(urls), err)
	}
}
