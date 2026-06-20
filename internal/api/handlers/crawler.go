package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/utkarshrai2811/redtrace/internal/crawler"
	"github.com/utkarshrai2811/redtrace/internal/storage"
	"github.com/utkarshrai2811/redtrace/internal/storage/models"
)

// crawlBody is the create payload for a crawl.
type crawlBody struct {
	Name     string `json:"name"`
	Seed     string `json:"seed"`
	MaxDepth int    `json:"maxDepth"`
	MaxPages int    `json:"maxPages"`
}

func (b crawlBody) normalized() crawlBody {
	if b.Name == "" {
		b.Name = "Crawl"
	}
	if b.MaxDepth <= 0 {
		b.MaxDepth = 3
	}
	if b.MaxDepth > 10 {
		b.MaxDepth = 10
	}
	if b.MaxPages <= 0 {
		b.MaxPages = 200
	}
	if b.MaxPages > 2000 {
		b.MaxPages = 2000
	}
	return b
}

// ListCrawlTasks handles GET /api/crawler/tasks.
func (a *API) ListCrawlTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.Store.ListCrawlTasks(r.Context())
	if err != nil {
		a.serverError(w, "list_failed", err)
		return
	}
	if tasks == nil {
		tasks = []*models.CrawlTask{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

// CreateCrawlTask handles POST /api/crawler/tasks.
func (a *API) CreateCrawlTask(w http.ResponseWriter, r *http.Request) {
	var body crawlBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	body = body.normalized()
	u, err := url.Parse(body.Seed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "invalid_seed", "seed must be an absolute http(s) URL")
		return
	}
	task := &models.CrawlTask{
		ID: storage.NewID(), Name: body.Name, Seed: u.String(), Scheme: u.Scheme, Host: u.Host,
		MaxDepth: body.MaxDepth, MaxPages: body.MaxPages, Status: crawler.StatusPending,
	}
	if err := a.Store.CreateCrawlTask(r.Context(), task); err != nil {
		a.serverError(w, "create_failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

// GetCrawlTask handles GET /api/crawler/tasks/{id}, returning the crawl and its
// discovered URLs.
func (a *API) GetCrawlTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetCrawlTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such crawl")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	urls, err := a.Store.ListCrawlURLs(r.Context(), task.ID)
	if err != nil {
		a.serverError(w, "urls_failed", err)
		return
	}
	if urls == nil {
		urls = []*models.CrawlURL{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task, "urls": urls})
}

// DeleteCrawlTask handles DELETE /api/crawler/tasks/{id}.
func (a *API) DeleteCrawlTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.Crawler.Stop(id)
	err := a.Store.DeleteCrawlTask(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such crawl")
		return
	}
	if err != nil {
		a.serverError(w, "delete_failed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartCrawlTask handles POST /api/crawler/tasks/{id}/start.
func (a *API) StartCrawlTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.Store.GetCrawlTask(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no such crawl")
		return
	}
	if err != nil {
		a.serverError(w, "get_failed", err)
		return
	}
	cfg := crawler.Config{Seed: task.Seed, MaxDepth: task.MaxDepth, MaxPages: task.MaxPages, HTTPVersion: "HTTP/1.1"}
	started, err := a.Crawler.Start(task.ID, cfg)
	if err != nil {
		a.serverError(w, "start_failed", err)
		return
	}
	if !started {
		writeError(w, http.StatusConflict, "already_running", "crawl is already running")
		return
	}
	task.Status, task.Pages, task.Found = crawler.StatusRunning, 0, 0
	writeJSON(w, http.StatusOK, task)
}

// StopCrawlTask handles POST /api/crawler/tasks/{id}/stop.
func (a *API) StopCrawlTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a.Crawler.Stop(id) {
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	if task, err := a.Store.GetCrawlTask(r.Context(), id); err == nil && task.Status == crawler.StatusRunning {
		_ = a.Store.SetCrawlStatus(r.Context(), id, crawler.StatusStopped)
		writeJSON(w, http.StatusOK, map[string]any{"stopped": true})
		return
	}
	writeError(w, http.StatusConflict, "not_running", "crawl is not running")
}

// CrawlerStore adapts the storage layer to the crawler.Store interface, storing
// each fetched page as an exchange (so it appears in history / site map) and as
// a per-crawl discovered-URL row.
type CrawlerStore struct {
	DB *storage.DB
}

func (s CrawlerStore) PrepareCrawlRun(ctx context.Context, taskID string) error {
	return s.DB.PrepareCrawlRun(ctx, taskID)
}

func (s CrawlerStore) AddPage(ctx context.Context, taskID string, p crawler.Page) (bool, error) {
	now := time.Now()
	reqID := storage.NewID()
	req := &models.Request{
		ID: reqID, Timestamp: now, Source: models.SourceCrawler, Method: p.Method,
		Scheme: p.Scheme, Host: p.Host, Port: p.Port, Path: p.Path, Query: p.Query,
		URL: p.URL, HTTPVersion: "HTTP/1.1", InScope: true, Raw: p.RequestRaw,
	}
	resp := &models.Response{
		ID: storage.NewID(), RequestID: reqID, Timestamp: now, StatusCode: p.StatusCode,
		HTTPVersion: "HTTP/1.1", MimeType: p.ContentType, BodySize: p.Length,
		DurationMs: p.DurationMs, Raw: p.ResponseRaw,
	}
	if err := s.DB.StoreExchange(ctx, &models.Exchange{Request: req, Response: resp}); err != nil {
		return false, err
	}
	return s.DB.AddCrawlURL(ctx, &models.CrawlURL{
		ID: storage.NewID(), TaskID: taskID, URL: p.URL, Method: p.Method,
		StatusCode: p.StatusCode, Length: p.Length, ContentType: p.ContentType, Depth: p.Depth,
	})
}

func (s CrawlerStore) UpdateCrawlProgress(ctx context.Context, taskID string, pages, found int) error {
	return s.DB.UpdateCrawlProgress(ctx, taskID, pages, found)
}

func (s CrawlerStore) SetCrawlStatus(ctx context.Context, taskID, status string) error {
	return s.DB.SetCrawlStatus(ctx, taskID, status)
}
