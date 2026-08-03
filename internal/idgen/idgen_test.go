package idgen

import (
	"encoding/base64"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPrefixedLengthAndCharset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		prefix string
		n      int
		hexLen int
	}{
		{"zk_", 16, 32},
		{"px_", 16, 32},
		{"key_", 16, 32},
		{"acct_", 16, 32},
		{"ollama_", 16, 32},
		{"ses_", 12, 24},
		{"ur_", 12, 24},
		{"rl_", 12, 24},
		{"resp_", 12, 24},
		{"msg_", 12, 24},
		{"fc_", 12, 24},
		{"call_", 12, 24},
		{"rs_", 12, 24},
	}
	hexOnly := regexp.MustCompile(`^[0-9a-f]+$`)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.prefix, func(t *testing.T) {
			t.Parallel()
			id, err := Prefixed(tc.prefix, tc.n)
			if err != nil {
				t.Fatalf("Prefixed: %v", err)
			}
			if !strings.HasPrefix(id, tc.prefix) {
				t.Fatalf("prefix: got %q want prefix %q", id, tc.prefix)
			}
			suffix := strings.TrimPrefix(id, tc.prefix)
			if len(suffix) != tc.hexLen {
				t.Fatalf("hex length: got %d want %d (%q)", len(suffix), tc.hexLen, id)
			}
			if !hexOnly.MatchString(suffix) {
				t.Fatalf("suffix not lowercase hex: %q", suffix)
			}
		})
	}
}

func TestPrefixedRejectsNonPositiveN(t *testing.T) {
	t.Parallel()
	if _, err := Prefixed("x_", 0); err == nil {
		t.Fatal("expected error for n=0")
	}
	if _, err := Prefixed("x_", -1); err == nil {
		t.Fatal("expected error for n<0")
	}
}

func TestOpenCodeFormat(t *testing.T) {
	t.Parallel()
	// prefix_ + unixMilli hex + rawurl base64 of 12 bytes (16 chars, no padding)
	re := regexp.MustCompile(`^msg_[0-9a-f]+[A-Za-z0-9_-]{16}$`)
	before := time.Now().UnixMilli()
	id, err := OpenCode("msg")
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("OpenCode: %v", err)
	}
	if !re.MatchString(id) {
		t.Fatalf("shape: %q", id)
	}
	if !strings.HasPrefix(id, "msg_") {
		t.Fatalf("prefix: %q", id)
	}
	body := strings.TrimPrefix(id, "msg_")
	if len(body) < 17 {
		t.Fatalf("too short: %q", id)
	}
	milliHex := body[:len(body)-16]
	b64 := body[len(body)-16:]
	milli, err := strconv.ParseInt(milliHex, 16, 64)
	if err != nil {
		t.Fatalf("parse milli %q: %v", milliHex, err)
	}
	if milli < before || milli > after {
		t.Fatalf("milli %d not in [%d,%d]", milli, before, after)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(decoded) != 12 {
		t.Fatalf("random bytes: got %d want 12", len(decoded))
	}
}
