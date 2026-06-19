package storage

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// SitemapPath is a single endpoint (path) under a host in the site map.
type SitemapPath struct {
	Path    string   `json:"path"`
	Methods []string `json:"methods"`
	Count   int      `json:"count"`
	InScope bool     `json:"inScope"`
	Note    string   `json:"note,omitempty"`
	Tags    string   `json:"tags,omitempty"`
}

// SitemapHost groups the observed paths for one host.
type SitemapHost struct {
	Host  string        `json:"host"`
	Count int           `json:"count"`
	Paths []SitemapPath `json:"paths"`
}

// Sitemap builds the host → path tree from observed traffic, merging any saved
// notes/tags.
func (db *DB) Sitemap(ctx context.Context) ([]SitemapHost, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT host, path, method, in_scope, COUNT(*)
		 FROM requests GROUP BY host, path, method ORDER BY host, path`)
	if err != nil {
		return nil, fmt.Errorf("sitemap query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pathAgg struct {
		methods map[string]struct{}
		count   int
		inScope bool
	}
	hosts := map[string]map[string]*pathAgg{}
	var hostOrder []string

	for rows.Next() {
		var host, path, method string
		var inScope bool
		var count int
		if err := rows.Scan(&host, &path, &method, &inScope, &count); err != nil {
			return nil, fmt.Errorf("scan sitemap: %w", err)
		}
		if _, ok := hosts[host]; !ok {
			hosts[host] = map[string]*pathAgg{}
			hostOrder = append(hostOrder, host)
		}
		agg := hosts[host][path]
		if agg == nil {
			agg = &pathAgg{methods: map[string]struct{}{}}
			hosts[host][path] = agg
		}
		agg.methods[method] = struct{}{}
		agg.count += count
		agg.inScope = agg.inScope || inScope
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	notes, err := db.sitemapNotes(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]SitemapHost, 0, len(hostOrder))
	for _, host := range hostOrder {
		paths := hosts[host]
		sh := SitemapHost{Host: host, Paths: make([]SitemapPath, 0, len(paths))}
		pathKeys := make([]string, 0, len(paths))
		for p := range paths {
			pathKeys = append(pathKeys, p)
		}
		sort.Strings(pathKeys)
		for _, p := range pathKeys {
			agg := paths[p]
			methods := make([]string, 0, len(agg.methods))
			for m := range agg.methods {
				methods = append(methods, m)
			}
			sort.Strings(methods)
			sp := SitemapPath{Path: p, Methods: methods, Count: agg.count, InScope: agg.inScope}
			if n, ok := notes[host+"\x00"+p]; ok {
				sp.Note, sp.Tags = n[0], n[1]
			}
			sh.Paths = append(sh.Paths, sp)
			sh.Count += agg.count
		}
		out = append(out, sh)
	}
	return out, nil
}

func (db *DB) sitemapNotes(ctx context.Context) (map[string][2]string, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT host, path, COALESCE(note,''), COALESCE(tags,'') FROM sitemap_notes`)
	if err != nil {
		return nil, fmt.Errorf("sitemap notes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string][2]string{}
	for rows.Next() {
		var host, path, note, tags string
		if err := rows.Scan(&host, &path, &note, &tags); err != nil {
			return nil, err
		}
		out[host+"\x00"+path] = [2]string{note, tags}
	}
	return out, rows.Err()
}

// UpsertSitemapNote sets the note/tags for a host+path endpoint.
func (db *DB) UpsertSitemapNote(ctx context.Context, host, path, note, tags string) error {
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO sitemap_notes (id, host, path, note, tags, updated_at)
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT(host, path) DO UPDATE SET note=excluded.note, tags=excluded.tags, updated_at=excluded.updated_at`,
		NewID(), host, path, note, tags, time.Now().Format(timeLayout))
	if err != nil {
		return fmt.Errorf("upsert sitemap note: %w", err)
	}
	return nil
}
