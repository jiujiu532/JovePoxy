import { useCallback, useState } from "react";
import { Badge, Button, Dialog } from "@/components";
import { api, type VersionInfoDTO } from "@/lib/api";
import { APP_VERSION } from "@/lib/version";
import { cn } from "@/lib/cn";

function formatCheckedAt(value?: string): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString("zh-CN", {
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
  const [open, setOpen] = useState(false);
  const [info, setInfo] = useState<VersionInfoDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (refresh: boolean) => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.version(refresh);
      setInfo(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "检查失败");
      setInfo({
        current: APP_VERSION,
        latest: APP_VERSION,
        update_available: false,
        image: "jovepoxy",
        checked_at: new Date().toISOString(),
        source: "local",
        note: "无法连接版本检查接口",
      });
    } finally {
      setLoading(false);
    }
  }, []);

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
        aria-label={`当前版本 v${APP_VERSION}，点击查看更新`}
      >
        v{APP_VERSION}
      </button>

      <Dialog open={open} title="版本" onClose={() => setOpen(false)}>
        <div className="flex flex-col gap-3.5">
          <div className="grid gap-3">
            <div>
              <p className="text-[12px] text-ink-muted">当前版本</p>
              <p className="mt-0.5 font-mono text-[18px] font-semibold tracking-tight text-ink">
                v{current}
              </p>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">最新版本</p>
              <div className="mt-0.5 flex flex-wrap items-center gap-2">
                <p className="font-mono text-[18px] font-semibold tracking-tight text-ink">
                  v{latest}
                </p>
                {update ? <Badge kind="healthy">可更新</Badge> : null}
                {!update && info && !error ? (
                  <Badge kind="neutral">最新</Badge>
                ) : null}
              </div>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">镜像 / 产物</p>
              <p className="mt-0.5 break-all font-mono text-[13px] text-ink">
                {info?.image ?? "jovepoxy"}
              </p>
            </div>
            <div>
              <p className="text-[12px] text-ink-muted">检查时间</p>
              <p className="mt-0.5 text-[13px] text-ink">
                {formatCheckedAt(info?.checked_at)}
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
            远端检查读取公开 GitHub Releases（VERSION_REPO=owner/repo）。私有仓库不会返回更新。Docker
            环境请 docker pull / 重建镜像后重启。
          </p>

          <div className="flex justify-end gap-2 pt-1">
            <Button
              variant="secondary"
              size="sm"
              loading={loading}
              onClick={() => void load(true)}
            >
              立即检查
            </Button>
            <Button size="sm" onClick={() => setOpen(false)}>
              关闭
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
}
