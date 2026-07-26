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

type InfoRow = {
  readonly label: string;
  readonly value: string;
  readonly tip: string;
  readonly badge?: boolean;
  readonly on?: boolean;
};

function serviceRows(s: SettingsDTO): InfoRow[] {
  return [
    {
      label: "监听地址",
      value: s.listen,
      tip: "服务对外监听 host:port。改 LISTEN 环境变量后需重启。",
    },
    {
      label: "数据目录",
      value: s.data_dir,
      tip: "SQLite 与加密密钥存放路径（DATA_DIR）。",
    },
    {
      label: "上游 Zen 基址",
      value: s.zen_base,
      tip: "OpenCode paid 模型转发地址（ZEN_BASE）。",
    },
    {
      label: "上游超时",
      value: `${s.upstream_timeout_seconds} 秒`,
      tip: "转发上游请求的超时时间（UPSTREAM_TIMEOUT）。",
    },
    {
      label: "进程 HTTP 代理",
      value: s.http_proxy_configured ? "已配置" : "未配置",
      tip: "环境变量 HTTP_PROXY 是否已设置（影响上游出站）。",
      badge: true,
      on: s.http_proxy_configured,
    },
    {
      label: "进程 HTTPS 代理",
      value: s.https_proxy_configured ? "已配置" : "未配置",
      tip: "环境变量 HTTPS_PROXY 是否已设置。",
      badge: true,
      on: s.https_proxy_configured,
    },
  ];
}

function modelRows(s: SettingsDTO): InfoRow[] {
  return [
    {
      label: "展示全部模型",
      value: s.show_all_models ? "开启" : "关闭",
      tip: "SHOW_ALL_MODELS=true 时，/v1/models 会同时返回 paid 模型。",
      badge: true,
      on: s.show_all_models,
    },
    {
      label: "模型目录缓存",
      value: `${s.model_cache_ttl_seconds} 秒`,
      tip: "模型列表缓存时长（MODEL_CACHE_TTL）。到期后下次请求会刷新。",
    },
    {
      label: "OpenCode 客户端版本",
      value: s.oc_version,
      tip: "请求上游时声明的客户端版本（OC_VERSION），不是本管理台版本号。",
    },
    {
      label: "Cookie Secure",
      value: s.cookie_secure ? "开启" : "关闭",
      tip: "COOKIE_SECURE=true 时会话 Cookie 仅经 HTTPS 发送。反代 HTTPS 后建议开启。",
      badge: true,
      on: s.cookie_secure,
    },
    {
      label: "管理会话有效期",
      value: `${s.session_ttl_hours} 小时`,
      tip: "登录成功后会话 cookie 的有效时长。",
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
  const [settings, setSettings] = useState<SettingsDTO | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [savingPassword, setSavingPassword] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setSettings(await api.settings());
      setError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setError(err instanceof Error ? err.message : "加载失败");
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
      push("新密码至少 8 位", "error");
      return;
    }
    if (newPassword !== confirmPassword) {
      push("两次输入的新密码不一致", "error");
      return;
    }
    setSavingPassword(true);
    try {
      await api.changePassword(currentPassword, newPassword);
      // Backend revokes all sessions + clears cookie; force re-login.
      setSessionHint(false);
      push("密码已更新，请使用新密码重新登录", "success");
      void navigate("/login", { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        push("当前密码不正确", "error");
      } else {
        push(err instanceof Error ? err.message : "修改失败", "error");
      }
    } finally {
      setSavingPassword(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="设置"
        description="管理台鉴权、服务连接与运行参数。标 ? 的说明悬停查看。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => void load()}>
            刷新
          </Button>
        }
      />

      {loading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-48 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : null}
      {!loading && error ? <ErrorState title="加载失败" description={error} /> : null}

      {!loading && settings ? (
        <>
          <SectionPanel
            title="登录鉴权"
            description={
              settings.password_custom
                ? "当前使用已自定义的管理密码（保存在本地数据）"
                : "当前使用环境变量 ADMIN_PASSWORD；在此修改会写入本地并覆盖环境密码"
            }
            bodyClassName="!p-4 sm:!p-5"
          >
            <form
              className="grid max-w-xl gap-3"
              onSubmit={(e) => void onChangePassword(e)}
            >
              <SecretInput
                label="当前密码"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
              <div className="grid gap-3 sm:grid-cols-2">
                <SecretInput
                  label="新密码"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
                <SecretInput
                  label="确认新密码"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
                <p className="text-[12px] text-ink-faint">
                  至少 8 位；保存后所有会话立即失效并回到登录页
                </p>
                <Button type="submit" size="sm" loading={savingPassword}>
                  更新密码
                </Button>
              </div>
            </form>
          </SectionPanel>

          <SectionPanel
            title="服务连接"
            description="进程级连接参数（多数需改环境变量并重启）"
            bodyClassName="p-0"
          >
            <InfoList rows={serviceRows(settings)} />
          </SectionPanel>

          <SectionPanel
            title="模型与会话"
            description="目录缓存、模型可见性与管理 Cookie"
            bodyClassName="p-0"
          >
            <InfoList rows={modelRows(settings)} />
          </SectionPanel>

          <SectionPanel
            title="环境变量速查"
            description="在 start.bat 或系统环境中配置"
            bodyClassName="!p-4 sm:!p-5"
          >
            <div className="grid gap-2 sm:grid-cols-2">
              {(
                [
                  ["ADMIN_PASSWORD", "管理台登录密码（初始）"],
                  ["ADMIN_SECRET", "加密密钥，至少 32 字符"],
                  ["LISTEN", "监听地址"],
                  ["DATA_DIR", "数据目录"],
                  ["ZEN_BASE", "上游 Zen API"],
                  ["SHOW_ALL_MODELS", "是否暴露 paid 模型"],
                  ["COOKIE_SECURE", "HTTPS 下的安全 Cookie"],
                  ["MODEL_CACHE_TTL", "模型缓存时长"],
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
