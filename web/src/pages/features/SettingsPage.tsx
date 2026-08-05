import {
  ArrowsClockwise,
  ArrowRight,
  GearSix,
  Key,
  Lock,
  Stack,
  TerminalWindow,
  WarningCircle,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  ErrorState,
  HelpTip,
  PageHeader,
  SectionPanel,
  SecretInput,
  Skeleton,
  useToast,
} from "@/components";
import { api, ApiError, type SettingsDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { cn } from "@/lib/cn";
import { useI18n, type Translate } from "@/lib/i18n";

type LoadPolicy = "spread" | "sticky";
type AttemptCount = 2 | 3 | 4;

type InfoRow = {
  readonly label: string;
  readonly value: string;
  readonly tip: string;
  readonly badge?: boolean;
  readonly on?: boolean;
};

function clampAttempts(value: number | undefined | null): AttemptCount {
  const n = value ?? 2;
  if (n <= 2) return 2;
  if (n >= 4) return 4;
  return 3;
}

const DEFAULT_BENCH_MINUTES = 10;
const MIN_BENCH_MINUTES = 1;
const MAX_BENCH_MINUTES = 60;

function clampBenchMinutes(value: number | undefined | null): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return DEFAULT_BENCH_MINUTES;
  if (n < MIN_BENCH_MINUTES) return MIN_BENCH_MINUTES;
  if (n > MAX_BENCH_MINUTES) return MAX_BENCH_MINUTES;
  return Math.round(n);
}

function normalizePolicy(value: string | undefined | null): LoadPolicy {
  return value === "sticky" ? "sticky" : "spread";
}

function policyLabel(t: Translate, policy: LoadPolicy): string {
  return policy === "sticky"
    ? t("settings.loadPolicyStickyLabel")
    : t("settings.loadPolicySpreadLabel");
}

function serviceRows(t: Translate, s: SettingsDTO): InfoRow[] {
  return [
    {
      label: t("settings.listen"),
      value: s.listen,
      tip: t("settings.listenTip"),
    },
    {
      label: t("settings.dataDir"),
      value: s.data_dir,
      tip: t("settings.dataDirTip"),
    },
    {
      label: t("settings.zenBase"),
      value: s.zen_base,
      tip: t("settings.zenBaseTip"),
    },
    {
      label: t("settings.upstreamTimeout"),
      value: t("settings.upstreamTimeoutValue", { seconds: s.upstream_timeout_seconds }),
      tip: t("settings.upstreamTimeoutTip"),
    },
    {
      label: t("settings.httpProxy"),
      value: s.http_proxy_configured ? t("settings.configured") : t("settings.notConfigured"),
      tip: t("settings.httpProxyTip"),
      badge: true,
      on: s.http_proxy_configured,
    },
    {
      label: t("settings.httpsProxy"),
      value: s.https_proxy_configured ? t("settings.configured") : t("settings.notConfigured"),
      tip: t("settings.httpsProxyTip"),
      badge: true,
      on: s.https_proxy_configured,
    },
  ];
}

function modelRows(t: Translate, s: SettingsDTO): InfoRow[] {
  return [
    {
      label: t("settings.showAllModels"),
      value: s.show_all_models ? t("settings.on") : t("settings.off"),
      tip: t("settings.showAllModelsTip"),
      badge: true,
      on: s.show_all_models,
    },
    {
      label: t("settings.modelCacheTtl"),
      value: t("settings.modelCacheTtlValue", { seconds: s.model_cache_ttl_seconds }),
      tip: t("settings.modelCacheTtlTip"),
    },
    {
      label: t("settings.ocVersion"),
      value: s.oc_version,
      tip: t("settings.ocVersionTip"),
    },
    {
      label: t("settings.cookieSecure"),
      value: s.cookie_secure ? t("settings.on") : t("settings.off"),
      tip: t("settings.cookieSecureTip"),
      badge: true,
      on: s.cookie_secure,
    },
    {
      label: t("settings.sessionTtl"),
      value: t("settings.sessionTtlValue", { hours: s.session_ttl_hours }),
      tip: t("settings.sessionTtlTip"),
    },
  ];
}

function InfoList({ rows }: { readonly rows: readonly InfoRow[] }) {
  return (
    <div className="divide-y divide-border">
      {rows.map((row, index) => (
        <div
          key={row.label}
          className={cn(
            "flex items-center justify-between gap-4 px-4 py-3 sm:px-5",
            index % 2 === 1 && "bg-paper-2/35",
          )}
        >
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="text-[13px] font-medium text-ink">{row.label}</span>
            <HelpTip content={row.tip} label={row.label} />
          </div>
          {row.badge ? (
            <Badge kind={row.on ? "healthy" : "neutral"}>{row.value}</Badge>
          ) : (
            <span className="max-w-[55%] truncate text-right font-mono text-[12px] text-ink-muted">
              {row.value}
            </span>
          )}
        </div>
      ))}
    </div>
  );
}

/** Larger hard-edge segment control for primary settings (min 44px touch). */
function SettingSegmented({
  value,
  onChange,
  options,
  "aria-label": ariaLabel,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly options: ReadonlyArray<{ readonly value: string; readonly label: string }>;
  readonly "aria-label"?: string;
}) {
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className="inline-flex min-h-11 w-full max-w-xl flex-wrap items-stretch gap-0 overflow-hidden rounded-none border-2 border-border bg-paper-2 shadow-[3px_3px_0_var(--border)] sm:w-auto"
    >
      {options.map((opt, index) => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            aria-pressed={active}
            className={cn(
              "inline-flex min-h-11 min-w-[4.5rem] flex-1 items-center justify-center px-4 text-[13px] font-semibold transition-[background-color,color] duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus-ring",
              index > 0 && "border-l-2 border-border",
              active
                ? "bg-accent text-accent-fg"
                : "bg-paper-1 text-ink-muted hover:bg-paper-2 hover:text-ink",
            )}
            onClick={() => onChange(opt.value)}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

function FailoverChain({
  attempts,
  t,
}: {
  readonly attempts: AttemptCount;
  readonly t: Translate;
}) {
  const steps = Array.from({ length: attempts }, (_, i) => i + 1);
  return (
    <div
      className="flex flex-wrap items-center gap-1.5"
      aria-label={t("settings.poolAttemptsPath")}
    >
      {steps.map((n, index) => (
        <div key={n} className="inline-flex items-center gap-1.5">
          {index > 0 ? (
            <ArrowRight size={14} weight="bold" className="text-ink-faint" aria-hidden />
          ) : null}
          <span
            className={cn(
              "inline-flex min-h-9 items-center gap-1.5 rounded-none border-2 border-border px-2.5 py-1 text-[12px] font-semibold",
              index === 0
                ? "bg-accent-mint text-black shadow-[2px_2px_0_var(--border)]"
                : "bg-paper-2 text-ink shadow-[2px_2px_0_var(--border)]",
            )}
          >
            <span className="tabular-nums">{t("settings.poolAttemptStep", { n })}</span>
            <span className="text-[11px] font-medium text-ink-muted">
              {index === 0
                ? t("settings.poolAttemptFirst")
                : t("settings.poolAttemptFailover")}
            </span>
          </span>
        </div>
      ))}
    </div>
  );
}

export function SettingsPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const { t } = useI18n();
  const [settings, setSettings] = useState<SettingsDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [savingPassword, setSavingPassword] = useState(false);

  // Snapshot = last known server/runtime values; draft = editable form.
  const [livePolicy, setLivePolicy] = useState<LoadPolicy>("spread");
  const [liveAttempts, setLiveAttempts] = useState<AttemptCount>(2);
  const [liveBenchMinutes, setLiveBenchMinutes] = useState(DEFAULT_BENCH_MINUTES);
  const [draftPolicy, setDraftPolicy] = useState<LoadPolicy>("spread");
  const [draftAttempts, setDraftAttempts] = useState<AttemptCount>(2);
  const [draftBenchMinutes, setDraftBenchMinutes] = useState(DEFAULT_BENCH_MINUTES);
  const [savingPool, setSavingPool] = useState(false);

  const poolDirty =
    draftPolicy !== livePolicy ||
    draftAttempts !== liveAttempts ||
    draftBenchMinutes !== liveBenchMinutes;

  const effectiveSummary = useMemo(
    () =>
      t("settings.poolEffectiveSummary", {
        policy: `${livePolicy} · ${policyLabel(t, livePolicy)}`,
        attempts: liveAttempts,
        bench: liveBenchMinutes,
      }),
    [livePolicy, liveAttempts, liveBenchMinutes, t],
  );

  function applyPoolSnapshot(next: SettingsDTO) {
    const policy = normalizePolicy(next.load_policy);
    const attempts = clampAttempts(next.max_failover_attempts);
    const bench = clampBenchMinutes(next.bench_duration_minutes);
    setLivePolicy(policy);
    setLiveAttempts(attempts);
    setLiveBenchMinutes(bench);
    setDraftPolicy(policy);
    setDraftAttempts(attempts);
    setDraftBenchMinutes(bench);
  }

  function discardPoolDraft() {
    setDraftPolicy(livePolicy);
    setDraftAttempts(liveAttempts);
    setDraftBenchMinutes(liveBenchMinutes);
  }

  async function load() {
    setLoading(true);
    try {
      const next = await api.settings();
      setSettings(next);
      applyPoolSnapshot(next);
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onChangePassword(event: FormEvent) {
    event.preventDefault();
    if (newPassword.length < 8) {
      push(t("settings.errNewPasswordTooShort"), "error");
      return;
    }
    if (newPassword !== confirmPassword) {
      push(t("settings.errPasswordMismatch"), "error");
      return;
    }
    setSavingPassword(true);
    try {
      await api.changePassword(currentPassword, newPassword);
      // Backend revokes all sessions + clears cookie; force re-login.
      setSessionHint(false);
      push(t("settings.successPasswordUpdated"), "success");
      void navigate("/login", { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        push(t("settings.errCurrentPasswordWrong"), "error");
      } else {
        push(err instanceof Error ? err.message : t("settings.errUpdateFailed"), "error");
      }
    } finally {
      setSavingPassword(false);
    }
  }

  async function onSavePool(event: FormEvent) {
    event.preventDefault();
    if (!poolDirty) return;
    setSavingPool(true);
    try {
      const next = await api.patchSettings({
        load_policy: draftPolicy,
        max_failover_attempts: draftAttempts,
        bench_duration_minutes: draftBenchMinutes,
      });
      setSettings(next);
      applyPoolSnapshot(next);
      push(t("settings.poolSaved"), "success");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      // Keep draft on failure — do not pretend live values changed.
      push(err instanceof Error ? err.message : t("settings.errUpdateFailed"), "error");
    } finally {
      setSavingPool(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("settings.title")}
        actions={
          <Button variant="secondary" size="sm" onClick={() => void load()}>
            {t("common.refresh")}
          </Button>
        }
      />

      {loading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : null}
      {!loading && error ? <ErrorState title={t("common.loadFailed")} description={error} /> : null}

      {!loading && settings ? (
        <>
          <SectionPanel
            title={t("settings.authTitle")}
            description={
              settings.password_custom
                ? t("settings.authCustomDescription")
                : t("settings.authEnvDescription")
            }
            icon={Lock}
            iconTone="accent"
            bodyClassName="!p-4 sm:!p-5"
          >
            <form
              className="grid max-w-xl gap-3"
              onSubmit={(e) => void onChangePassword(e)}
            >
              <SecretInput
                label={t("settings.currentPassword")}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <SecretInput
                  label={t("settings.newPassword")}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
                <SecretInput
                  label={t("settings.confirmNewPassword")}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <p className="text-[12px] text-ink-faint">{t("settings.authHint")}</p>
                <Button type="submit" size="sm" loading={savingPassword}>
                  {t("settings.updatePassword")}
                </Button>
              </div>
            </form>
          </SectionPanel>

          <SectionPanel
            title={t("settings.poolTitle")}
            description={t("settings.poolDescription")}
            icon={ArrowsClockwise}
            iconTone="accent"
            bodyClassName="!p-4 sm:!p-5"
            actions={
              <Badge kind={poolDirty ? "warning" : "healthy"}>
                {poolDirty ? t("settings.poolDraftDirty") : t("settings.poolSynced")}
              </Badge>
            }
          >
            <form className="grid gap-4" onSubmit={(e) => void onSavePool(e)}>
              {/* Live runtime snapshot */}
              <div className="flex flex-col gap-2 rounded-none border-2 border-border bg-paper-2 p-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    <Badge kind="healthy">{t("settings.poolEffective")}</Badge>
                    <span className="font-mono text-[12px] text-ink-muted">
                      {livePolicy} · {liveAttempts}
                    </span>
                  </div>
                  <p className="text-[13px] font-medium text-ink">{effectiveSummary}</p>
                </div>
                {poolDirty ? (
                  <div className="shrink-0 text-[12px] text-ink-muted">
                    <span className="font-medium text-ink">{t("settings.poolDraftDirty")}: </span>
                    <span className="font-mono">
                      {draftPolicy} · {draftAttempts}
                    </span>
                  </div>
                ) : null}
              </div>

              {/* Runtime-only + scope notes */}
              <div className="grid gap-2">
                <div className="flex gap-2.5 rounded-none border-2 border-border bg-accent-yellow text-black shadow-[2px_2px_0_var(--border)] px-3 py-2.5">
                  <WarningCircle
                    size={18}
                    weight="fill"
                    className="mt-0.5 shrink-0 text-ink"
                    aria-hidden
                  />
                  <p className="text-[12px] leading-relaxed text-ink">
                    {t("settings.poolRuntimeNotice")}
                  </p>
                </div>
                <p className="border-l-4 border-border pl-3 text-[12px] leading-relaxed text-ink-muted">
                  {t("settings.poolScopeNote")}
                </p>
              </div>

              {/* Policy control */}
              <div className="grid gap-2">
                <div className="inline-flex items-center gap-1.5">
                  <span className="text-[13px] font-medium text-ink">
                    {t("settings.loadPolicy")}
                  </span>
                  <HelpTip
                    content={t("settings.loadPolicyTip")}
                    label={t("settings.loadPolicy")}
                  />
                </div>
                <SettingSegmented
                  aria-label={t("settings.loadPolicy")}
                  value={draftPolicy}
                  onChange={(v) => setDraftPolicy(v === "sticky" ? "sticky" : "spread")}
                  options={[
                    {
                      value: "spread",
                      label: `${t("settings.loadPolicySpread")} · ${t("settings.loadPolicySpreadLabel")}`,
                    },
                    {
                      value: "sticky",
                      label: `${t("settings.loadPolicySticky")} · ${t("settings.loadPolicyStickyLabel")}`,
                    },
                  ]}
                />
                <p className="text-[12px] leading-relaxed text-ink-muted">
                  {draftPolicy === "sticky"
                    ? t("settings.poolStickyPath")
                    : t("settings.poolSpreadPath")}
                </p>
              </div>

              {/* Attempts control + chain diagram */}
              <div className="grid gap-2">
                <div className="inline-flex items-center gap-1.5">
                  <span className="text-[13px] font-medium text-ink">
                    {t("settings.maxFailoverAttempts")}
                  </span>
                  <HelpTip
                    content={t("settings.maxFailoverAttemptsTip")}
                    label={t("settings.maxFailoverAttempts")}
                  />
                </div>
                <SettingSegmented
                  aria-label={t("settings.maxFailoverAttempts")}
                  value={String(draftAttempts)}
                  onChange={(v) => setDraftAttempts(clampAttempts(Number(v)))}
                  options={[
                    { value: "2", label: "2" },
                    { value: "3", label: "3" },
                    { value: "4", label: "4" },
                  ]}
                />
                <FailoverChain attempts={draftAttempts} t={t} />
                <p className="text-[12px] leading-relaxed text-ink-muted">
                  {t("settings.poolAttemptsPath")}
                </p>
              </div>

              {/* 401 bench isolation minutes */}
              <div className="grid gap-2">
                <div className="inline-flex items-center gap-1.5">
                  <span className="text-[13px] font-medium text-ink">
                    {t("settings.benchDurationMinutes")}
                  </span>
                  <HelpTip
                    content={t("settings.benchDurationMinutesTip")}
                    label={t("settings.benchDurationMinutes")}
                  />
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <SettingSegmented
                    aria-label={t("settings.benchDurationMinutes")}
                    value={
                      [5, 10, 15, 30].includes(draftBenchMinutes)
                        ? String(draftBenchMinutes)
                        : "custom"
                    }
                    onChange={(v) => {
                      if (v === "custom") return;
                      setDraftBenchMinutes(clampBenchMinutes(Number(v)));
                    }}
                    options={[
                      { value: "5", label: "5" },
                      { value: "10", label: "10" },
                      { value: "15", label: "15" },
                      { value: "30", label: "30" },
                      { value: "custom", label: "…" },
                    ]}
                  />
                  <label className="inline-flex items-center gap-1.5 text-[12px] text-ink-muted">
                    <span className="sr-only">{t("settings.benchDurationMinutes")}</span>
                    <input
                      type="number"
                      min={MIN_BENCH_MINUTES}
                      max={MAX_BENCH_MINUTES}
                      step={1}
                      value={draftBenchMinutes}
                      onChange={(event) =>
                        setDraftBenchMinutes(clampBenchMinutes(Number(event.target.value)))
                      }
                      className="w-20 border-2 border-border bg-paper px-2 py-1.5 font-mono text-[13px] text-ink shadow-[2px_2px_0_var(--border)] outline-none focus:border-ink"
                    />
                    <span>min</span>
                  </label>
                </div>
                <p className="text-[12px] leading-relaxed text-ink-muted">
                  {t("settings.benchDurationPath")}
                </p>
              </div>

              {/* Key pool deep-link */}
              <div className="flex flex-col gap-2 rounded-none border-2 border-border bg-paper-2 p-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-start gap-2.5">
                  <span
                    className="mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center border-2 border-border bg-accent-teal text-black shadow-[2px_2px_0_var(--border)]"
                    aria-hidden
                  >
                    <Key size={16} weight="duotone" />
                  </span>
                  <p className="text-[12px] leading-relaxed text-ink-muted">
                    {t("settings.poolGoKeyPoolTip")}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                  onClick={() => void navigate("/app/key-pool")}
                >
                  {t("settings.poolGoKeyPool")}
                  <ArrowRight size={14} weight="bold" aria-hidden />
                </Button>
              </div>

              {/* Actions: save only when dirty */}
              <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
                {poolDirty ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={savingPool}
                    onClick={discardPoolDraft}
                  >
                    {t("settings.poolDiscard")}
                  </Button>
                ) : null}
                <Button
                  type="submit"
                  size="md"
                  loading={savingPool}
                  disabled={!poolDirty || savingPool}
                >
                  {t("settings.savePool")}
                </Button>
              </div>
            </form>
          </SectionPanel>

          <SectionPanel
            title={t("settings.serviceTitle")}
            icon={GearSix}
            iconTone="yellow"
            bodyClassName="p-0"
          >
            <InfoList rows={serviceRows(t, settings)} />
          </SectionPanel>

          <SectionPanel
            title={t("settings.modelTitle")}
            icon={Stack}
            iconTone="teal"
            bodyClassName="p-0"
          >
            <InfoList rows={modelRows(t, settings)} />
          </SectionPanel>

          <SectionPanel
            title={t("settings.envTitle")}
            icon={TerminalWindow}
            iconTone="mint"
            bodyClassName="!p-4 sm:!p-5"
          >
            <div className="grid gap-2 sm:grid-cols-2">
              {(
                [
                  ["ADMIN_PASSWORD", t("settings.envAdminPassword")],
                  ["ADMIN_SECRET", t("settings.envAdminSecret")],
                  ["LISTEN", t("settings.listen")],
                  ["DATA_DIR", t("settings.dataDir")],
                  ["ZEN_BASE", t("settings.envZenBase")],
                  ["SHOW_ALL_MODELS", t("settings.envShowAllModels")],
                  ["COOKIE_SECURE", t("settings.envCookieSecure")],
                  ["MODEL_CACHE_TTL", t("settings.envModelCacheTtl")],
                  ["ZEN_LOAD_POLICY", t("settings.envZenLoadPolicy")],
                  ["ZEN_MAX_ATTEMPTS", t("settings.envZenMaxAttempts")],
                  ["ZEN_BENCH_MINUTES", t("settings.envZenBenchMinutes")],
                ] as const
              ).map(([env, tip]) => (
                <div
                  key={env}
                  className="flex items-center justify-between gap-2 rounded-none border border-border bg-paper-2 px-3 py-2"
                >
                  <code className="font-mono text-[12px] text-ink">{env}</code>
                  <HelpTip content={tip} label={env} />
                </div>
              ))}
            </div>
          </SectionPanel>
        </>
      ) : null}
    </div>
  );
}
