import { Globe, Path, PencilSimple, Plus } from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  ComposerPanel,
  DeleteButton,
  Dialog,
  EmptyState,
  FilterSelect,
  HelpTip,
  ListToolbar,
  MetaChip,
  MobileEntityCard,
  PageHeader,
  Pagination,
  ResponsiveList,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  TextInput,
  fieldInputClass,
  slicePage,
  useToast,
} from "@/components";
import { api, ApiError } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { cn } from "@/lib/cn";
import {
  compareBySort,
  matchWeight,
  useRowSelection,
  type SortKey,
  type StatusFilter,
  type WeightFilter,
} from "@/lib/selection";
import { tableRowClass } from "@/lib/table-row";

type ProxyRow = {
  id: string;
  label: string;
  scheme: string;
  host: string;
  weight: number;
  enabled: boolean;
  cooldown_until?: string;
};

function friendlyError(err: unknown, fallback: string): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return "登录已过期，请重新登录";
    return err.message || fallback;
  }
  if (err instanceof TypeError) return "无法连接服务，请确认后端已启动";
  if (err instanceof Error) {
    if (/failed to fetch|networkerror|load failed/i.test(err.message)) {
      return "无法连接服务，请确认后端已启动";
    }
    return err.message || fallback;
  }
  return fallback;
}

const BEHAVIOR_TIP =
  "未配置时 free 直连本机 IP；配置后按权重轮询出口。上游 429/5xx/连接失败会冷却节点并最多换一个重试。";

/** Parse multi-line proxy batch: `url` or `label|url` or `label|url|weight` */
function parseProxyLines(text: string): Array<{ label: string; url: string; weight: number }> {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"))
    .map((line, index) => {
      const parts = line.split("|").map((p) => p.trim());
      if (parts.length === 1) {
        return { label: `proxy-${index + 1}`, url: parts[0] ?? "", weight: 1 };
      }
      if (parts.length === 2) {
        return { label: parts[0] || `proxy-${index + 1}`, url: parts[1] ?? "", weight: 1 };
      }
      const w = Number(parts[2]);
      return {
        label: parts[0] || `proxy-${index + 1}`,
        url: parts[1] ?? "",
        weight: Number.isFinite(w) && w > 0 ? w : 1,
      };
    })
    .filter((row) => row.url.length > 0);
}

export function ProxiesPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const [rows, setRows] = useState<ProxyRow[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [scheme, setScheme] = useState("all");
  const [weightFilter, setWeightFilter] = useState<WeightFilter>("all");
  const [sort, setSort] = useState<SortKey>("label_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [batch, setBatch] = useState("");
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<ProxyRow | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editWeight, setEditWeight] = useState("1");
  const [editUrl, setEditUrl] = useState("");
  const [editSaving, setEditSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const res = await api.proxies();
      const next: ProxyRow[] = res.proxies.map((p) => {
        const row: ProxyRow = {
          id: p.id,
          label: p.label,
          scheme: p.scheme,
          host: p.host,
          weight: p.weight,
          enabled: p.enabled,
        };
        if (p.cooldown_until) row.cooldown_until = p.cooldown_until;
        return row;
      });
      setRows(next);
      setListError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setListError(friendlyError(err, "加载失败"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    const items = parseProxyLines(batch);
    if (items.length === 0) {
      push("请粘贴至少一行代理 URL", "error");
      return;
    }
    setSaving(true);
    let ok = 0;
    let fail = 0;
    try {
      for (const item of items) {
        try {
          await api.createProxy(item.label, item.url, item.weight);
          ok += 1;
        } catch {
          fail += 1;
        }
      }
      if (ok > 0) {
        setBatch("");
        setShowAdd(false);
        push(fail > 0 ? `成功 ${ok}，失败 ${fail}` : `已添加 ${ok} 个节点`, fail > 0 ? "info" : "success");
        await load();
      } else {
        push("全部添加失败，请检查 URL 格式", "error");
      }
    } finally {
      setSaving(false);
    }
  }

  function openEdit(row: ProxyRow) {
    setEditing(row);
    setEditLabel(row.label);
    setEditWeight(String(row.weight));
    setEditUrl("");
  }

  async function onSaveEdit(event: FormEvent) {
    event.preventDefault();
    if (!editing) return;
    if (!editLabel.trim()) {
      push("标签不能为空", "error");
      return;
    }
    setEditSaving(true);
    try {
      const payload: { label: string; weight: number; url?: string } = {
        label: editLabel.trim(),
        weight: Number(editWeight) || 1,
      };
      if (editUrl.trim()) payload.url = editUrl.trim();
      await api.updateProxy(editing.id, payload);
      setEditing(null);
      push("已保存", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "保存失败"), "error");
    } finally {
      setEditSaving(false);
    }
  }

  const enabled = rows.filter((r) => r.enabled).length;
  const cooling = rows.filter((r) => r.cooldown_until).length;
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = rows.filter((r) => {
      if (status === "enabled" && !r.enabled) return false;
      if (status === "disabled" && r.enabled) return false;
      if (status === "cooling" && !r.cooldown_until) return false;
      if (scheme !== "all" && r.scheme !== scheme) return false;
      if (!matchWeight(r.weight, weightFilter)) return false;
      if (!q) return true;
      return (
        r.label.toLowerCase().includes(q) ||
        r.host.toLowerCase().includes(q) ||
        r.scheme.toLowerCase().includes(q)
      );
    });
    list.sort((a, b) =>
      compareBySort(a, b, sort, {
        label: (x) => x.label,
        weight: (x) => x.weight,
        host: (x) => x.host,
        statusRank: (x) =>
          x.cooldown_until ? 2 : x.enabled ? 0 : 1,
      }),
    );
    return list;
  }, [rows, query, status, scheme, weightFilter, sort]);

  useEffect(() => {
    setPage(1);
  }, [query, status, scheme, weightFilter, sort]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );
  const visibleIds = useMemo(() => paged.map((r) => r.id), [paged]);
  const selection = useRowSelection(visibleIds);

  async function bulkSetEnabled(next: boolean) {
    if (selection.selected.size === 0) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.setProxyEnabled(id, next);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(next ? `已启用 ${ok} 个` : `已禁用 ${ok} 个`, ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  async function bulkDelete() {
    if (selection.selected.size === 0) return;
    if (!window.confirm(`确认删除选中的 ${selection.selected.size} 个节点？`)) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.deleteProxy(id);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(`已删除 ${ok} 个`, ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  const parsedCount = parseProxyLines(batch).length;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="出口代理池"
        description="Free 模型出站轮换，支持 http / socks5 / socks5h。"
        meta={
          <>
            <MetaChip>{rows.length} 节点</MetaChip>
            <MetaChip>启用 {enabled}</MetaChip>
            <MetaChip>冷却 {cooling}</MetaChip>
            <HelpTip content={BEHAVIOR_TIP} label="行为说明" />
          </>
        }
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              刷新
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? "收起" : "批量添加"}
            </Button>
          </div>
        }
      />

      {showAdd ? (
        <ComposerPanel
          title="批量添加代理"
          description="每行一个节点。支持 URL，或 label|url|weight。"
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <div className="flex flex-wrap items-center gap-1.5">
                <MetaChip>已解析 {parsedCount}</MetaChip>
                <span className="text-[11px] text-ink-faint"># 开头为注释</span>
              </div>
              <Button type="submit" form="proxy-batch-create" size="sm" loading={saving}>
                写入池
              </Button>
            </>
          }
        >
          <form id="proxy-batch-create" onSubmit={(e) => void onCreate(e)}>
            <textarea
              className={cn(
                fieldInputClass,
                "min-h-[9.5rem] resize-y py-3 font-mono text-[12px] leading-relaxed",
              )}
              value={batch}
              onChange={(e) => setBatch(e.target.value)}
              placeholder={`socks5h://user:pass@host1:1080\nhk-2|socks5h://user:pass@host2:1080|2\n# 井号开头为注释`}
              spellCheck={false}
            />
            <div className="mt-3 flex flex-wrap gap-1.5">
              {[
                "socks5h://…",
                "label|url|weight",
                "http / https",
              ].map((tip) => (
                <span
                  key={tip}
                  className="rounded-none border border-border bg-paper-0 px-2.5 py-1 text-[11px] text-ink-faint"
                >
                  {tip}
                </span>
              ))}
            </div>
          </form>
        </ComposerPanel>
      ) : null}

      <SectionPanel
        title="代理列表"
        description={`${filtered.length} / ${rows.length} · 启用 ${enabled} · 冷却 ${cooling}`}
        bodyClassName="p-0"
      >
        <ListToolbar
          search={query}
          onSearchChange={setQuery}
          searchPlaceholder="标签 / 主机 / 协议"
          selectedCount={selection.selected.size}
          totalVisible={paged.length}
          allSelected={selection.allSelected}
          onSelectAll={selection.toggleAll}
          onInvert={selection.invert}
          onClear={selection.clear}
          filters={
            <SegmentedFilter
              aria-label="状态"
              value={status}
              onChange={(v) => setStatus(v as StatusFilter)}
              options={[
                { value: "all", label: "全部" },
                { value: "enabled", label: "启用" },
                { value: "disabled", label: "禁用" },
                { value: "cooling", label: "冷却" },
              ]}
            />
          }
          trailing={
            <>
              <FilterSelect
                label="协议"
                value={scheme}
                onChange={setScheme}
                options={[
                  { value: "all", label: "全部" },
                  { value: "http", label: "http" },
                  { value: "https", label: "https" },
                  { value: "socks5", label: "socks5" },
                  { value: "socks5h", label: "socks5h" },
                ]}
              />
              <FilterSelect
                label="权重"
                value={weightFilter}
                onChange={(v) => setWeightFilter(v as WeightFilter)}
                options={[
                  { value: "all", label: "全部" },
                  { value: "1", label: "=1" },
                  { value: "ge2", label: "≥2" },
                  { value: "ge5", label: "≥5" },
                ]}
              />
              <FilterSelect
                label="排序"
                value={sort}
                onChange={(v) => setSort(v as SortKey)}
                options={[
                  { value: "label_asc", label: "标签 A→Z" },
                  { value: "host_asc", label: "主机" },
                  { value: "weight_desc", label: "权重高→低" },
                  { value: "status", label: "状态优先" },
                ]}
              />
            </>
          }
          bulkActions={
            <>
              <Button
                variant="secondary"
                size="sm"
                loading={bulkBusy}
                onClick={() => void bulkSetEnabled(true)}
              >
                启用
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={bulkBusy}
                onClick={() => void bulkSetEnabled(false)}
              >
                禁用
              </Button>
              <DeleteButton loading={bulkBusy} onClick={() => void bulkDelete()} />
            </>
          }
        />
        {loading ? (
          <div className="space-y-2 p-3">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : listError ? (
          <EmptyState
            compact
            icon={Globe}
            title="无法加载"
            description={listError}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : rows.length === 0 ? (
          <EmptyState
            compact
            icon={Globe}
            title="暂无出口"
            description="批量粘贴代理 URL，free 模型可换 IP 出站。"
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                批量添加
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState compact title="无匹配" description="调整筛选或关键词。" />
        ) : (
          <div className="min-w-0">
            <div className="flex items-center gap-2 border-b border-border bg-paper-0/40 px-3 py-2 md:hidden">
              <input
                type="checkbox"
                checked={selection.allSelected}
                ref={(el) => {
                  if (el) {
                    el.indeterminate =
                      selection.someSelected && !selection.allSelected;
                  }
                }}
                onChange={selection.toggleAll}
                aria-label="全选"
              />
              <span className="text-[12px] text-ink-muted">全选本页</span>
            </div>
            <ResponsiveList
              mobile={
                <div>
                  {paged.map((row) => (
                    <MobileEntityCard
                      key={row.id}
                      muted={!row.enabled}
                      leading={
                        <input
                          type="checkbox"
                          checked={selection.selected.has(row.id)}
                          onChange={() => selection.toggleOne(row.id)}
                          aria-label={`选择 ${row.label}`}
                        />
                      }
                      title={row.label}
                      subtitle={
                        <span className="font-mono text-[11px]">
                          {row.scheme} · {row.host}
                        </span>
                      }
                      badge={
                        row.cooldown_until ? (
                          <Badge kind="warning">冷却</Badge>
                        ) : row.enabled ? (
                          <Badge kind="healthy">启用</Badge>
                        ) : (
                          <Badge kind="neutral">禁用</Badge>
                        )
                      }
                      fields={[
                        { label: "权重", value: row.weight },
                        {
                          label: "冷却",
                          value: row.cooldown_until
                            ? new Date(row.cooldown_until).toLocaleString()
                            : "-",
                        },
                      ]}
                      actions={
                        <>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label="编辑"
                            onClick={() => openEdit(row)}
                          >
                            <PencilSimple size={14} />
                          </Button>
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() =>
                              void api
                                .setProxyEnabled(row.id, !row.enabled)
                                .then(load)
                                .catch((err) =>
                                  push(friendlyError(err, "操作失败"), "error"),
                                )
                            }
                          >
                            {row.enabled ? "禁用" : "启用"}
                          </Button>
                          <DeleteButton
                            onClick={() => {
                              if (window.confirm(`删除 ${row.label}？`)) {
                                void api
                                  .deleteProxy(row.id)
                                  .then(load)
                                  .catch((err) =>
                                    push(friendlyError(err, "删除失败"), "error"),
                                  );
                              }
                            }}
                          />
                        </>
                      }
                    />
                  ))}
                </div>
              }
              desktop={
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[42rem] text-left text-sm">
                    <thead>
                      <tr className="border-b border-border bg-paper-0/60 text-caption text-ink-muted">
                        <th className="w-10 px-3 py-2">
                          <input
                            type="checkbox"
                            checked={selection.allSelected}
                            ref={(el) => {
                              if (el) {
                                el.indeterminate =
                                  selection.someSelected && !selection.allSelected;
                              }
                            }}
                            onChange={selection.toggleAll}
                            aria-label="全选"
                          />
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">标签</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">协议</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">主机</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">
                          <span className="inline-flex items-center gap-1">
                            权重
                            <HelpTip content="启用节点按权重轮询；权重大 → 选中概率高。" />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">状态</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">冷却</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          操作
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((row) => (
                        <tr
                          key={row.id}
                          className={tableRowClass(!row.enabled)}
                          aria-disabled={!row.enabled}
                        >
                          <td className="px-3 py-2.5">
                            <input
                              type="checkbox"
                              checked={selection.selected.has(row.id)}
                              onChange={() => selection.toggleOne(row.id)}
                              aria-label={`选择 ${row.label}`}
                            />
                          </td>
                          <td
                            className={`whitespace-nowrap px-3 py-2.5 font-medium ${row.enabled ? "text-ink" : "text-ink-faint"}`}
                          >
                            {row.label}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {row.scheme}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                            {row.host}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5">
                            <span className="inline-flex items-center gap-1 tabular-nums">
                              <Path
                                size={14}
                                className={
                                  row.enabled ? "text-accent" : "text-ink-faint"
                                }
                                weight="duotone"
                              />
                              {row.weight}
                            </span>
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5">
                            {row.cooldown_until ? (
                              <Badge kind="warning">冷却</Badge>
                            ) : row.enabled ? (
                              <Badge kind="healthy">启用</Badge>
                            ) : (
                              <Badge kind="neutral">禁用</Badge>
                            )}
                          </td>
                          <td className="whitespace-nowrap px-3 py-2.5 text-[12px] text-ink-muted">
                            {row.cooldown_until
                              ? new Date(row.cooldown_until).toLocaleString()
                              : "-"}
                          </td>
                          <td className="px-3 py-2.5">
                            <div className="flex justify-end gap-1.5">
                              <Button
                                variant="ghost"
                                size="sm"
                                aria-label="编辑"
                                onClick={() => openEdit(row)}
                              >
                                <PencilSimple size={14} />
                              </Button>
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() =>
                                  void api
                                    .setProxyEnabled(row.id, !row.enabled)
                                    .then(load)
                                    .catch((err) =>
                                      push(friendlyError(err, "操作失败"), "error"),
                                    )
                                }
                              >
                                {row.enabled ? "禁用" : "启用"}
                              </Button>
                              <DeleteButton
                                onClick={() => {
                                  if (window.confirm(`删除 ${row.label}？`)) {
                                    void api
                                      .deleteProxy(row.id)
                                      .then(load)
                                      .catch((err) =>
                                        push(
                                          friendlyError(err, "删除失败"),
                                          "error",
                                        ),
                                      );
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
              }
            />
            <Pagination
              total={filtered.length}
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
      </SectionPanel>

      <Dialog
        open={editing !== null}
        title="编辑出口代理"
        description="可改标签与权重；URL 留空则保持原节点"
        onClose={() => setEditing(null)}
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onSaveEdit(e)}>
          <TextInput
            label="标签"
            value={editLabel}
            onChange={(e) => setEditLabel(e.target.value)}
          />
          <div>
            <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
              权重
              <HelpTip content="启用节点按权重轮询。" />
            </label>
            <input
              className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
              value={editWeight}
              onChange={(e) => setEditWeight(e.target.value)}
              inputMode="numeric"
            />
          </div>
          <TextInput
            label="新 URL（可选）"
            value={editUrl}
            onChange={(e) => setEditUrl(e.target.value)}
            placeholder="留空 = 不更换"
          />
          {editing ? (
            <p className="font-mono text-[12px] text-ink-faint">
              当前 {editing.scheme}://{editing.host}
            </p>
          ) : null}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" size="sm" onClick={() => setEditing(null)}>
              取消
            </Button>
            <Button type="submit" size="sm" loading={editSaving}>
              保存
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
