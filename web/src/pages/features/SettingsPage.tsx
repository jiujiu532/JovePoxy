import { GearSix, Lock, Stack, TerminalWindow, ArrowsClockwise } from "@phosphor-icons/react";
import { useEffect, useState, type FormEvent } from "react";
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

type InfoRow = {
  readonly label: string;
  readonly value: string;
  readonly tip: string;
  readonly badge?: boolean;
  readonly on?: boolean;
};

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
            index % 2 === 1 && "bg-paper-0/35",
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
  const [loadPolicy, setLoadPolicy] = useState<"spread" | "sticky">("spread");
  const [maxAttempts, setMaxAttempts] = useState(2);
  const [savingPool, setSavingPool] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const next = await api.settings();
      setSettings(next);
      const policy = next.load_policy === "sticky" ? "sticky" : "spread";
      setLoadPolicy(policy);
      const attempts = next.max_failover_attempts ?? 2;
      setMaxAttempts(attempts < 2 ? 2 : attempts > 4 ? 4 : attempts);
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
    setSavingPool(true);
    try {
      const next = await api.patchSettings({
        load_policy: loadPolicy,
        max_failover_attempts: maxAttempts,
      });
      setSettings(next);
      setLoadPolicy(next.load_policy === "sticky" ? "sticky" : "spread");
      const attempts = next.max_failover_attempts ?? maxAttempts;
      setMaxAttempts(attempts < 2 ? 2 : attempts > 4 ? 4 : attempts);
      push(t("settings.poolSaved"), "success");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
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
                <p className="text-[12px] text-ink-faint">
                  {t("settings.authHint")}
                </p>
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
          >
            <form
              className="grid max-w-xl gap-3"
              onSubmit={(e) => void onSavePool(e)}
            >
              <label className="grid gap-1.5">
                <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink">
                  {t("settings.loadPolicy")}
                  <HelpTip content={t("settings.loadPolicyTip")} label={t("settings.loadPolicy")} />
                </span>
                <select
                  className="h-9 rounded-none border border-border bg-paper-0 px-2 text-[13px] text-ink"
                  value={loadPolicy}
                  onChange={(e) => setLoadPolicy(e.target.value === "sticky" ? "sticky" : "spread")}
                >
                  <option value="spread">{t("settings.loadPolicySpread")}</option>
                  <option value="sticky">{t("settings.loadPolicySticky")}</option>
                </select>
              </label>
              <label className="grid gap-1.5">
                <span className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink">
                  {t("settings.maxFailoverAttempts")}
                  <HelpTip
                    content={t("settings.maxFailoverAttemptsTip")}
                    label={t("settings.maxFailoverAttempts")}
                  />
                </span>
                <select
                  className="h-9 rounded-none border border-border bg-paper-0 px-2 text-[13px] text-ink"
                  value={String(maxAttempts)}
                  onChange={(e) => setMaxAttempts(Number(e.target.value))}
                >
                  <option value="2">2</option>
                  <option value="3">3</option>
                  <option value="4">4</option>
                </select>
              </label>
              <div className="flex justify-end pt-1">
                <Button type="submit" size="sm" loading={savingPool}>
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
                ] as const
              ).map(([env, tip]) => (
                <div
                  key={env}
                  className="flex items-center justify-between gap-2 rounded-none border border-border bg-paper-0 px-3 py-2"
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
