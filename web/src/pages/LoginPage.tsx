import { useEffect, useState, type FormEvent } from "react";
import { ArrowRight } from "@phosphor-icons/react";
import { Navigate, useNavigate } from "react-router-dom";
import { BrandMark, SecretInput, Skeleton } from "@/components";
import { ApiError, api } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { validatePasswordInput } from "@/lib/format";
import { APP_VERSION } from "@/lib/version";
import { cn } from "@/lib/cn";

/**
 * JovePoxy login — Neo-Brutalist hard card.
 * Black frame, hard shadow, Joyful Press CTA.
 */
export function LoginPage() {
  const navigate = useNavigate();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [alreadyAuthed, setAlreadyAuthed] = useState(false);
  const [entered, setEntered] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await api.me();
        if (!cancelled) {
          setSessionHint(true);
          setAlreadyAuthed(true);
        }
      } catch {
        if (!cancelled) {
          setSessionHint(false);
          setAlreadyAuthed(false);
        }
      } finally {
        if (!cancelled) setChecking(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (checking || alreadyAuthed) return;
    const id = window.requestAnimationFrame(() => setEntered(true));
    return () => window.cancelAnimationFrame(id);
  }, [checking, alreadyAuthed]);

  if (checking) {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center bg-paper-0">
        <Skeleton className="h-44 w-[min(92vw,420px)] rounded-none border-2 border-border" />
      </div>
    );
  }
  if (alreadyAuthed) {
    return <Navigate to="/app/overview" replace />;
  }

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const validation = validatePasswordInput(password);
    if (validation) {
      setError(validation);
      return;
    }
    setError(undefined);
    setLoading(true);
    try {
      await api.login(password);
      setSessionHint(true);
      void navigate("/app/overview");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex min-h-[100dvh] items-center justify-center overflow-hidden bg-paper-0 px-4 py-12 sm:px-6">
      <div
        className={cn(
          "relative z-[1] w-full max-w-[400px]",
          "transition-[opacity,transform] duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)]",
          entered ? "translate-y-0 opacity-100" : "translate-y-6 opacity-0",
        )}
      >
        {/* Hard-edge card */}
        <div
          className={cn(
            "border-4 border-border bg-paper-1",
            "shadow-[6px_6px_0_var(--border)]",
          )}
        >
          {/* Accent top bar (solid, no gradient) */}
          <div className="h-3 w-full bg-accent" aria-hidden />

          <div className="relative px-7 pb-7 pt-8 sm:px-8 sm:pb-8 sm:pt-9">
            {/* Brand */}
            <div className="flex items-center gap-3">
              <BrandMark size={42} className="rounded-none" />
              <div className="min-w-0">
                <p className="text-[15px] font-semibold tracking-tight text-ink">
                  JovePoxy
                </p>
                <p className="mt-0.5 text-[12px] text-ink-faint">
                  Gateway Console
                </p>
              </div>
            </div>

            {/* Title */}
            <div className="mt-7">
              <h1 className="text-[1.55rem] font-semibold tracking-tight text-ink">
                登录控制台
              </h1>
              <p className="mt-1.5 text-[13px] leading-relaxed text-ink-muted">
                使用管理员密码进入。
              </p>
            </div>

            {/* Form */}
            <form
              className="mt-6 flex flex-col gap-4"
              onSubmit={onSubmit}
              noValidate
            >
              <SecretInput
                label="管理员密码"
                name="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  if (error) setError(undefined);
                }}
                {...(error ? { error } : {})}
                disabled={loading}
                autoFocus
                autoComplete="current-password"
                placeholder="输入密码"
                inputClassName={cn(
                  "h-11 rounded-none border-2 border-border bg-paper-2",
                  "focus-visible:ring-2 focus-visible:ring-focus-ring focus-visible:ring-offset-2",
                )}
              />

              <button
                type="submit"
                disabled={loading}
                aria-busy={loading || undefined}
                className={cn(
                  "group relative flex h-12 w-full items-center justify-between gap-3",
                  "rounded-none border-4 border-border bg-accent pl-5 pr-2 text-[14px] font-semibold text-accent-fg",
                  "shadow-[var(--shadow-hard)]",
                  "transition-[transform,box-shadow,background-color] duration-150",
                  "ease-[cubic-bezier(0.34,1.56,0.64,1)]",
                  "hover:bg-accent-hover",
                  "active:translate-x-[4px] active:translate-y-[4px] active:scale-95 active:shadow-none",
                  "disabled:pointer-events-none disabled:opacity-55",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring focus-visible:ring-offset-2 focus-visible:ring-offset-paper-1",
                )}
              >
                <span className="truncate">
                  {loading ? "正在进入…" : "进入控制台"}
                </span>
                <span
                  className={cn(
                    "flex h-8 w-8 shrink-0 items-center justify-center",
                    "border-2 border-border bg-paper-2 text-ink",
                  )}
                  aria-hidden
                >
                  <ArrowRight size={16} weight="bold" />
                </span>
              </button>
            </form>

            {/* Quiet foot */}
            <div className="mt-6 flex items-center justify-between gap-3 border-t-2 border-border pt-4">
              <p className="text-[11px] text-ink-faint">本地网关 · 会话 Cookie</p>
              <p className="font-mono text-[11px] tabular-nums text-ink-faint">
                v{APP_VERSION}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
