/** OpenAI-compatible base path clients should use against this gateway. */
export function gatewayOpenAIBaseURL(origin?: string): string {
  const raw =
    origin ??
    (typeof window !== "undefined" && window.location?.origin
      ? window.location.origin
      : "");
  const base = raw.replace(/\/+$/, "");
  return base ? `${base}/v1` : "/v1";
}
