package zenpool_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

type permanentNetError struct{}

func (permanentNetError) Error() string   { return "connection refused" }
func (permanentNetError) Timeout() bool   { return false }
func (permanentNetError) Temporary() bool { return false }

func TestShouldFailover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "wrapped context canceled", err: fmt.Errorf("request cancelled: %w", context.Canceled), want: false},
		{name: "401 unauthorized", err: &zen.StatusError{StatusCode: http.StatusUnauthorized}, want: true},
		{name: "429 rate limit", err: &zen.StatusError{StatusCode: http.StatusTooManyRequests}, want: true},
		{name: "500 server error", err: &zen.StatusError{StatusCode: http.StatusInternalServerError}, want: true},
		{name: "503 unavailable", err: &zen.StatusError{StatusCode: http.StatusServiceUnavailable}, want: true},
		{name: "400 bad request", err: &zen.StatusError{StatusCode: http.StatusBadRequest}, want: false},
		{name: "403 forbidden", err: &zen.StatusError{StatusCode: http.StatusForbidden}, want: false},
		{name: "404 not found", err: &zen.StatusError{StatusCode: http.StatusNotFound}, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped deadline exceeded", err: fmt.Errorf("upstream: %w", context.DeadlineExceeded), want: true},
		{name: "os deadline exceeded", err: os.ErrDeadlineExceeded, want: true},
		{name: "zen TimeoutError", err: &zen.TimeoutError{}, want: true},
		{name: "wrapped zen TimeoutError", err: fmt.Errorf("send: %w", &zen.TimeoutError{}), want: true},
		{name: "net timeout", err: timeoutNetError{}, want: true},
		{name: "wrapped net timeout", err: fmt.Errorf("dial: %w", timeoutNetError{}), want: true},
		{name: "connection refused", err: permanentNetError{}, want: true},
		{name: "generic dial failure", err: errors.New("dial tcp: connection reset by peer"), want: true},
		{name: "net.OpError dial", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connectex: No connection could be made")}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := zenpool.ShouldFailover(tt.err)
			if got != tt.want {
				t.Fatalf("ShouldFailover(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestProxyPaid_failsover_on_timeout(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "slow", Secret: "slow-key"}); err != nil {
		t.Fatalf("create slow: %v", err)
	}
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "good-key"}); err != nil {
		t.Fatalf("create good: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.TimeoutError{}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}

	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{"model":"paid"}`), false, "", "")
	if err != nil {
		t.Fatalf("ProxyPaid: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || dialer.calls != 2 {
		t.Fatalf("status=%d calls=%d, want 200 with 2 attempts", response.StatusCode, dialer.calls)
	}
}

func TestProxyPaid_failsover_on_network_error(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "down", Secret: "down-key"}); err != nil {
		t.Fatalf("create down: %v", err)
	}
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "good", Secret: "good-key"}); err != nil {
		t.Fatalf("create good: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}

	response, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if err != nil {
		t.Fatalf("ProxyPaid: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || dialer.calls != 2 {
		t.Fatalf("status=%d calls=%d, want 200 with 2 attempts", response.StatusCode, dialer.calls)
	}
}

func TestProxyPaid_no_failover_on_canceled(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "a", Secret: "key-a"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "b", Secret: "key-b"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: context.Canceled},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}

	_, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if dialer.calls != 1 {
		t.Fatalf("calls = %d, want 1 (no failover on cancel)", dialer.calls)
	}
}

func TestProxyPaid_no_failover_on_400(t *testing.T) {
	service := newPool(t)
	ctx := context.Background()
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "a", Secret: "key-a"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := service.Create(ctx, zenpool.CreateInput{Label: "b", Secret: "key-b"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	dialer := &scriptedDialer{responses: []dialResult{
		{err: &zen.StatusError{StatusCode: http.StatusBadRequest}},
		{response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}},
	}}

	_, err := zenpool.ProxyPaid(ctx, service, dialer, json.RawMessage(`{}`), false, "", "")
	if err == nil {
		t.Fatal("expected 400 error without failover")
	}
	var status *zen.StatusError
	if !errors.As(err, &status) || status.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want StatusError 400", err)
	}
	if dialer.calls != 1 {
		t.Fatalf("calls = %d, want 1", dialer.calls)
	}
}
