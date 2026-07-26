import { useEffect, useState, type ReactNode } from "react";
import { Navigate } from "react-router-dom";
import { Skeleton } from "@/components";
import { api, ApiError } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";

type GateState = "loading" | "authed" | "guest";

export function AuthGate({ children }: { readonly children: ReactNode }) {
  const [state, setState] = useState<GateState>("loading");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await api.me();
        if (!cancelled) {
          setSessionHint(true);
          setState("authed");
        }
      } catch (err) {
        if (!cancelled) {
          setSessionHint(false);
          setState(err instanceof ApiError && err.status === 401 ? "guest" : "guest");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

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
  return children;
}
