package ai

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// readSSE parses a text/event-stream body, invoking handle once per event with
// the concatenated data payload. handle returns stop=true to end reading early
// (e.g. on a terminal event). Comment/keep-alive lines and field names other
// than "data" are ignored — callers dispatch on the JSON payload itself.
func readSSE(r io.Reader, handle func(data []byte) (stop bool, err error)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var data strings.Builder
	dispatch := func() (bool, error) {
		if data.Len() == 0 {
			return false, nil
		}
		payload := data.String()
		data.Reset()
		return handle([]byte(payload))
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" { // event boundary
			stop, err := dispatch()
			if err != nil || stop {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") { // comment / keep-alive
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(v, " "))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	// Flush a final event that wasn't terminated by a blank line.
	_, err := dispatch()
	return err
}

// providerError builds an error from a non-2xx provider response, including a
// bounded snippet of the body (the operator's own provider/credentials).
func providerError(name string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("%s provider error (HTTP %d): %s", name, resp.StatusCode, truncate(msg, 500))
}
