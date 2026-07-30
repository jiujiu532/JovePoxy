package zenpool

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strings"
)

// ConversationAffinityKey builds a stable hash material from common conversation/session fields.
// Only the hash is returned — raw ids are never stored or logged by this helper.
//
// Priority:
//  1. Header X-Conversation-Id / X-Session-Id / X-Client-Request-Id
//  2. JSON body conversation_id / session_id / user
//  3. empty string when nothing usable is present (caller should fall back to spread)
func ConversationAffinityKey(headers http.Header, body json.RawMessage) string {
	raw := firstNonEmpty(
		headerValue(headers, "X-Conversation-Id"),
		headerValue(headers, "X-Session-Id"),
		headerValue(headers, "X-Client-Request-Id"),
	)
	if raw == "" && len(body) > 0 {
		var probe struct {
			ConversationID string `json:"conversation_id"`
			SessionID      string `json:"session_id"`
			User           string `json:"user"`
		}
		if err := json.Unmarshal(body, &probe); err == nil {
			raw = firstNonEmpty(probe.ConversationID, probe.SessionID, probe.User)
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Cap length before hashing so huge headers cannot inflate work.
	if len(raw) > 256 {
		raw = raw[:256]
	}
	sum := sha256.Sum256([]byte(raw))
	// 16-byte hex (32 chars) is enough for rendezvous stability without retaining raw material.
	return hex.EncodeToString(sum[:16])
}

// weightedRendezvousPick chooses the candidate with the highest weighted rendezvous score
// for the given affinity key. Candidates must be non-empty; weight <= 0 is treated as 1.
func weightedRendezvousPick(candidates []storedKey, affinityKey string) storedKey {
	if len(candidates) == 1 {
		return candidates[0]
	}
	var best storedKey
	bestScore := math.Inf(-1)
	for _, candidate := range candidates {
		weight := candidate.weight
		if weight <= 0 {
			weight = 1
		}
		score := rendezvousScore(affinityKey, string(candidate.id), weight)
		if score > bestScore || (score == bestScore && (best.id == "" || string(candidate.id) < string(best.id))) {
			bestScore = score
			best = candidate
		}
	}
	return best
}

// rendezvousScore implements weighted highest-random-weight: score = -ln(u) * weight, u in (0,1].
func rendezvousScore(affinityKey, keyID string, weight int) float64 {
	sum := sha256.Sum256([]byte(affinityKey + ":" + keyID))
	// Map 64-bit digest into (0,1] deterministically.
	u := binary.BigEndian.Uint64(sum[:8])
	// Avoid 0 so Log is defined; map full range onto (0,1].
	f := (float64(u) + 1.0) / (float64(^uint64(0)) + 1.0)
	return -math.Log(f) * float64(weight)
}

func headerValue(headers http.Header, name string) string {
	if headers == nil {
		return ""
	}
	return strings.TrimSpace(headers.Get(name))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
