package quota_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/quota"
)

func TestScraper_fetch_account_parses_dashboard(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/wrk_demo/go" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Cookie"); got != testCookie {
			t.Errorf("cookie = %q", got)
		}
		_, _ = writer.Write([]byte(`
			rollingUsage: $R[0] = { usagePercent: 10, resetInSec: 60 }
			weeklyUsage: $R[0] = { usagePercent: 20, resetInSec: 120 }
			monthlyUsage: $R[0] = { usagePercent: 30, resetInSec: 180 }
		`))
	}))
	defer server.Close()
	scraper, err := quota.NewScraper(quota.ScraperConfig{
		DashboardBase: server.URL, HTTPClient: server.Client(), Timeout: time.Second,
		Now: func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewScraper: %v", err)
	}
	target := quota.ScrapeTarget{
		Account: quota.Account{
			ID: "acct_1", Name: "primary", WorkspaceID: "wrk_demo",
			ShowRolling: true, ShowWeekly: true, ShowMonthly: true,
		},
		AuthCookie: testCookie,
	}

	// When
	result := scraper.FetchAccount(context.Background(), target)

	// Then
	if !result.Success || len(result.Windows) != 3 || result.Error != "" {
		t.Fatalf("result = %+v", result)
	}
	if result.Windows[0].Used != 10 {
		t.Fatalf("rolling used = %v", result.Windows[0].Used)
	}
}

func TestScraper_fetch_all_isolates_failures(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "wrk_bad") {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`rollingUsage: $R[0] = { usagePercent: 5, resetInSec: 10 }`))
	}))
	defer server.Close()
	scraper, err := quota.NewScraper(quota.ScraperConfig{
		DashboardBase: server.URL, HTTPClient: server.Client(), Timeout: time.Second, Concurrency: 2,
		Now: func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewScraper: %v", err)
	}
	targets := []quota.ScrapeTarget{
		{
			Account:    quota.Account{ID: "a", Name: "good", WorkspaceID: "wrk_good", ShowRolling: true},
			AuthCookie: testCookie,
		},
		{
			Account:    quota.Account{ID: "b", Name: "bad", WorkspaceID: "wrk_bad", ShowRolling: true},
			AuthCookie: testCookie,
		},
	}

	// When
	results := scraper.FetchAll(context.Background(), targets)

	// Then
	if len(results) != 2 {
		t.Fatalf("results len = %d", len(results))
	}
	if !results[0].Success || len(results[0].Windows) != 1 {
		t.Fatalf("good result = %+v", results[0])
	}
	if results[1].Success || !strings.Contains(results[1].Error, "401") {
		t.Fatalf("bad result = %+v", results[1])
	}
}

func TestScraper_fetch_account_rejects_empty_parse(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<html>no quota data</html>`))
	}))
	defer server.Close()
	scraper, err := quota.NewScraper(quota.ScraperConfig{
		DashboardBase: server.URL, HTTPClient: server.Client(), Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewScraper: %v", err)
	}

	// When
	result := scraper.FetchAccount(context.Background(), quota.ScrapeTarget{
		Account:    quota.Account{ID: "a", Name: "x", WorkspaceID: "wrk_x", ShowRolling: true},
		AuthCookie: testCookie,
	})

	// Then
	if result.Success || result.Error == "" {
		t.Fatalf("result = %+v", result)
	}
}
