package usage

import (
	"regexp"
	"strconv"
)

// PageSize is the OpenCode usage server page size used by QuotaHub.
const PageSize = 50

// Record is one parsed usage row (control plane only).
type Record struct {
	USGID        string
	CreatedAt    string
	Model        string
	Provider     string
	InputTokens  int
	OutputTokens int
	CostRaw      int64
	CostUSD      float64
	KeyID        string
	Plan         string
}

var (
	recordRE = regexp.MustCompile(
		`id:"(usg_[^"]+)"[^}]*?` +
			`timeCreated:\$R\[\d+\]=new Date\("([^"]+)"\)[^}]*?` +
			`model:"([^"]+)"[^}]*?provider:"([^"]+)"[^}]*?` +
			`inputTokens:(\d+)[^}]*?outputTokens:(\d+)[^}]*?` +
			`cost:([0-9]+)[^}]*?keyID:"([^"]+)"`,
	)
	planRE = regexp.MustCompile(`id:"(usg_[^"]+)"[^}]*?enrichment:\$R\[\d+\]=\{plan:"([^"]+)"\}`)
)

// ParseUsageResponse extracts usage records from an OpenCode _server JS payload.
func ParseUsageResponse(text string) []Record {
	plans := map[string]string{}
	for _, match := range planRE.FindAllStringSubmatch(text, -1) {
		if len(match) == 3 {
			plans[match[1]] = match[2]
		}
	}
	matches := recordRE.FindAllStringSubmatch(text, -1)
	records := make([]Record, 0, len(matches))
	for _, match := range matches {
		if len(match) != 9 {
			continue
		}
		costRaw, _ := strconv.ParseInt(match[7], 10, 64)
		inputTokens, _ := strconv.Atoi(match[5])
		outputTokens, _ := strconv.Atoi(match[6])
		records = append(records, Record{
			USGID: match[1], CreatedAt: match[2], Model: match[3], Provider: match[4],
			InputTokens: inputTokens, OutputTokens: outputTokens, CostRaw: costRaw,
			CostUSD: float64(costRaw) / 1_000_000_000, KeyID: match[8], Plan: plans[match[1]],
		})
	}
	return records
}
