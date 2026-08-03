import type { Translate } from "./i18n";
import type { ZenKeyDTO, ZenKeyStatus } from "./api";

/**
 * Parse API timestamps (often fixed-width RFC3339Nano UTC with 9 fractional digits).
 * JS Date commonly only accepts ms precision — truncate excess fraction before parse.
 */
export function parseApiTime(iso: string): Date | null {
  const raw = iso.trim();
  if (!raw) return null;
  // 2026-08-03T11:10:33.678043900Z → 2026-08-03T11:10:33.678Z
  const normalized = raw.replace(/(\.\d{3})\d+(Z|[+-]\d{2}:?\d{2})$/i, "$1$2");
  const ms = Date.parse(normalized);
  if (!Number.isFinite(ms)) return null;
  return new Date(ms);
}

/**
 * Human-readable local wall time for log/usage tables.
 * e.g. "2026-08-03 19:10:33". Falls back to the raw string when unparseable.
 */
export function formatDateTime(iso: string): string {
  const d = parseApiTime(iso);
  if (!d) {
    const trimmed = iso.trim();
    return trimmed.length > 0 ? trimmed : "-";
  }
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${y}-${m}-${day} ${hh}:${mm}:${ss}`;
}

/** Format a model id for display tables. */
export function formatModelId(id: string): string {
  const trimmed = id.trim();
  if (trimmed.length === 0) {
    return "-";
  }
  return trimmed;
}

/** Mask secret for list views: keep prefix, hide rest. */
export function maskSecret(value: string, visiblePrefix = 8): string {
  const trimmed = value.trim();
  if (trimmed.length === 0) {
    return "";
  }
  if (trimmed.length <= visiblePrefix) {
    return "*".repeat(trimmed.length);
  }
  return `${trimmed.slice(0, visiblePrefix)}${"*".repeat(Math.min(12, trimmed.length - visiblePrefix))}`;
}

/** Validate non-empty admin password field (client-side). */
export function validatePasswordInput(password: string, t: Translate): string | null {
  if (password.trim().length === 0) {
    return t("format.passwordRequired");
  }
  if (password.length < 4) {
    return t("format.passwordTooShort");
  }
  return null;
}

/** Derive zen key status when server field is missing (client fallback). */
export function zenKeyStatus(key: Pick<ZenKeyDTO, "enabled" | "status" | "cooldown_until">, nowMs = Date.now()): ZenKeyStatus {
  if (!key.enabled) return "disabled";
  // Process-memory 401 bench is server-authoritative (no client until timestamp).
  if (key.status === "benched") return "benched";
  if (key.status === "active" || key.status === "cooling" || key.status === "disabled") {
    // Re-check cooling against client clock for countdown UX.
    if (key.status === "cooling" || key.cooldown_until) {
      const until = key.cooldown_until ? Date.parse(key.cooldown_until) : NaN;
      if (!key.enabled) return "disabled";
      if (Number.isFinite(until) && until > nowMs) return "cooling";
      return key.enabled ? "active" : "disabled";
    }
    return key.status;
  }
  if (key.cooldown_until) {
    const until = Date.parse(key.cooldown_until);
    if (Number.isFinite(until) && until > nowMs) return "cooling";
  }
  return "active";
}

/** Format traffic share percentage (one decimal when needed). */
export function formatTrafficPct(pct: number | undefined | null): string {
  if (pct == null || Number.isNaN(pct) || pct <= 0) return "0%";
  const rounded = Math.round(pct * 10) / 10;
  if (Number.isInteger(rounded)) return `${rounded}%`;
  return `${rounded.toFixed(1)}%`;
}

/**
 * Format remaining cooldown for list cells.
 * Prefers server cooldown_remaining_sec when positive; else derives from until.
 */
export function formatCooldownRemaining(
  key: Pick<ZenKeyDTO, "cooldown_until" | "cooldown_remaining_sec">,
  nowMs = Date.now(),
): string | null {
  let remainingSec = 0;
  if (key.cooldown_until) {
    const until = Date.parse(key.cooldown_until);
    if (Number.isFinite(until) && until > nowMs) {
      remainingSec = Math.max(0, Math.ceil((until - nowMs) / 1000));
    }
  } else if (typeof key.cooldown_remaining_sec === "number" && key.cooldown_remaining_sec > 0) {
    remainingSec = Math.floor(key.cooldown_remaining_sec);
  }
  if (remainingSec <= 0) return null;
  if (remainingSec < 60) return `${remainingSec}s`;
  const minutes = Math.floor(remainingSec / 60);
  const seconds = remainingSec % 60;
  if (minutes < 60) {
    return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  const remMin = minutes % 60;
  return remMin > 0 ? `${hours}h ${remMin}m` : `${hours}h`;
}

