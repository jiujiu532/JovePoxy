import { useCallback, useState } from "react";
import { Badge, Button, Dialog } from "@/components";
import { api, type VersionInfoDTO } from "@/lib/api";
import { APP_VERSION } from "@/lib/version";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/cn";

function formatCheckedAt(value: string | undefined, lang: "zh" | "en"): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString(lang === "zh" ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

/**
 * Full-width bar (not pill) for sidebar footer. Opens remote version dialog.
 */
export function VersionBadge({ className }: { readonly className?: string }) {
  const { t, lang } = useI18n();
  const [open, setOpen] = useState(false);
  const [info, setInfo] = useState<VersionInfoDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (refresh: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.version(refresh);
        setInfo(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("version.checkFailed"));
        setInfo({
          current: APP_VERSION,
          latest: APP_VERSION,
          update_available: false,
          image: "jovepoxy",
          checked_at: new Date().toISOString(),
          source: "local",
          note: t("version.offlineNote"),
        });
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  async function openDialog() {
    setOpen(true);
    await load(false);
  }

  const current = info?.current ?? APP_VERSION;
  const latest = info?.latest ?? current;
  const update = Boolean(info?.update_available);

  return (
    <>
      <button
        type="button"
        onClick={() => void openDialog()}
        className={cn(
          "flex h-8 w-full items-center justify-center rounded-none border-2 border-border bg-paper-0",
          "font-mono text-[11px] tabular-nums tracking-wide text-ink-muted",
          "transition-colors hover:bg-paper-1 hover:text-ink",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
          className,
        )}
        aria-label={t("version.badgeAria", { version: APP_VERSION })}
      >
        v{APP_VERSION}
      </button>

      <Dialog open={open} title={t("version.title")} onClose={() => setOpen(false)}>
        <div className="flex flex-col gap-3.5">
          <div className="grid gap-3">
            <div>
              <p className="text-[12px] text-ink-muted">{t("version.current")}</p>
              <p className="mt-0.5 font-mono text-[18px] font-semibold tracking-tight text-ink">
                v{current}
              </p>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">{t("version.latest")}</p>
              <div className="mt-0.5 flex flex-wrap items-center gap-2">
                <p className="font-mono text-[18px] font-semibold tracking-tight text-ink">
                  v{latest}
                </p>
                {update ? <Badge kind="healthy">{t("version.updatable")}</Badge> : null}
                {!update && info && !error ? (
                  <Badge kind="neutral">{t("version.upToDate")}</Badge>
                ) : null}
              </div>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">{t("version.image")}</p>
              <p className="mt-0.5 break-all font-mono text-[13px] text-ink">
                {info?.image ?? "jovepoxy"}
              </p>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">{t("version.checkedAt")}</p>
              <p className="mt-0.5 text-[13px] text-ink">
                {formatCheckedAt(info?.checked_at, lang)}
              </p>
            </div>
          </div>

          {info?.note ? (
            <p className="rounded-none border-2 border-border bg-paper-0 px-3 py-2 text-[12px] leading-relaxed text-ink-muted">
              {info.note}
            </p>
          ) : null}
          {error ? (
            <p className="text-[12px] text-status-error">{error}</p>
          ) : null}

          <p className="text-[11px] leading-relaxed text-ink-faint">
            {t("version.hint")}
          </p>

          <div className="flex justify-end gap-2 pt-1">
            <Button
              variant="secondary"
              size="sm"
              loading={loading}
              onClick={() => void load(true)}
            >
              {t("version.checkNow")}
            </Button>
            <Button size="sm" onClick={() => setOpen(false)}>
              {t("dialog.close")}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
