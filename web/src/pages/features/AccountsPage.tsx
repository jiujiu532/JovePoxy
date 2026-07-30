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
  EntityMark,
  MetricRail,
  PageHeader,
  Pagination,
  PosterEmpty,
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
import { useI18n } from "@/lib/i18n";
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
  const { t } = useI18n();
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
      setError(err instanceof Error ? err.message : t("common.loadFailed"));
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
  const activeAccounts = tab === "opencode" ? ocAccounts : olAccounts;
  const activeEnabled = tab === "opencode" ? ocEnabled : olEnabled;
  const activeDisabled = Math.max(0, activeAccounts.length - activeEnabled);
  const otherPoolCount = tab === "opencode" ? olAccounts.length : ocAccounts.length;

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
        fails.push(`${item.name}: ${err instanceof Error ? err.message : t("accounts.itemFailed")}`);
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
        fails.push(`${item.name}: ${err instanceof Error ? err.message : t("accounts.itemFailed")}`);
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
      setNotice(t("accounts.accountAdded"));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("common.createFailed"));
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
          setError(parsed.errors.join("；") || t("accounts.noImportableLines"));
          return;
        }
        const result = await runImportOpenCode(parsed.items);
        setNotice(t("accounts.batchResult", { ok: result.ok, fail: result.fails.length }));
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      } else {
        const parsed = parseOllamaBatchLines(batchText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || t("accounts.noImportableLines"));
          return;
        }
        const result = await runImportOllama(parsed.items);
        setNotice(t("accounts.batchResult", { ok: result.ok, fail: result.fails.length }));
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
          setError(parsed.errors.join("；") || t("accounts.noImportableItems"));
          return;
        }
        const result = await runImportOpenCode(parsed.items);
        setNotice(t("accounts.importResult", { ok: result.ok, fail: result.fails.length }));
        if (result.fails.length > 0) setError(result.fails.slice(0, 5).join("；"));
      } else {
        const parsed = parseOllamaImportJSON(importText);
        if (parsed.items.length === 0) {
          setError(parsed.errors.join("；") || t("accounts.noImportableItems"));
          return;
        }
        const result = await runImportOllama(parsed.items);
        setNotice(t("accounts.importResult", { ok: result.ok, fail: result.fails.length }));
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
        const confirmed = window.confirm(t("accounts.exportConfirmSecrets"));
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
      setNotice(exportSecrets ? t("accounts.exportedWithSecrets") : t("accounts.exportedManifest"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("accounts.exportFailed"));
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
      setNotice(enabled ? t("accounts.bulkEnabled") : t("accounts.bulkDisabled"));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("accounts.bulkUpdateFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function bulkDelete() {
    if (selected.size === 0) return;
    if (!window.confirm(t("accounts.bulkDeleteConfirm", { n: selected.size }))) return;
    setBusy(true);
    try {
      for (const id of selected) {
        if (tab === "opencode") await api.deleteAccount(id);
        else await api.deleteOllamaAccount(id);
      }
      setSelected(new Set());
      setNotice(t("accounts.bulkDeleted"));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("accounts.bulkDeleteFailed"));
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
        title={t("accounts.title")}
        toolbar={
          <Tabs
            aria-label={t("accounts.providerTabAria")}
            value={tab}
            onChange={(id) => setTab(id as ProviderTab)}
            items={[
              { id: "opencode", label: "OpenCode", count: ocAccounts.length },
              { id: "ollama", label: "Ollama", count: olAccounts.length },
            ]}
          />
        }
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void navigate("/app/quotas")}>
              {t("accounts.quotasLink")}
            </Button>
            <Button size="sm" onClick={() => setDialog("add")}>
              <Plus size={16} className="mr-1" />
              {t("common.add")}
            </Button>
          </div>
        }
      />

      {loading ? (
        <div className="space-y-2 border-2 border-border bg-paper-0 p-3 shadow-[4px_4px_0_var(--border)]">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : error && activeAccounts.length === 0 && ocAccounts.length + olAccounts.length === 0 ? (
        <EmptyState
          icon={UsersThree}
          title={t("common.loadFailed")}
          description={error}
          action={
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.retry")}
            </Button>
          }
        />
      ) : activeAccounts.length === 0 ? (
        <PosterEmpty
          stamp={
            tab === "opencode"
              ? t("accounts.posterStampOc")
              : t("accounts.posterStampOl")
          }
          stampSub={t("accounts.posterStampSub")}
          title={
            tab === "opencode"
              ? t("accounts.emptyTitleOc")
              : t("accounts.emptyTitleOl")
          }
          description={
            tab === "opencode"
              ? t("accounts.emptyDescOc")
              : t("accounts.emptyDescOl")
          }
          note={t("accounts.posterNote")}
          action={
            <div className="flex flex-wrap gap-2">
              <Button
                className="!px-5 !py-3 !text-[15px] !font-black shadow-[6px_6px_0_var(--border)]"
                onClick={() => setDialog("add")}
              >
                <Plus size={16} className="mr-1" weight="bold" />
                {tab === "opencode"
                  ? t("accounts.addCtaOc")
                  : t("accounts.addCtaOl")}
              </Button>
              <Button
                variant="secondary"
                className="!px-4 !py-3 !text-[14px] !font-bold shadow-[4px_4px_0_var(--border)]"
                onClick={() => setDialog("import")}
              >
                {t("accounts.importJson")}
              </Button>
            </div>
          }
          bars={[
            {
              label: t("accounts.barQuotaLabel"),
              detail:
                tab === "opencode"
                  ? t("accounts.barQuotaDetailOc")
                  : t("accounts.barQuotaDetailOl"),
              tone: tab === "opencode" ? "accent" : "coral",
            },
            {
              label: t("accounts.barCredLabel"),
              detail:
                tab === "opencode"
                  ? t("accounts.barCredDetailOc")
                  : t("accounts.barCredDetailOl"),
              tone: "teal",
            },
            {
              label: t("accounts.barIoLabel"),
              detail: t("accounts.barIoDetail"),
              tone: "mint",
            },
          ]}
        />
      ) : (
        <>
          <MetricRail
            items={[
              {
                label: t("kpi.total"),
                value: activeAccounts.length,
                hint: t("accounts.railTotalHint"),
                tone: "yellow",
              },
              {
                label: t("common.enabled"),
                value: activeEnabled,
                hint: t("accounts.railEnabledHint"),
                tone: "teal",
              },
              {
                label: t("common.disabled"),
                value: activeDisabled,
                hint: t("accounts.railDisabledHint"),
                tone: "white",
              },
              {
                label: tab === "opencode" ? "Ollama" : "OpenCode",
                value: otherPoolCount,
                hint: t("accounts.railOtherHint"),
                tone: "mint",
              },
            ]}
          />

      <div className="flex flex-col overflow-hidden rounded-none border-2 border-border bg-paper-1 shadow-[var(--shadow-hard)]">
        <div className="flex flex-col gap-2 border-b border-border bg-paper-0/35 px-3 py-2.5">
          <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              <SearchField
                value={query}
                onChange={setQuery}
                placeholder={t("accounts.searchPlaceholder")}
              />
              <SegmentedFilter
                aria-label={t("accounts.statusAria")}
                value={status}
                onChange={(v) => setStatus(v as StatusFilter)}
                options={[
                  { value: "all", label: t("common.all") },
                  { value: "enabled", label: t("common.enabled") },
                  { value: "disabled", label: t("common.disabled") },
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
                {t("accounts.exportWithSecrets")}
              </label>
              <Button variant="secondary" size="sm" loading={busy} onClick={() => void onExport()}>
                <DownloadSimple size={14} className="mr-1" />
                {t("common.export")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => fileRef.current?.click()}>
                <UploadSimple size={14} className="mr-1" />
                {t("common.import")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setDialog("import")}>
                {t("accounts.pasteJson")}
              </Button>
              <Button variant="secondary" size="sm" onClick={() => setDialog("batch")}>
                {t("accounts.batchAdd")}
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
                    {t("common.enable")}
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    loading={busy}
                    onClick={() => void bulkSetEnabled(false)}
                  >
                    {t("common.disable")}
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

        {rows.length === 0 ? (
          <EmptyState
            compact
            icon={tab === "opencode" ? UsersThree : Cloud}
            title={t("accounts.noMatchTitle")}
            description={t("accounts.noMatchDesc")}
          />
        ) : (
          <div className="overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[28rem] text-left text-sm md:min-w-[40rem]">
                <thead>
                  <tr className="border-b-2 border-border bg-paper-0 text-caption text-ink-muted">
                    <th className="w-10 px-3 py-2.5">
                      <input
                        type="checkbox"
                        checked={allSelected}
                        onChange={toggleAll}
                        aria-label={t("accounts.selectAllAria")}
                      />
                    </th>
                    <th className="px-3 py-2.5 font-medium">{t("accounts.colName")}</th>
                    {tab === "opencode" ? (
                      <th className="px-3 py-2.5 font-medium">Workspace</th>
                    ) : null}
                    <th className="px-3 py-2.5 font-medium">Cookie</th>
                    <th className="px-3 py-2.5 font-medium">{t("accounts.colStatus")}</th>
                    <th className="px-3 py-2.5 font-medium">{t("accounts.colActions")}</th>
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
                              aria-label={t("accounts.selectRowAria", { name: account.name })}
                            />
                          </td>
                          <td className="px-3 py-2.5 font-medium text-ink">
                            <span className="inline-flex min-w-0 items-center gap-2.5">
                              <EntityMark name={account.name} size="sm" />
                              <span className="truncate">{account.name}</span>
                            </span>
                          </td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.workspace_id}
                          </td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.masked_cookie}
                          </td>
                          <td className="px-3 py-2.5">
                            {account.enabled ? (
                              <Badge kind="healthy">{t("common.enabled")}</Badge>
                            ) : (
                              <Badge kind="neutral">{t("common.disabled")}</Badge>
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
                                {account.enabled ? t("common.disable") : t("common.enable")}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(t("accounts.deleteConfirm", { name: account.name }))) {
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
                              aria-label={t("accounts.selectRowAria", { name: account.name })}
                            />
                          </td>
                          <td className="px-3 py-2.5 font-medium text-ink">
                            <span className="inline-flex min-w-0 items-center gap-2.5">
                              <EntityMark name={account.name} size="sm" />
                              <span className="truncate">{account.name}</span>
                            </span>
                          </td>
                          <td className="px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {account.masked_cookie}
                          </td>
                          <td className="px-3 py-2.5">
                            {account.enabled ? (
                              <Badge kind="healthy">{t("common.enabled")}</Badge>
                            ) : (
                              <Badge kind="neutral">{t("common.disabled")}</Badge>
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
                                {account.enabled ? t("common.disable") : t("common.enable")}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(t("accounts.deleteConfirm", { name: account.name }))) {
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
        )}
      </div>
        </>
      )}

      <Dialog
        open={dialog === "add"}
        title={tab === "opencode" ? t("accounts.addDialogTitleOc") : t("accounts.addDialogTitleOl")}
        description={
          tab === "opencode" ? t("accounts.addDialogDescOc") : t("accounts.addDialogDescOl")
        }
        onClose={() => setDialog("closed")}
        className="max-w-lg"
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onCreateSingle(e)}>
          {tab === "opencode" ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label={t("accounts.nameLabel")}
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
                label={t("accounts.nameLabel")}
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
              {t("common.cancel")}
            </Button>
            <Button type="submit" loading={busy}>
              {t("common.add")}
            </Button>
          </div>
        </form>
      </Dialog>

      <Dialog
        open={dialog === "batch"}
        title={t("accounts.batchDialogTitle")}
        description={
          tab === "opencode"
            ? t("accounts.batchDescOc")
            : t("accounts.batchDescOl")
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
            {t("common.cancel")}
          </Button>
          <Button loading={busy} onClick={() => void onBatchSubmit()}>
            {t("accounts.importLines")}
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={dialog === "import"}
        title={t("accounts.importJson")}
        description={t("accounts.importDialogDesc", { provider: tab })}
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
            {t("common.cancel")}
          </Button>
          <Button loading={busy} onClick={() => void onImportSubmit()}>
            {t("accounts.startImport")}
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
