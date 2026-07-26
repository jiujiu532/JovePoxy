import { Copy, Key, PencilSimple, Plus } from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  Badge,
  Button,
  CompactField,
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
import { api, ApiError, type LocalKeyDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import {
  compareBySort,
  useRowSelection,
  type LimitFilter,
  type SortKey,
  type StatusFilter,
} from "@/lib/selection";
import { tableRowClass } from "@/lib/table-row";

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

const LIMIT_TIP =
  "后端在鉴权时按密钥原子计数：RPM 按自然分钟窗口、日上限按 UTC 自然日窗口；0 表示不限制。超限返回 429（真实生效，非展示字段）。";

const SECRET_TIP =
  "完整 sk-oc- 密钥仅在创建成功时返回一次；之后列表只显示前缀，丢失需删除后重建。";

function formatLimit(value: number): string {
  return value > 0 ? String(value) : "不限";
}

export function LocalKeysPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const [keys, setKeys] = useState<LocalKeyDTO[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [limitFilter, setLimitFilter] = useState<LimitFilter>("all");
  const [sort, setSort] = useState<SortKey>("label_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [label, setLabel] = useState("");
  const [rpm, setRpm] = useState("0");
  const [daily, setDaily] = useState("0");
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<LocalKeyDTO | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editRpm, setEditRpm] = useState("0");
  const [editDaily, setEditDaily] = useState("0");
  const [editSaving, setEditSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const res = await api.localKeys();
      setKeys(res.keys);
      setListError(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setSessionHint(false);
        void navigate("/login");
        return;
      }
      setListError(friendlyError(err, "加载密钥列表失败"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      const created = await api.createLocalKey(
        label.trim() || `key-${Date.now()}`,
        Number(rpm) || 0,
        Number(daily) || 0,
      );
      setCreatedSecret(created.secret);
      setCopied(false);
      setLabel("");
      setRpm("0");
      setDaily("0");
      setShowAdd(false);
      push("密钥已创建，请立即复制", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "创建失败"), "error");
    } finally {
      setSaving(false);
    }
  }

  async function onRevoke(id: string, name: string) {
    if (!window.confirm(`删除「${name}」后立即失效，确认？`)) return;
    try {
      await api.revokeLocalKey(id);
      push("已删除", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "删除失败"), "error");
    }
  }

  function openEdit(key: LocalKeyDTO) {
    setEditing(key);
    setEditLabel(key.label);
    setEditRpm(String(key.rpm_limit));
    setEditDaily(String(key.daily_limit));
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
      await api.updateLocalKey(
        editing.id,
        editLabel.trim(),
        Number(editRpm) || 0,
        Number(editDaily) || 0,
      );
      setEditing(null);
      push("已保存", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "保存失败"), "error");
    } finally {
      setEditSaving(false);
    }
  }

  async function onCopy() {
    if (!createdSecret) return;
    try {
      await navigator.clipboard.writeText(createdSecret);
      setCopied(true);
      push("已复制到剪贴板", "success");
    } catch {
      setCopied(false);
      push("复制失败，请手动选中复制", "error");
    }
  }

  const active = keys.filter((k) => k.enabled && !k.revoked).length;
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = keys.filter((k) => {
      if (status === "enabled" && (k.revoked || !k.enabled)) return false;
      if (status === "disabled" && (k.revoked || k.enabled)) return false;
      if (status === "revoked" && !k.revoked) return false;
      if (limitFilter === "unlimited" && (k.rpm_limit > 0 || k.daily_limit > 0)) {
        return false;
      }
      if (limitFilter === "has_rpm" && k.rpm_limit <= 0) return false;
      if (limitFilter === "has_daily" && k.daily_limit <= 0) return false;
      if (!q) return true;
      return (
        k.label.toLowerCase().includes(q) ||
        k.prefix.toLowerCase().includes(q)
      );
    });
    list.sort((a, b) =>
      compareBySort(a, b, sort, {
        label: (x) => x.label,
        statusRank: (x) => (x.revoked ? 2 : x.enabled ? 0 : 1),
      }),
    );
    return list;
  }, [keys, query, status, limitFilter, sort]);

  useEffect(() => {
    setPage(1);
  }, [query, status, limitFilter, sort]);

  const paged = useMemo(
    () => slicePage(filtered, page, pageSize),
    [filtered, page, pageSize],
  );
  const visibleIds = useMemo(() => paged.map((k) => k.id), [paged]);
  const selection = useRowSelection(visibleIds);

  async function bulkSetEnabled(next: boolean) {
    if (selection.selected.size === 0) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        const row = keys.find((k) => k.id === id);
        if (!row || row.revoked) continue;
        try {
          await api.setLocalKeyEnabled(id, next);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(next ? `已启用 ${ok} 把` : `已禁用 ${ok} 把`, ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  async function bulkRevoke() {
    if (selection.selected.size === 0) return;
    if (!window.confirm(`确认删除选中的 ${selection.selected.size} 把密钥？`)) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        const row = keys.find((k) => k.id === id);
        if (!row || row.revoked) continue;
        try {
          await api.revokeLocalKey(id);
          ok += 1;
        } catch {
          /* continue */
        }
      }
      push(`已删除 ${ok} 把`, ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="分发管理"
        description="向客户端发放本地 API Key，接入 Cursor / Claude Code。"
        meta={
          <>
            <MetaChip>{keys.length} 把</MetaChip>
            <MetaChip>可用 {active}</MetaChip>
            <HelpTip content={LIMIT_TIP} label="限流说明" />
          </>
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              刷新
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? "收起" : "发放"}
            </Button>
          </div>
        }
      />

      {createdSecret ? (
        <ComposerPanel
          title="新密钥已生成"
          description="明文仅展示一次，请立即复制保存。"
          onClose={() => {
            setCreatedSecret(null);
            setCopied(false);
          }}
          footer={
            <>
              <p className="text-[12px] text-ink-muted">
                Base URL{" "}
                <span className="font-mono text-ink">http://127.0.0.1:6446/v1</span>
              </p>
              <Button size="sm" onClick={() => void onCopy()}>
                <Copy size={14} className="mr-1" weight="bold" />
                {copied ? "已复制" : "复制密钥"}
              </Button>
            </>
          }
        >
          <div className="rounded-none border border-border bg-paper-0 px-3.5 py-3">
            <div className="mb-1.5 flex items-center gap-1 text-[11px] font-medium text-ink-muted">
              Secret
              <HelpTip content={SECRET_TIP} label="密钥可见性" />
            </div>
            <code className="block break-all font-mono text-[13px] leading-relaxed text-ink">
              {createdSecret}
            </code>
          </div>
        </ComposerPanel>
      ) : null}

      {showAdd ? (
        <ComposerPanel
          title="发放密钥"
          description="标签用于区分客户端；限流填 0 表示不限制。"
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <p className="text-[12px] text-ink-faint">生成后明文仅展示一次</p>
              <Button type="submit" form="local-key-create" size="sm" loading={saving}>
                生成密钥
              </Button>
            </>
          }
        >
          <form
            id="local-key-create"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr]"
            onSubmit={(e) => void onCreate(e)}
          >
            <CompactField label="标签" className="sm:col-span-2 lg:col-span-1">
              <input
                className={fieldInputClass}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="cursor / claude-code"
              />
            </CompactField>
            <CompactField
              label="RPM"
              tip={<HelpTip content="每分钟请求上限。0 = 不限。" />}
            >
              <input
                className={fieldInputClass}
                value={rpm}
                onChange={(e) => setRpm(e.target.value)}
                inputMode="numeric"
                placeholder="0"
              />
            </CompactField>
            <CompactField
              label="日上限"
              tip={<HelpTip content="每日请求上限（UTC）。0 = 不限。超限返回 429。" />}
            >
              <input
                className={fieldInputClass}
                value={daily}
                onChange={(e) => setDaily(e.target.value)}
                inputMode="numeric"
                placeholder="0"
              />
            </CompactField>
          </form>
        </ComposerPanel>
      ) : null}

      <SectionPanel
        title="已发放列表"
        description={`${filtered.length} / ${keys.length} 把 · 可多选批量操作`}
        bodyClassName="p-0"
      >
        <ListToolbar
          search={query}
          onSearchChange={setQuery}
          searchPlaceholder="标签 / 前缀"
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
                { value: "enabled", label: "可用" },
                { value: "disabled", label: "禁用" },
                { value: "revoked", label: "已删除" },
              ]}
            />
          }
          trailing={
            <>
              <FilterSelect
                label="限流"
                value={limitFilter}
                onChange={(v) => setLimitFilter(v as LimitFilter)}
                options={[
                  { value: "all", label: "全部" },
                  { value: "unlimited", label: "无限制" },
                  { value: "has_rpm", label: "有 RPM" },
                  { value: "has_daily", label: "有日限" },
                ]}
              />
              <FilterSelect
                label="排序"
                value={sort}
                onChange={(v) => setSort(v as SortKey)}
                options={[
                  { value: "label_asc", label: "标签 A→Z" },
                  { value: "label_desc", label: "标签 Z→A" },
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
              <DeleteButton loading={bulkBusy} onClick={() => void bulkRevoke()} />
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
            icon={Key}
            title="无法加载"
            description={listError}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                重试
              </Button>
            }
          />
        ) : keys.length === 0 ? (
          <EmptyState
            compact
            icon={Key}
            title="还没有密钥"
            description="发放一把 sk-oc- 密钥后，客户端即可调用本网关。"
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                发放
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState compact title="无匹配项" description="调整筛选或关键词。" />
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
                  {paged.map((key) => {
                    const inactive = key.revoked || !key.enabled;
                    return (
                      <MobileEntityCard
                        key={key.id}
                        muted={inactive}
                        leading={
                          <input
                            type="checkbox"
                            checked={selection.selected.has(key.id)}
                            onChange={() => selection.toggleOne(key.id)}
                            aria-label={`选择 ${key.label}`}
                          />
                        }
                        title={key.label}
                        subtitle={
                          <span className="font-mono text-[11px]">{key.prefix}</span>
                        }
                        badge={
                          key.revoked ? (
                            <Badge kind="error">已删除</Badge>
                          ) : key.enabled ? (
                            <Badge kind="healthy">可用</Badge>
                          ) : (
                            <Badge kind="neutral">禁用</Badge>
                          )
                        }
                        fields={[
                          {
                            label: "RPM / 日",
                            value: `${formatLimit(key.rpm_limit)} / ${formatLimit(key.daily_limit)}`,
                          },
                        ]}
                        actions={
                          key.revoked ? undefined : (
                            <>
                              <Button
                                variant="ghost"
                                size="sm"
                                aria-label="编辑"
                                onClick={() => openEdit(key)}
                              >
                                <PencilSimple size={14} />
                              </Button>
                              <Button
                                variant="secondary"
                                size="sm"
                                onClick={() =>
                                  void api
                                    .setLocalKeyEnabled(key.id, !key.enabled)
                                    .then(load)
                                    .catch((err) =>
                                      push(friendlyError(err, "操作失败"), "error"),
                                    )
                                }
                              >
                                {key.enabled ? "禁用" : "启用"}
                              </Button>
                              <DeleteButton
                                onClick={() => void onRevoke(key.id, key.label)}
                              />
                            </>
                          )
                        }
                      />
                    );
                  })}
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
                        <th className="whitespace-nowrap px-3 py-2 font-medium">前缀</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">
                          <span className="inline-flex items-center gap-1">
                            RPM / 日
                            <HelpTip content={LIMIT_TIP} />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">状态</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          操作
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((key) => {
                        const inactive = key.revoked || !key.enabled;
                        return (
                          <tr
                            key={key.id}
                            className={tableRowClass(inactive)}
                            aria-disabled={inactive}
                          >
                            <td className="px-3 py-2.5">
                              <input
                                type="checkbox"
                                checked={selection.selected.has(key.id)}
                                onChange={() => selection.toggleOne(key.id)}
                                aria-label={`选择 ${key.label}`}
                              />
                            </td>
                            <td
                              className={`whitespace-nowrap px-3 py-2.5 font-medium ${inactive ? "text-ink-faint" : "text-ink"}`}
                            >
                              {key.label}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                              {key.prefix}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 tabular-nums text-ink-muted">
                              {formatLimit(key.rpm_limit)} /{" "}
                              {formatLimit(key.daily_limit)}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              {key.revoked ? (
                                <Badge kind="error">已删除</Badge>
                              ) : key.enabled ? (
                                <Badge kind="healthy">可用</Badge>
                              ) : (
                                <Badge kind="neutral">禁用</Badge>
                              )}
                            </td>
                            <td className="px-3 py-2.5">
                              <div className="flex justify-end gap-1.5">
                                {key.revoked ? (
                                  <span className="text-[12px] text-ink-faint">-</span>
                                ) : (
                                  <>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      aria-label="编辑"
                                      onClick={() => openEdit(key)}
                                    >
                                      <PencilSimple size={14} />
                                    </Button>
                                    <Button
                                      variant="secondary"
                                      size="sm"
                                      onClick={() =>
                                        void api
                                          .setLocalKeyEnabled(key.id, !key.enabled)
                                          .then(load)
                                          .catch((err) =>
                                            push(
                                              friendlyError(err, "操作失败"),
                                              "error",
                                            ),
                                          )
                                      }
                                    >
                                      {key.enabled ? "禁用" : "启用"}
                                    </Button>
                                    <DeleteButton
                                      onClick={() =>
                                        void onRevoke(key.id, key.label)
                                      }
                                    />
                                  </>
                                )}
                              </div>
                            </td>
                          </tr>
                        );
                      })}
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
        title="编辑发放密钥"
        description="可改标签与限流；密钥本身不可轮换，丢失请删除后重建"
        onClose={() => setEditing(null)}
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onSaveEdit(e)}>
          <TextInput
            label="标签"
            value={editLabel}
            onChange={(e) => setEditLabel(e.target.value)}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
                RPM
                <HelpTip content="每分钟请求上限；0 不限。" />
              </label>
              <input
                className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
                value={editRpm}
                onChange={(e) => setEditRpm(e.target.value)}
                inputMode="numeric"
              />
            </div>
            <div>
              <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
                日上限
                <HelpTip content="每日请求上限（UTC 日）；0 不限。" />
              </label>
              <input
                className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
                value={editDaily}
                onChange={(e) => setEditDaily(e.target.value)}
                inputMode="numeric"
              />
            </div>
          </div>
          {editing ? (
            <p className="font-mono text-[12px] text-ink-faint">前缀 {editing.prefix}</p>
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
