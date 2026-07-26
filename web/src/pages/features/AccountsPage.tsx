import {
  Cloud,
  DownloadSimple,
  Plus,
  UploadSimple,
  UsersThree,
} from "@phosphor-icons/react";
import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Badge,
  Button,
  DeleteButton,
  Dialog,
  EmptyState,
  ErrorState,
  MetaChip,
  PageHeader,
  Pagination,
  SearchField,
  SegmentedFilter,
  SelectionStrip,
  SecretInput,
  Skeleton,
  Tabs,
  TextInput,
  slicePage,
} from "@/components";
import {
  downloadJSON,
  parseOllamaBatchLines,
  parseOllamaImportJSON,
  parseOpenCodeBatchLines,
  parseOpenCodeImportJSON,
  type OllamaExportBundle,
  type OllamaImportItem,
  type OpenCodeExportBundle,
  type OpenCodeImportItem,
} from "@/lib/account-io";
import { api, ApiError, type AccountDTO, type OllamaAccountDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { isProviderTab, type ProviderTab } from "@/lib/routes";

type StatusFilter = "all" | "enabled" | "disabled";
type DialogMode = "closed" | "add" | "batch" | "import";

function useProviderTab(
  defaultTab: ProviderTab = "opencode",
): readonly [ProviderTab, (tab: ProviderTab) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  const tab: ProviderTab = isProviderTab(raw) ? raw : defaultTab;
  function setTab(next: ProviderTab) {
    setParams(next === defaultTab ? {} : { tab: next }, { replace: true });
  }
  return [tab, setTab] as const;
}

export function AccountsPage() {
  const navigate = useNavigate();
  const fileRef = useRef<HTMLInputElement>(null);
  const [tab, setTab] = useProviderTab("opencode");
  const [ocAccounts, setOcAccounts] = useState<AccountDTO[]>([]);
  const [olAccounts, setOlAccounts] = useState<OllamaAccountDTO[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [dialog, setDialog] = useState<DialogMode>("closed");
  const [exportSecrets, setExportSecrets] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [ocName, setOcName] = useState("");
  const [workspaceID, setWorkspaceID] = useState("wrk_");
  const [ocCookie, setOcCookie] = useState("");
  const [olName, setOlName] = useState("");
  const [olCookie, setOlCookie] = useState("");
  const [batchText, setBatchText] = useState("");
  const [importText, setImportText] = useState("");

  async function load() {
    setLoading(true);
    try {
      const [oc, ol] = await Promise.all([api.accounts(), api.ollamaAccounts()]);
      setOcAccounts(oc.accounts);
      setOlAccounts([...ol.accounts]);
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

  useEffect(() => {
    setSelected(new Set());
    setQuery("");
    setStatus("all");
    setPage(1);
  }, [tab]);

  useEffect(() => {
    setPage(1);
  }, [query, status]);

  const ocEnabled = ocAccounts.filter((a) => a.enabled).length;
  const olEnabled = olAccounts.filter((a) => a.enabled).length;

  const filteredOc = useMemo(() => {
    const q = query.trim().toLowerCase();
    return ocAccounts.filter((a) => {
      if (status === "enabled" && !a.enabled) return false;
      if (status === "disabled" && a.enabled) return false;
      if (!q) return true;
      return (
        a.name.toLowerCase().includes(q) ||
        a.workspace_id.toLowerCase().includes(q) ||
        a.masked_cookie.toLowerCase().includes(q)
      );
    });
  }, [ocAccounts, query, status]);

  const filteredOl = useMemo(() => {
    const q = query.trim().toLowerCase();
    return olAccounts.filter((a) => {
      if (status === "enabled" && !a.enabled) return false;
      if (status === "disabled" && a.enabled) return false;
      if (!q) return true;
      return a.name.toLowerCase().includes(q) || a.masked_cookie.toLowerCase().includes(q);
    });
  }, [olAccounts, query, status]);

  const rows = tab === "opencode" ? filteredOc : filteredOl;
  const pagedOc = useMemo(
    () => slicePage(filteredOc, page, pageSize),
    [filteredOc, page, pageSize],
  );
  const pagedOl = useMemo(
    () => slicePage(filteredOl, page, pageSize),
    [filteredOl, page, pageSize],
  );
  const pageRows = tab === "opencode" ? pagedOc : pagedOl;
  const rowIds = pageRows.map((r) => r.id);
  const allSelected = rowIds.length > 0 && rowIds.every((id) => selected.has(id));

  function toggleAll() {
    setSelected(allSelected ? new Set() : new Set(rowIds));
  }

  function toggleOne(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function runImportOpenCode(items: OpenCodeImportItem[]) {
    let ok = 0;
    const fails: string[] = [];
    for (const item of items) {
      try {
        await api.createAccount(item);
        ok += 1;
      } catch (err) {
        fails.push(`${item.name}: ${err instanceof Error ? err.message : "失败"}`);
      }
    }
    return { ok, fails };
  }

  async function runImportOllama(items: OllamaImportItem[]) {
    let ok = 0;
    const fails: string[] = [];
    for (const item of items) {
      try {
        await api.createOllamaAccount(item);
        ok += 1;
      } catch (err) {
        fails.push(`${item.name}: ${err instanceof Error ? err.message : "失败"}`);
      }
    }
    return { ok, fails };
  }

  async function onCreateSingle(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setNotice(null);
    try {
      if (tab === "opencode") {
        await api.createAccount({
          name: ocName.trim() || "account",
          workspace_id: workspaceID.trim(),
          auth_cookie: ocCookie.trim(),
          show_rolling: true,
          show_weekly: true,
          show_monthly: true,
          enabled: true,
        });
        setOcName("");
        setWorkspaceID("wrk_");
        setOcCookie("");
      } else {
        await api.createOllamaAccount({
          name: olName.trim() || "ollama",
          session_cookie: olCookie.trim(),
          enabled: true,
        });
        setOlName("");
        setOlCookie("");
      }
      setDialog("closed");
      setNotice("已添加账号");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  async function onBatchSubmit() {
    setBusy(true);
    setNotice(null);
    try {
      if (tab === "opencode") {
        const parsed = parseOpenCodeBatchLines(batchText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || "没有可导入行");
          return;
        }
        const result = await runImportOpenCode(parsed.items);
        setNotice(`批量完成：成功 ${result.ok}，失败 ${result.fails.length}`);
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      } else {
        const parsed = parseOllamaBatchLines(batchText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || "没有可导入行");
          return;
        }
        const result = await runImportOllama(parsed.items);
        setNotice(`批量完成：成功 ${result.ok}，失败 ${result.fails.length}`);
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      }
      setBatchText("");
      setDialog("closed");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function onImportSubmit() {
    setBusy(true);
    setNotice(null);
    try {
      if (tab === "opencode") {
        const parsed = parseOpenCodeImportJSON(importText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || "没有可导入项");
          return;
        }
        const result = await runImportOpenCode(parsed.items);
        setNotice(`导入完成：成功 ${result.ok}，失败 ${result.fails.length}`);
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      } else {
        const parsed = parseOllamaImportJSON(importText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || "没有可导入项");
          return;
        }
        const result = await runImportOllama(parsed.items);
        setNotice(`导入完成：成功 ${result.ok}，失败 ${result.fails.length}`);
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      }
      setImportText("");
      setDialog("closed");
      await load();
    } finally {
      setBusy(false);
    }
  }

  async function onExport() {
    setBusy(true);
    setError(null);
    try {
      if (exportSecrets) {
        const confirmed = window.confirm(
          "导出将包含完整 cookie 密钥，请妥善保管文件。确认继续？",
        );
        if (!confirmed) return;
      }
      const stamp = new Date().toISOString();
      if (tab === "opencode") {
        const accounts = [];
        for (const account of ocAccounts) {
          if (exportSecrets) {
            const cred = await api.getAccountCredential(account.id);
            accounts.push({
              name: account.name,
              workspace_id: account.workspace_id,
              enabled: account.enabled,
              show_rolling: account.show_rolling,
              show_weekly: account.show_weekly,
              show_monthly: account.show_monthly,
              auth_cookie: cred.auth_cookie,
            });
          } else {
            accounts.push({
              name: account.name,
              workspace_id: account.workspace_id,
              enabled: account.enabled,
              show_rolling: account.show_rolling,
              show_weekly: account.show_weekly,
              show_monthly: account.show_monthly,
              masked_cookie: account.masked_cookie,
            });
          }
        }
        const bundle: OpenCodeExportBundle = {
          version: 1,
          provider: "opencode",
          exported_at: stamp,
          include_secrets: exportSecrets,
          accounts,
        };
        downloadJSON(
          `opencode-accounts-${exportSecrets ? "secrets" : "manifest"}.json`,
          bundle,
        );
      } else {
        const accounts = [];
        for (const account of olAccounts) {
          if (exportSecrets) {
            const cred = await api.getOllamaAccountCredential(account.id);
            accounts.push({
              name: account.name,
              enabled: account.enabled,
              show_session: account.show_session,
              show_weekly: account.show_weekly,
              session_cookie: cred.session_cookie,
            });
          } else {
            accounts.push({
              name: account.name,
              enabled: account.enabled,
              show_session: account.show_session,
              show_weekly: account.show_weekly,
              masked_cookie: account.masked_cookie,
            });
          }
        }
        const bundle: OllamaExportBundle = {
          version: 1,
          provider: "ollama",
          exported_at: stamp,
          include_secrets: exportSecrets,
          accounts,
        };
        downloadJSON(
          `ollama-accounts-${exportSecrets ? "secrets" : "manifest"}.json`,
          bundle,
        );
      }
      setNotice(exportSecrets ? "已导出（含密钥）" : "已导出清单（不含密钥）");
    } catch (err) {
      setError(err instanceof Error ? err.message : "导出失败");
    } finally {
      setBusy(false);
    }
  }

  async function bulkSetEnabled(enabled: boolean) {
    if (selected.size === 0) return;
    setBusy(true);
    try {
      for (const id of selected) {
        if (tab === "opencode") await api.setAccountEnabled(id, enabled);
        else await api.setOllamaAccountEnabled(id, enabled);
      }
      setSelected(new Set());
      setNotice(enabled ? "已批量启用" : "已批量禁用");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "批量更新失败");
    } finally {
      setBusy(false);
    }
  }

  async function bulkDelete() {
    if (selected.size === 0) return;
    if (!window.confirm(`确认删除选中的 ${selected.size} 个账号？`)) return;
    setBusy(true);
    try {
      for (const id of selected) {
        if (tab === "opencode") await api.deleteAccount(id);
        else await api.deleteOllamaAccount(id);
      }
      setSelected(new Set());
      setNotice("已批量删除");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "批量删除失败");
    } finally {
      setBusy(false);
    }
  }

  function onFilePicked(file: File | null) {
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      setImportText(String(reader.result ?? ""));
      setDialog("import");
    };
    reader.readAsText(file);
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="账号统计"
        description="控制面账号。支持筛选、批量操作与 JSON 导入导出。"
        toolbar={
          <Tabs
            aria-label="提供商"
            value={tab}
            onChange={(id) => setTab(id as ProviderTab)}
            items={[
              { id: "opencode", label: "OpenCode", count: ocAccounts.length },
              { id: "ollama", label: "Ollama", count: olAccounts.length },
            ]}
          />
        }
        meta={
          <>
            <MetaChip>合计 {ocAccounts.length + olAccounts.length}</MetaChip>
            <MetaChip>启用 {ocEnabled + olEnabled}</MetaChip>
          </>
        }
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void navigate("/app/quotas")}>
              额度监控
            </Button>
            <Button size="sm" onClick={() => setDialog("add")}>
              <Plus size={16} className="mr-1" />
              添加
            </Button>
          </div>
        }
      />

      <div className="flex flex-col overflow-hidden rounded-none border border-border bg-paper-1">
        <div className="flex flex-col gap-2 border-b border-border bg-paper-0/35 px-3 py-2.5">
          <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              <SearchField
                value={query}
                onChange={setQuery}
                placeholder="名称 / workspace / cookie"
              />
              <SegmentedFilter
                aria-label="状态"
                value={status}
                onChange={(v) => setStatus(v as StatusFilter)}
                options={[
                  { value: "all", label: "全部" },
                  { value: "enabled", label: "启用" },
                  { value: "disabled", label: "禁用" },
                ]}
              />
            </div>
            <div className="flex flex-wrap items-center gap-1.5 lg:justify-end">
              <label className="flex h-8 items-center gap-1.5 rounded-none border border-border bg-paper-0 px-2.5 text-[12px] text-ink-muted">
                <input
                  type="checkbox"
                  checked={exportSecrets}
                  onChange={(e) => setExportSecrets(e.target.checked)}
                />
                导出含密钥
              </label>
              <Button variant="secondary" size="sm" loading={busy} onClick={() => void onExport()}>
                <DownloadSimple size={14} className="mr-1" />
                导出
              </Button>
              <Button variant="secondary" size="sm" onClick={() => fileRef.current?.click()}>
                <UploadSimple size={14} className="mr-1" />
                导入
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setDialog("import")}>
                粘贴 JSON
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setDialog("batch")}>
                批量添加
              </Button>
              <input
                ref={fileRef}
                type="file"
                accept="application/json,.json"
                className="hidden"
                onChange={(e) => onFilePicked(e.target.files?.[0] ?? null)}
              />
            </div>
          </div>
          {selected.size > 0 ? (
            <SelectionStrip
              selectedCount={selected.size}
              totalVisible={rowIds.length}
              allSelected={allSelected}
              onSelectAll={toggleAll}
              onInvert={() => {
                setSelected((prev) => {
                  const next = new Set(prev);
                  for (const id of rowIds) {
                    if (next.has(id)) next.delete(id);
                    else next.add(id);
                  }
                  return next;
                });
              }}
              onClear={() => setSelected(new Set())}
              bulkActions={
                <>
                  <Button
                    variant="secondary"
                    size="sm"
                    loading={busy}
                    onClick={() => void bulkSetEnabled(true)}
                  >
                    启用
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    loading={busy}
                    onClick={() => void bulkSetEnabled(false)}
                  >
                    禁用
                  </Button>
                  <DeleteButton loading={busy} onClick={() => void bulkDelete()} />
                </>
              }
            />
          ) : null}
        </div>

        {notice ? (
          <p className="mx-3 mt-2 rounded-none border border-border bg-paper-0/80 px-3 py-2 text-[12px] text-status-success">
            {notice}
          </p>
        ) : null}
        {error ? (
          <p className="mx-3 mt-2 rounded-none border border-border bg-paper-0 px-3 py-2 text-[12px] text-status-error">
            {error}
          </p>
        ) : null}

        {loading ? (
          <div className="p-3">
            <Skeleton className="h-40 w-full" />
          </div>
        ) : null}

        {!loading &&
        error &&
        rows.length === 0 &&
        ocAccounts.length + olAccounts.length === 0 ? (
          <ErrorState title="加载失败" description={error} />
        ) : null}

        {!loading && rows.length === 0 ? (
          <EmptyState
            icon={tab === "opencode" ? UsersThree : Cloud}
            title={tab === "opencode" ? "暂无 OpenCode 账号" : "暂无 Ollama 账号"}
            description="可手动添加、批量粘贴，或导入 JSON。"
            action={
              <div className="flex flex-wrap justify-center gap-2">
                <Button onClick={() => setDialog("add")}>添加账号</Button>
                <Button variant="secondary" onClick={() => setDialog("import")}>
                  导入 JSON
                </Button>
              </div>
            }
          />
        ) : null}

        {!loading && rows.length > 0 ? (
          <div className="overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[28rem] text-left text-sm md:min-w-[40rem]">
                <thead>
                  <tr className="border-b border-border bg-paper-0/70 text-caption text-ink-muted">
                    <th className="w-10 px-3 py-2.5">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        onChange={toggleAll}
                        aria-label="全选"
                      />
                    </th>
                    <th className="px-3 py-2.5 font-medium">名称</th>
                    {tab === "opencode" ? (
                      <th className="px-3 py-2.5 font-medium">Workspace</th>
                    ) : null}
                    <th className="px-3 py-2.5 font-medium">Cookie</th>
                    <th className="px-3 py-2.5 font-medium">状态</th>
                    <th className="px-3 py-2.5 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {tab === "opencode"
                    ? pagedOc.map((account) => (
                        <tr
                          key={account.id}
                          className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                        >
                          <td className="px-3 py-2.5">
                            <input
                              type="checkbox"
                              checked={selected.has(account.id)}
                              onChange={() => toggleOne(account.id)}
                              aria-label={`选择 ${account.name}`}
                            />
                          </td>
                          <td className="px-3 py-2.5 font-medium text-ink">{account.name}</td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.workspace_id}
                          </td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.masked_cookie}
                          </td>
                          <td className="px-3 py-2.5">
                            {account.enabled ? (
                              <Badge kind="healthy">启用</Badge>
                            ) : (
                              <Badge kind="neutral">禁用</Badge>
                            )}
                          </td>
                          <td className="px-3 py-2.5">
                            <div className="flex flex-wrap gap-1.5">
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() =>
                                  void api
                                    .setAccountEnabled(account.id, !account.enabled)
                                    .then(load)
                                }
                              >
                                {account.enabled ? "禁用" : "启用"}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(`删除 ${account.name}？`)) {
                                    void api.deleteAccount(account.id).then(load);
                                  }
                                }}
                              />
                            </div>
                          </td>
                        </tr>
                      ))
                    : pagedOl.map((account) => (
                        <tr
                          key={account.id}
                          className="border-b border-border last:border-b-0 hover:bg-paper-0/50"
                        >
                          <td className="px-3 py-2.5">
                            <input
                              type="checkbox"
                              checked={selected.has(account.id)}
                              onChange={() => toggleOne(account.id)}
                              aria-label={`选择 ${account.name}`}
                            />
                          </td>
                          <td className="px-3 py-2.5 font-medium text-ink">{account.name}</td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.masked_cookie}
                          </td>
                          <td className="px-3 py-2.5">
                            {account.enabled ? (
                              <Badge kind="healthy">启用</Badge>
                            ) : (
                              <Badge kind="neutral">禁用</Badge>
                            )}
                          </td>
                          <td className="px-3 py-2.5">
                            <div className="flex flex-wrap gap-1.5">
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() =>
                                  void api
                                    .setOllamaAccountEnabled(account.id, !account.enabled)
                                    .then(load)
                                }
                              >
                                {account.enabled ? "禁用" : "启用"}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(`删除 ${account.name}？`)) {
                                    void api.deleteOllamaAccount(account.id).then(load);
                                  }
                                }}
                              />
                            </div>
                          </td>
                        </tr>
                      ))}
                </tbody>
              </table>
            </div>
            <Pagination
              total={rows.length}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              onPageSizeChange={(size) => {
                setPageSize(size);
                setPage(1);
              }}
            />
          </div>
        ) : null}
      </div>

      <Dialog
        open={dialog === "add"}
        title={tab === "opencode" ? "添加 OpenCode 账号" : "添加 Ollama 账号"}
        description={
          tab === "opencode" ? "需要 workspace 与 auth cookie" : "session cookie 加密存储"
        }
        onClose={() => setDialog("closed")}
        className="max-w-lg"
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onCreateSingle(e)}>
          {tab === "opencode" ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="名称"
                  value={ocName}
                  onChange={(e) => setOcName(e.target.value)}
                />
                <TextInput
                  label="Workspace ID"
                  value={workspaceID}
                  onChange={(e) => setWorkspaceID(e.target.value)}
                />
              </div>
              <SecretInput
                label="auth_cookie"
                value={ocCookie}
                onChange={(e) => setOcCookie(e.target.value)}
              />
            </>
          ) : (
            <>
              <TextInput
                label="名称"
                value={olName}
                onChange={(e) => setOlName(e.target.value)}
              />
              <SecretInput
                label="session cookie"
                value={olCookie}
                onChange={(e) => setOlCookie(e.target.value)}
              />
            </>
          )}
          <div className="flex justify-end gap-2 pt-1">
            <Button variant="secondary" type="button" onClick={() => setDialog("closed")}>
              取消
            </Button>
            <Button type="submit" loading={busy}>
              添加
            </Button>
          </div>
        </form>
      </Dialog>

      <Dialog
        open={dialog === "batch"}
        title="批量添加"
        description={
          tab === "opencode"
            ? "每行：名称|wrk_xxx|auth=..."
            : "每行：名称|session_cookie 或纯 cookie"
        }
        onClose={() => setDialog("closed")}
        className="max-w-xl"
      >
        <textarea
          value={batchText}
          onChange={(e) => setBatchText(e.target.value)}
          rows={10}
          className="w-full rounded-none border border-border bg-paper-0 p-3 font-mono text-[12px] text-ink outline-none focus:ring-2 focus:ring-focus-ring"
          placeholder={
            tab === "opencode"
              ? "main|wrk_abc123|auth=xxxxx"
              : "pro-1|__Secure-session=xxxxx"
          }
        />
        <div className="mt-3 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setDialog("closed")}>
            取消
          </Button>
          <Button loading={busy} onClick={() => void onBatchSubmit()}>
            导入行
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={dialog === "import"}
        title="导入 JSON"
        description={`provider 须为 ${tab}，accounts 数组含密钥字段`}
        onClose={() => setDialog("closed")}
        className="max-w-xl"
      >
        <textarea
          value={importText}
          onChange={(e) => setImportText(e.target.value)}
          rows={12}
          className="w-full rounded-none border border-border bg-paper-0 p-3 font-mono text-[12px] text-ink outline-none focus:ring-2 focus:ring-focus-ring"
          placeholder='{"version":1,"provider":"opencode","accounts":[...]}'
        />
        <div className="mt-3 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setDialog("closed")}>
            取消
          </Button>
          <Button loading={busy} onClick={() => void onImportSubmit()}>
            开始导入
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
