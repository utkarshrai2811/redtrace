package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func postJSON(t *testing.T, url, body string, v any) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if v != nil {
		_ = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp.StatusCode
}

func TestAPI_Decoder(t *testing.T) {
	ts, _ := newTestServer(t)
	var out struct {
		Output string `json:"output"`
	}
	if code := postJSON(t, ts.URL+"/api/decoder/run",
		`{"input":"aGVsbG8=","operations":["base64_decode"]}`, &out); code != 200 {
		t.Fatalf("status = %d", code)
	}
	if out.Output != "hello" {
		t.Errorf("output = %q, want hello", out.Output)
	}
}

func TestAPI_Comparer(t *testing.T) {
	ts, _ := newTestServer(t)
	var out struct {
		Segments []struct {
			Op   string `json:"op"`
			Text string `json:"text"`
		} `json:"segments"`
	}
	if code := postJSON(t, ts.URL+"/api/comparer",
		`{"a":"alpha\nbravo\n","b":"alpha\ncharlie\n","mode":"lines"}`, &out); code != 200 {
		t.Fatalf("status = %d", code)
	}
	var hasInsert, hasDelete bool
	for _, s := range out.Segments {
		hasInsert = hasInsert || s.Op == "insert"
		hasDelete = hasDelete || s.Op == "delete"
	}
	if !hasInsert || !hasDelete {
		t.Errorf("expected insert+delete segments, got %+v", out.Segments)
	}
}

func TestAPI_SendToRepeaterAndTabCRUD(t *testing.T) {
	ts, db := newTestServer(t)
	id := seed(t, db)

	var tab struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Host string `json:"host"`
	}
	if code := postJSON(t, ts.URL+"/api/requests/"+id+"/send-to-repeater", "", &tab); code != http.StatusCreated {
		t.Fatalf("send-to-repeater status = %d", code)
	}
	if tab.ID == "" || tab.Host != "example.com:443" {
		t.Fatalf("unexpected tab: %+v", tab)
	}

	var tabs []map[string]any
	getJSON(t, ts.URL+"/api/repeater/tabs", &tabs)
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(tabs))
	}

	// Tab detail includes raw + (empty) history.
	var detail struct {
		Tab     map[string]any   `json:"tab"`
		History []map[string]any `json:"history"`
	}
	getJSON(t, ts.URL+"/api/repeater/tabs/"+tab.ID, &detail)
	if detail.Tab["id"] != tab.ID || len(detail.History) != 0 {
		t.Errorf("unexpected detail: %+v", detail)
	}

	if code := statusOf(t, http.MethodDelete, ts.URL+"/api/repeater/tabs/"+tab.ID, ""); code != http.StatusNoContent {
		t.Errorf("delete status = %d", code)
	}
}

func TestAPI_Sitemap(t *testing.T) {
	ts, db := newTestServer(t)
	seed(t, db)

	var hosts []struct {
		Host  string `json:"host"`
		Paths []struct {
			Path string `json:"path"`
		} `json:"paths"`
	}
	getJSON(t, ts.URL+"/api/sitemap", &hosts)
	if len(hosts) == 0 {
		t.Fatal("expected at least one host in sitemap")
	}

	if code := postPut(t, ts.URL+"/api/sitemap/note", `{"host":"example.com","path":"/","note":"home","tags":"x"}`); code != http.StatusNoContent {
		t.Errorf("put note status = %d", code)
	}
}

func postPut(t *testing.T, url, body string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}
