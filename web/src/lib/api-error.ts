import { ApiError } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import type { MessageKey, Translate } from "@/lib/i18n";

/** Page-scoped i18n keys for session / network errors. */
export type FriendlyErrorMessages = {
  readonly sessionExpired: MessageKey;
  readonly connectFailed: MessageKey;
};

/**
 * Map unknown API / network errors to a user-facing string.
 * Session (401) and connect failures use `messages` so each page keeps its i18n keys.
 */
export function friendlyError(
  err: unknown,
  fallback: string,
  t: Translate,
  messages: FriendlyErrorMessages,
): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return t(messages.sessionExpired);
    return err.message || fallback;
  }
  if (err instanceof TypeError) return t(messages.connectFailed);
  if (err instanceof Error) {
    if (/failed to fetch|networkerror|load failed/i.test(err.message)) {
      return t(messages.connectFailed);
    }
    return err.message || fallback;
  }
  return fallback;
}

/**
 * Bind page-scoped message keys; returns the `(err, fallback, t)` helper used by feature pages.
 */
export function bindFriendlyError(
  messages: FriendlyErrorMessages,
): (err: unknown, fallback: string, t: Translate) => string {
  return (err, fallback, t) => friendlyError(err, fallback, t, messages);
}

/**
 * Unified 401 handling: clear client session hint and send user to login.
 * Returns true when the error was a 401 (caller should stop other error UI).
 */
export function handleUnauthorized(
  err: unknown,
  navigate: (to: string) => void,
): boolean {
  if (err instanceof ApiError && err.status === 401) {
    setSessionHint(false);
    navigate("/login");
    return true;
  }
  return false;
}
