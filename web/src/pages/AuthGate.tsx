import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { Button, ErrorState, Skeleton } from "@/components";
import { api, ApiError } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { useI18n } from "@/lib/i18n";

type GateState = "loading" | "authed" | "guest" | "error";

export function AuthGate({ children }: { readonly children: ReactNode }) {
  const { t } = useI18n();
  const [state, setState] = useState<GateState>("loading");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [retryToken, setRetryToken] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setState("loading");
    setErrorMessage(null);
    void (async () => {
      try {
        await api.me();
        if (!cancelled) {
          setSessionHint(true);
          setState("authed");
        }
      } catch (err) {
        if (cancelled) return;
        // Only 401 means unauthenticated → clear hint and force login.
        if (err instanceof ApiError && err.status === 401) {
          setSessionHint(false);
          setState("guest");
          return;
        }
        // Network / 5xx / other: keep any existing session hint and show retry UI.
        const message =
          err instanceof Error && err.message
            ? err.message
            : t("authGate.loadFailedDesc");
        setErrorMessage(message);
        setState("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [retryToken, t]);

  if (state === "loading") {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-paper-0 p-6">
        <Skeleton className="h-12 w-48" />
      </div>
    );
  }
  if (state === "guest") {
    return <Navigate to="/login" replace />;
  }
  if (state === "error") {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-paper-0 p-6">
        <ErrorState
          title={t("authGate.loadFailed")}
          description={errorMessage ?? t("authGate.loadFailedDesc")}
          action={
            <Button
              variant="secondary"
              onClick={() => setRetryToken((n) => n + 1)}
            >
              {t("common.retry")}
            </Button>
          }
        />
      </div>
    );
  }
  return children;
}
