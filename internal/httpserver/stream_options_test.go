package httpserver

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEnsureStreamIncludeUsage_injectsWhenMissing(t *testing.T) {
	body := []byte(`{"model":"m","stream":true,"messages":[]}`)
	out := ensureStreamIncludeUsage(body, true)
	var object map[string]json.RawMessage
	if err := json.Unmarshal(out, &object); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(object["stream_options"], []byte(`"include_usage":true`)) {
		t.Fatalf("got %s", object["stream_options"])
	}
}

func TestEnsureStreamIncludeUsage_preservesExisting(t *testing.T) {
	body := []byte(`{"model":"m","stream":true,"stream_options":{"include_usage":false},"messages":[]}`)
	out := ensureStreamIncludeUsage(body, true)
	if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(body)) && !bytes.Contains(out, []byte(`"include_usage":false`)) {
		t.Fatalf("should preserve existing stream_options, got %s", out)
	}
}

func TestEnsureStreamIncludeUsage_nonstreamUnchanged(t *testing.T) {
	body := []byte(`{"model":"m","stream":false,"messages":[]}`)
	out := ensureStreamIncludeUsage(body, false)
	if !bytes.Equal(out, body) {
		t.Fatalf("non-stream body mutated: %s", out)
	}
}
