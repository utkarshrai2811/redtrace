package crawler

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/utkarshrai2811/redtrace/internal/repeater"
)

const (
	defaultConcurrency = 8
	progressEvery      = 10
)

// Status values for a crawl's lifecycle.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusStopped   = "stopped"
	StatusError     = "error"
)

type activeRun struct {
	cancel context.CancelFunc
}

// Crawler runs scope-bounded crawls in the background. It is safe for concurrent use.
type Crawler struct {
	sender  *repeater.Engine
	store   Store
	notify  func(Update)
	inScope func(host, path string) bool
	onPage  func(Page) // optional: e.g. passively scan each fetched page

	mu      sync.Mutex
	running map[string]*activeRun
	wg      sync.WaitGroup
}

// NewCrawler wires a Crawler to its store, notifier, scope check, and an optional
// per-page hook.
func NewCrawler(store Store, notify func(Update), inScope func(host, path string) bool, onPage func(Page)) *Crawler {
	if inScope == nil {
		inScope = func(string, string) bool { return true }
	}
	return &Crawler{
		sender:  repeater.New(),
		store:   store,
		notify:  notify,
		inScope: inScope,
		onPage:  onPage,
		running: make(map[string]*activeRun),
	}
}

// Start prepares and launches a crawl in the background. It reports false (no
// error) if a crawl for this task is already running.
func (c *Crawler) Start(taskID string, cfg Config) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	if _, exists := c.running[taskID]; exists {
		c.mu.Unlock()
		cancel()
		return false, nil
	}
	run := &activeRun{cancel: cancel}
	c.running[taskID] = run
	c.mu.Unlock()

	bg := context.Background()
	if err := c.store.PrepareCrawlRun(bg, taskID); err != nil {
		c.release(taskID, run)
		cancel()
		return false, err
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			c.release(taskID, run)
			cancel()
		}()

		c.emit(Update{Kind: "status", TaskID: taskID, Status: StatusRunning})
		pages, found := c.runCrawl(ctx, bg, taskID, cfg)

		status := StatusCompleted
		if ctx.Err() != nil {
			status = StatusStopped
		}
		_ = c.store.SetCrawlStatus(bg, taskID, status)
		c.emit(Update{Kind: "status", TaskID: taskID, Status: status, Pages: pages, Found: found})
	}()
	return true, nil
}

func (c *Crawler) runCrawl(ctx, bg context.Context, taskID string, cfg Config) (int, int) {
	seed, err := url.Parse(cfg.Seed)
	if err != nil || (seed.Scheme != "http" && seed.Scheme != "https") || seed.Host == "" {
		return 0, 0
	}
	seedHost := strings.ToLower(seed.Hostname())
	maxPages := cfg.MaxPages
	if maxPages <= 0 {
		maxPages = 200
	}

	var mu sync.Mutex
	discovered := map[string]bool{seed.String(): true}
	pages := 0

	progress := func() {
		mu.Lock()
		p, f := pages, len(discovered)
		mu.Unlock()
		_ = c.store.UpdateCrawlProgress(bg, taskID, p, f)
		c.emit(Update{Kind: "progress", TaskID: taskID, Status: StatusRunning, Pages: p, Found: f})
	}

	frontier := []string{seed.String()}
	for depth := 0; depth <= cfg.MaxDepth; depth++ {
		mu.Lock()
		stop := len(frontier) == 0 || pages >= maxPages
		mu.Unlock()
		if stop || ctx.Err() != nil {
			break
		}

		nextSet := map[string]bool{}
		jobs := make(chan string)
		var wg sync.WaitGroup
		for w := 0; w < defaultConcurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for u := range jobs {
					if ctx.Err() != nil {
						continue
					}
					mu.Lock()
					over := pages >= maxPages
					mu.Unlock()
					if over {
						continue
					}
					page, links, ok := c.fetch(ctx, cfg, u, depth)
					if !ok {
						continue
					}
					mu.Lock()
					if pages >= maxPages {
						mu.Unlock()
						continue
					}
					pages++
					p := pages
					if depth < cfg.MaxDepth {
						for _, l := range links {
							if !discovered[l] && c.allowed(l, seedHost) {
								discovered[l] = true
								nextSet[l] = true
							}
						}
					}
					mu.Unlock()

					if _, err := c.store.AddPage(bg, taskID, page); err == nil {
						c.emit(Update{Kind: "page", TaskID: taskID, Status: StatusRunning, Page: &PageSummary{
							URL: page.URL, StatusCode: page.StatusCode, ContentType: page.ContentType, Depth: page.Depth,
						}})
					}
					if c.onPage != nil {
						c.onPage(page)
					}
					if p%progressEvery == 0 {
						progress()
					}
				}
			}()
		}

	feed:
		for _, u := range frontier {
			select {
			case jobs <- u:
			case <-ctx.Done():
				break feed
			}
		}
		close(jobs)
		wg.Wait()

		frontier = frontier[:0]
		for l := range nextSet {
			frontier = append(frontier, l)
		}
	}

	progress()
	mu.Lock()
	defer mu.Unlock()
	return pages, len(discovered)
}

// fetch retrieves one URL and returns the page plus the links discovered in it.
func (c *Crawler) fetch(ctx context.Context, cfg Config, rawURL string, depth int) (Page, []string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return Page{}, nil, false
	}
	host, port := hostPort(u)
	raw := []byte("GET " + u.RequestURI() + " HTTP/1.1\r\nHost: " + u.Host +
		"\r\nUser-Agent: RedTrace-Crawler\r\nAccept: text/html,application/xhtml+xml,*/*\r\nConnection: keep-alive\r\n\r\n")
	resp, err := c.sender.Send(ctx, repeater.Request{
		Scheme: u.Scheme, Host: u.Host, Raw: raw, FollowRedirects: false, HTTPVersion: cfg.HTTPVersion,
	})
	if err != nil {
		return Page{}, nil, false
	}
	headers, body := parseResponse(resp.Raw)
	ct := mimeOf(headers.Get("Content-Type"))
	page := Page{
		URL: u.String(), Method: "GET", Scheme: u.Scheme, Host: host, Port: port,
		Path: u.Path, Query: u.RawQuery, StatusCode: resp.StatusCode, ContentType: ct,
		Length: len(resp.Raw), Depth: depth, DurationMs: resp.DurationMs, RequestRaw: raw, ResponseRaw: resp.Raw,
	}
	var links []string
	if strings.Contains(ct, "html") {
		links = extractLinks(u, body)
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := headers.Get("Location"); loc != "" {
			if ref, err := url.Parse(loc); err == nil {
				links = append(links, u.ResolveReference(ref).String())
			}
		}
	}
	return page, links, true
}

// allowed reports whether a discovered link should be crawled: same host as the
// seed and in scope.
func (c *Crawler) allowed(link, seedHost string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	if strings.ToLower(u.Hostname()) != seedHost {
		return false
	}
	return c.inScope(u.Hostname(), u.Path)
}

// Stop cancels a running crawl, reporting whether it was running.
func (c *Crawler) Stop(taskID string) bool {
	c.mu.Lock()
	run, ok := c.running[taskID]
	c.mu.Unlock()
	if ok {
		run.cancel()
	}
	return ok
}

// Shutdown cancels every running crawl and waits for them to drain, bounded by ctx.
func (c *Crawler) Shutdown(ctx context.Context) {
	c.mu.Lock()
	for _, run := range c.running {
		run.cancel()
	}
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (c *Crawler) release(taskID string, run *activeRun) {
	c.mu.Lock()
	if c.running[taskID] == run {
		delete(c.running, taskID)
	}
	c.mu.Unlock()
}

func (c *Crawler) emit(u Update) {
	if c.notify != nil {
		c.notify(u)
	}
}
