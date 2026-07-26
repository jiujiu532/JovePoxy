package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Current is the product release version for this binary / UI build.
// Override at build time: -ldflags "-X jovepoxy/internal/version.Current=0.0.2"
var Current = "0.0.1"

// DefaultRepo is the GitHub repo used for release checks when VERSION_REPO is unset.
const DefaultRepo = "jiujiu532/JovePoxy"

// DefaultImage is the display name for the container image (not necessarily published).
func DefaultImage() string {
	if v := strings.TrimSpace(os.Getenv("VERSION_IMAGE")); v != "" {
		return v
	}
	return "jovepoxy"
}

// Info is the admin version snapshot.
type Info struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	UpdateAvailable bool      `json:"update_available"`
	Image           string    `json:"image"`
	CheckedAt       time.Time `json:"checked_at"`
	Source          string    `json:"source"`
	Note            string    `json:"note,omitempty"`
}

type Checker struct {
	client *http.Client
	mu     sync.Mutex
	cache  *Info
}

func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// Get returns cached info or refreshes when force is true / cache empty.
func (c *Checker) Get(ctx context.Context, force bool) Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.cache != nil && time.Since(c.cache.CheckedAt) < 10*time.Minute {
		return *c.cache
	}
	info := c.fetch(ctx)
	c.cache = &info
	return info
}

func (c *Checker) fetch(ctx context.Context) Info {
	now := time.Now().UTC()
	info := Info{
		Current:   normalize(Current),
		Latest:    normalize(Current),
		Image:     DefaultImage(),
		CheckedAt: now,
		Source:    "local",
	}

	repo := strings.TrimSpace(os.Getenv("VERSION_REPO"))
	if repo == "" {
		repo = DefaultRepo
	}

	latest, err := c.fetchGitHubLatest(ctx, repo)
	if err != nil {
		info.Note = "远端检查失败: " + err.Error()
		info.Source = "github"
		return info
	}
	info.Latest = normalize(latest)
	info.Source = "github"
	info.UpdateAvailable = isNewer(info.Latest, info.Current)
	if info.UpdateAvailable {
		info.Note = fmt.Sprintf("可更新: v%s → v%s", info.Current, info.Latest)
	} else {
		info.Note = "已是最新版本"
	}
	return info
}

func (c *Checker) fetchGitHubLatest(ctx context.Context, repo string) (string, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "jovepoxy-version-check")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub HTTP %d", resp.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("empty tag_name")
	}
	return payload.TagName, nil
}

func normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// isNewer reports whether a is semver-newer than b (simple dotted numeric compare).
func isNewer(a, b string) bool {
	as := splitNums(a)
	bs := splitNums(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av > bv {
			return true
		}
		if av < bv {
			return false
		}
	}
	return false
}

func splitNums(v string) []int {
	parts := strings.Split(normalize(v), ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out = append(out, n)
	}
	return out
}
