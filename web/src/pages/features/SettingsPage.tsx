import {
  ArrowsClockwise,
  ArrowRight,
  Key,
  Lock,
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

/** Compact hard-edge segment control for dense settings. */
function SettingSegmented({
  value,
  onChange,
  options,
  "aria-label": ariaLabel,
  size = "md",
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly options: ReadonlyArray<{ readonly value: string; readonly label: string }>;
  readonly "aria-label"?: string;
  readonly size?: "sm" | "md";
}) {
  const tall = size === "md";
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={cn(
        "inline-flex w-full max-w-xl flex-wrap items-stretch gap-0 overflow-hidden rounded-none border-2 border-border bg-paper-2 shadow-[2px_2px_0_var(--border)] sm:w-auto",
        tall ? "min-h-10" : "min-h-9",
      )}
    >
      {options.map((opt, index) => {
        const active = value === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            aria-pressed={active}
            className={cn(
              "inline-flex flex-1 items-center justify-center font-semibold transition-[background-color,color] duration-150",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus-ring",
              tall
                ? "min-h-10 min-w-[3.75rem] px-3 text-[13px]"
                : "min-h-9 min-w-[2.75rem] px-2.5 text-[12px]",
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
      className="flex flex-wrap items-center gap-1"
      aria-label={t("settings.poolAttemptsPath")}
    >
      {steps.map((n, index) => (
        <div key={n} className="inline-flex items-center gap-1">
          {index > 0 ? (
            <ArrowRight size={12} weight="bold" className="text-ink-faint" aria-hidden />
          ) : null}
          <span
            className={cn(
              "inline-flex items-center gap-1 rounded-none border border-border px-2 py-0.5 text-[11px] font-semibold",
              index === 0
                ? "bg-accent-mint text-black"
                : "bg-paper-2 text-ink",
            )}
          >
            <span className="tabular-nums">{t("settings.poolAttemptStep", { n })}</span>
            <span className="font-medium text-ink-muted">
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
  const [livePaidUseProxy, setLivePaidUseProxy] = useState(false);
  const [draftPolicy, setDraftPolicy] = useState<LoadPolicy>("spread");
  const [draftAttempts, setDraftAttempts] = useState<AttemptCount>(2);
  const [draftBenchMinutes, setDraftBenchMinutes] = useState(DEFAULT_BENCH_MINUTES);
  const [draftPaidUseProxy, setDraftPaidUseProxy] = useState(false);
  const [savingPool, setSavingPool] = useState(false);

  const poolDirty =
    draftPolicy !== livePolicy ||
    draftAttempts !== liveAttempts ||
    draftBenchMinutes !== liveBenchMinutes ||
    draftPaidUseProxy !== livePaidUseProxy;

  const effectiveSummary = useMemo(
    () =>
      t("settings.poolEffectiveSummary", {
        policy: `${livePolicy} · ${policyLabel(t, livePolicy)}`,
        attempts: liveAttempts,
        bench: liveBenchMinutes,
        proxy: livePaidUseProxy ? t("settings.on") : t("settings.off"),
      }),
    [livePolicy, liveAttempts, liveBenchMinutes, livePaidUseProxy, t],
  );

  const runtimeChip = useMemo(
    () =>
      `${livePolicy} · ${liveAttempts} · ${liveBenchMinutes}m · proxy:${livePaidUseProxy ? "on" : "off"}`,
    [livePolicy, liveAttempts, liveBenchMinutes, livePaidUseProxy],
  );

  function applyPoolSnapshot(next: SettingsDTO) {
    const policy = normalizePolicy(next.load_policy);
    const attempts = clampAttempts(next.max_failover_attempts);
    const bench = clampBenchMinutes(next.bench_duration_minutes);
    const paidProxy = next.paid_use_proxy_pool === true;
    setLivePolicy(policy);
    setLiveAttempts(attempts);
    setLiveBenchMinutes(bench);
    setLivePaidUseProxy(paidProxy);
    setDraftPolicy(policy);
    setDraftAttempts(attempts);
    setDraftBenchMinutes(bench);
    setDraftPaidUseProxy(paidProxy);
  }

  function discardPoolDraft() {
    setDraftPolicy(livePolicy);
    setDraftAttempts(liveAttempts);
    setDraftBenchMinutes(liveBenchMinutes);
    setDraftPaidUseProxy(livePaidUseProxy);
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
        paid_use_proxy_pool: draftPaidUseProxy,
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
    <div className="flex flex-col gap-3">
      <PageHeader
        title={t("settings.title")}
        actions={
          <Button variant="secondary" size="sm" onClick={() => void load()}>
            {t("common.refresh")}
          </Button>
        }
      />

      {loading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-36 w-full" />
          <Skeleton className="h-28 w-full" />
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
            bodyClassName="!p-3 sm:!p-4"
          >
            <form
              className="grid max-w-xl gap-2.5"
              onSubmit={(e) => void onChangePassword(e)}
            >
              <SecretInput
                label={t("settings.currentPassword")}
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
              <div className="grid gap-2.5 sm:grid-cols-2">
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
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-[11px] text-ink-faint">{t("settings.authHint")}</p>
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
            bodyClassName="!p-3 sm:!p-4"
            actions={
              <div className="flex flex-wrap items-center gap-1.5">
                <Badge kind={poolDirty ? "warning" : "healthy"}>
                  {poolDirty ? t("settings.poolDraftDirty") : t("settings.poolSynced")}
                </Badge>
                <code className="border border-border bg-paper-2 px-1.5 py-0.5 font-mono text-[11px] text-ink">
                  {runtimeChip}
                </code>
              </div>
            }
          >
            <form className="grid gap-3" onSubmit={(e) => void onSavePool(e)}>
              <div className="flex flex-wrap items-start justify-between gap-2 border-2 border-border bg-paper-2 px-2.5 py-2">
                <div className="min-w-0">
                  <div className="mb-0.5 flex flex-wrap items-center gap-1.5">
                    <Badge kind="healthy">{t("settings.poolEffective")}</Badge>
                    <span className="text-[12px] font-medium text-ink">{effectiveSummary}</span>
                  </div>
                  <p className="text-[11px] leading-snug text-ink-muted">
                    {t("settings.poolRuntimeNoticeShort")}
                  </p>
                </div>
                {poolDirty ? (
                  <span className="shrink-0 font-mono text-[11px] text-ink-muted">
                    {t("settings.poolDraftDirty")}: {draftPolicy} · {draftAttempts} ·{" "}
                    {draftBenchMinutes}m · proxy:{draftPaidUseProxy ? "on" : "off"}
                  </span>
                ) : null}
              </div>

              <div className="grid gap-3 lg:grid-cols-2 xl:grid-cols-4">
                <div className="grid gap-1.5">
                  <div className="inline-flex items-center gap-1">
                    <span className="text-[12px] font-medium text-ink">
                      {t("settings.loadPolicy")}
                    </span>
                    <HelpTip
                      content={`${t("settings.loadPolicyTip")} ${t("settings.poolScopeNote")}`}
                      label={t("settings.loadPolicy")}
                    />
                  </div>
                  <SettingSegmented
                    size="sm"
                    aria-label={t("settings.loadPolicy")}
                    value={draftPolicy}
                    onChange={(v) => setDraftPolicy(v === "sticky" ? "sticky" : "spread")}
                    options={[
                      {
                        value: "spread",
                        label: t("settings.loadPolicySpreadLabel"),
                      },
                      {
                        value: "sticky",
                        label: t("settings.loadPolicyStickyLabel"),
                      },
                    ]}
                  />
                  <p className="text-[11px] leading-snug text-ink-muted">
                    {draftPolicy === "sticky"
                      ? t("settings.poolStickyPath")
                      : t("settings.poolSpreadPath")}
                  </p>
                </div>

                <div className="grid gap-1.5">
                  <div className="inline-flex items-center gap-1">
                    <span className="text-[12px] font-medium text-ink">
                      {t("settings.maxFailoverAttempts")}
                    </span>
                    <HelpTip
                      content={t("settings.maxFailoverAttemptsTip")}
                      label={t("settings.maxFailoverAttempts")}
                    />
                  </div>
                  <SettingSegmented
                    size="sm"
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
                </div>

                <div className="grid gap-1.5">
                  <div className="inline-flex items-center gap-1">
                    <span className="text-[12px] font-medium text-ink">
                      {t("settings.benchDurationMinutes")}
                    </span>
                    <HelpTip
                      content={t("settings.benchDurationMinutesTip")}
                      label={t("settings.benchDurationMinutes")}
                    />
                  </div>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <SettingSegmented
                      size="sm"
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
                      ]}
                    />
                    <label className="inline-flex items-center gap-1 text-[11px] text-ink-muted">
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
                        className="w-14 border-2 border-border bg-paper px-1.5 py-1 font-mono text-[12px] text-ink outline-none focus:border-ink"
                      />
                      <span>min</span>
                    </label>
                  </div>
                  <p className="text-[11px] leading-snug text-ink-muted">
                    {t("settings.benchDurationPath")}
                  </p>
                </div>

                <div className="grid gap-1.5">
                  <div className="inline-flex items-center gap-1">
                    <span className="text-[12px] font-medium text-ink">
                      {t("settings.paidUseProxyPool")}
                    </span>
                    <HelpTip
                      content={t("settings.paidUseProxyPoolTip")}
                      label={t("settings.paidUseProxyPool")}
                    />
                  </div>
                  <SettingSegmented
                    size="sm"
                    aria-label={t("settings.paidUseProxyPool")}
                    value={draftPaidUseProxy ? "on" : "off"}
                    onChange={(v) => setDraftPaidUseProxy(v === "on")}
                    options={[
                      { value: "off", label: t("settings.off") },
                      { value: "on", label: t("settings.on") },
                    ]}
                  />
                  <p className="text-[11px] leading-snug text-ink-muted">
                    {t("settings.paidUseProxyPoolPath")}
                  </p>
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-2 border-t-2 border-border pt-2.5">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="!px-2"
                  onClick={() => void navigate("/app/key-pool")}
                >
                  <Key size={14} weight="duotone" aria-hidden />
                  {t("settings.poolGoKeyPool")}
                  <ArrowRight size={12} weight="bold" aria-hidden />
                </Button>
                <div className="flex flex-wrap items-center gap-1.5">
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
                    size="sm"
                    loading={savingPool}
                    disabled={!poolDirty || savingPool}
                  >
                    {t("settings.savePool")}
                  </Button>
                </div>
              </div>
            </form>
          </SectionPanel>
        </>
      ) : null}
    </div>
  );
}
