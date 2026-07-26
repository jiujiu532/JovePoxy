package usage_test

import (
	"testing"

	"jovepoxy/internal/usage"
)

const sampleUsage = `
$R[26]={id:"usg_01KX2Z5HDXVJ9MGSBPF2WYQZCQ",workspaceID:"wrk_01KVZAXQQS6ZJ6D5W2195DY9W8",
timeCreated:$R[27]=new Date("2026-07-09T08:16:06.000Z"),timeUpdated:$R[28]=new Date("2026-07-09T08:16:06.086Z"),
timeDeleted:null,model:"glm-5.2",provider:"deepinfra-glm-5.2",inputTokens:78675,outputTokens:177,
cost:11092380,keyID:"key_01KVZDHTH6F9NCZW5AMFB7RMCJ",sessionID:"",enrichment:$R[29]={plan:"lite"}}
`

func TestParseUsageResponse_extracts_record_and_plan(t *testing.T) {
	// When
	records := usage.ParseUsageResponse(sampleUsage)

	// Then
	if len(records) != 1 {
		t.Fatalf("records = %d", len(records))
	}
	record := records[0]
	if record.USGID != "usg_01KX2Z5HDXVJ9MGSBPF2WYQZCQ" || record.Model != "glm-5.2" {
		t.Fatalf("record = %+v", record)
	}
	if record.InputTokens != 78675 || record.OutputTokens != 177 || record.CostRaw != 11092380 {
		t.Fatalf("tokens/cost = %+v", record)
	}
	if record.Plan != "lite" || record.KeyID == "" {
		t.Fatalf("plan/key = %+v", record)
	}
}

func TestParseUsageResponse_empty_returns_nil_slice(t *testing.T) {
	// When
	records := usage.ParseUsageResponse("no usage data")

	// Then
	if len(records) != 0 {
		t.Fatalf("records = %+v", records)
	}
}
