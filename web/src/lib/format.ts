import type { Translate } from "./i18n";

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
