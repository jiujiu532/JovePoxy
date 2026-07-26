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
export function validatePasswordInput(password: string): string | null {
  if (password.trim().length === 0) {
    return "请输入管理员密码";
  }
  if (password.length < 4) {
    return "密码至少 4 位";
  }
  return null;
}
