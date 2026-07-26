import { Coins, Key, PencilSimple, Plus } from "@phosphor-icons/react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  Badge,
  Button,
  DeleteButton,
  Dialog,
  EmptyState,
  FilterSelect,
  CompactField,
  ComposerPanel,
  HelpTip,
  ListToolbar,
  MetaChip,
  MobileEntityCard,
  PageHeader,
  Pagination,
  ResponsiveList,
  SecretInput,
  SectionPanel,
  SegmentedFilter,
  Skeleton,
  Tabs,
  TextInput,
  fieldInputClass,
  slicePage,
  useToast,
} from "@/components";
import { api, ApiError, type KeyProvider, type ZenKeyDTO } from "@/lib/api";
import { setSessionHint } from "@/lib/auth-session";
import { isProviderTab, type ProviderTab } from "@/lib/routes";
import {
  compareBySort,
  matchWeight,
  useRowSelection,
  type SortKey,
  type StatusFilter,
  type WeightFilter,
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

const POLL_TIP =
  "OpenCode 付费模型按权重加权轮询健康密钥；上游 401/429/5xx 会冷却当前 key 并最多换一把重试一次。权重越大被选中概率越高。";

function useKeyProviderTab(): readonly [ProviderTab, (tab: ProviderTab) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  const tab: ProviderTab = isProviderTab(raw) ? raw : "opencode";
  function setTab(next: ProviderTab) {
    setParams(next === "opencode" ? {} : { tab: next }, { replace: true });
  }
  return [tab, setTab] as const;
}

export function KeyPoolPage() {
  const navigate = useNavigate();
  const { push } = useToast();
  const [provider, setProvider] = useKeyProviderTab();
  const [keys, setKeys] = useState<ZenKeyDTO[]>([]);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [weightFilter, setWeightFilter] = useState<WeightFilter>("all");
  const [sort, setSort] = useState<SortKey>("label_asc");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  const [label, setLabel] = useState("");
  const [secret, setSecret] = useState("");
  const [weight, setWeight] = useState("1");
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<ZenKeyDTO | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editWeight, setEditWeight] = useState("1");
  const [editSecret, setEditSecret] = useState("");
  const [editSaving, setEditSaving] = useState(false);

  const keyProvider = provider as KeyProvider;
  const providerLabel = provider === "opencode" ? "OpenCode" : "Ollama";

  async function load() {
    setLoading(true);
    try {
      const res = await api.zenKeys(keyProvider);
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
    setQuery("");
    setStatus("all");
    setWeightFilter("all");
    setSort("label_asc");
    setPage(1);
    setShowAdd(false);
    setEditing(null);
    void load();
  }, [provider]);

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    if (!secret.trim()) {
      push(`请填写 ${providerLabel} API Key`, "error");
      return;
    }
    setSaving(true);
    try {
      await api.createZenKey(
        label.trim() || `${provider}-key`,
        secret.trim(),
        Number(weight) || 1,
        keyProvider,
      );
      setLabel("");
      setSecret("");
      setWeight("1");
      setShowAdd(false);
      push("密钥已添加", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "添加失败"), "error");
    } finally {
      setSaving(false);
    }
  }

  function openEdit(key: ZenKeyDTO) {
    setEditing(key);
    setEditLabel(key.label);
    setEditWeight(String(key.weight));
    setEditSecret("");
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
      const payload: { label: string; weight: number; secret?: string } = {
        label: editLabel.trim(),
        weight: Number(editWeight) || 1,
      };
      if (editSecret.trim()) payload.secret = editSecret.trim();
      await api.updateZenKey(editing.id, payload);
      setEditing(null);
      push("已保存", "success");
      await load();
    } catch (err) {
      push(friendlyError(err, "保存失败"), "error");
    } finally {
      setEditSaving(false);
    }
  }

  const enabled = keys.filter((k) => k.enabled).length;
  const cooling = keys.filter((k) => k.cooldown_until).length;
  const totalWeight = keys.filter((k) => k.enabled && !k.cooldown_until).reduce((s, k) => s + k.weight, 0);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = keys.filter((k) => {
      if (status === "enabled" && !k.enabled) return false;
      if (status === "disabled" && k.enabled) return false;
      if (status === "cooling" && !k.cooldown_until) return false;
      if (!matchWeight(k.weight, weightFilter)) return false;
      if (!q) return true;
      return (
        k.label.toLowerCase().includes(q) ||
        k.prefix.toLowerCase().includes(q)
      );
    });
    list.sort((a, b) =>
      compareBySort(a, b, sort, {
        label: (x) => x.label,
        weight: (x) => x.weight,
        statusRank: (x) =>
          x.cooldown_until ? 2 : x.enabled ? 0 : 1,
      }),
    );
    return list;
  }, [keys, query, status, weightFilter, sort]);

  useEffect(() => {
    setPage(1);
  }, [query, status, weightFilter, sort]);

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
        try {
          await api.setZenKeyEnabled(id, next);
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

  async function bulkDelete() {
    if (selection.selected.size === 0) return;
    if (!window.confirm(`确认删除选中的 ${selection.selected.size} 把密钥？`)) return;
    setBulkBusy(true);
    let ok = 0;
    try {
      for (const id of selection.selected) {
        try {
          await api.deleteZenKey(id);
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
        title="密钥池"
        description="上游 API 密钥池。按提供商分池管理，支持加权轮询与冷却。"
        toolbar={
          <Tabs
            aria-label="提供商"
            value={provider}
            onChange={(id) => setProvider(id as ProviderTab)}
            items={[
              { id: "opencode", label: "OpenCode" },
              { id: "ollama", label: "Ollama" },
            ]}
          />
        }
        meta={
          <>
            <MetaChip>{keys.length} 把</MetaChip>
            <MetaChip>启用 {enabled}</MetaChip>
            <MetaChip>冷却 {cooling}</MetaChip>
            <MetaChip>权重 {totalWeight}</MetaChip>
            <HelpTip content={POLL_TIP} label="轮询说明" />
          </>
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              刷新
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? "收起" : "添加"}
            </Button>
          </div>
        }
      />

      {showAdd ? (
        <ComposerPanel
          title={`添加 ${providerLabel} 密钥`}
          description="完整 secret 入库后不可回显，仅保存掩码前缀。"
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <p className="text-[12px] text-ink-faint">权重越大，被轮询选中概率越高</p>
              <Button type="submit" form="key-pool-create" size="sm" loading={saving}>
                入库
              </Button>
            </>
          }
        >
          <form
            id="key-pool-create"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[1fr_1.3fr_6.5rem]"
            onSubmit={(e) => void onCreate(e)}
          >
            <CompactField label="标签">
              <input
                className={fieldInputClass}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="primary"
              />
            </CompactField>
            <CompactField label={`${providerLabel} API Key`}>
              <input
                type="password"
                className={fieldInputClass}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                autoComplete="off"
                placeholder="sk-…"
              />
            </CompactField>
            <CompactField
              label="权重"
              tip={
                <HelpTip content="加权轮询：建议主力 2–5，备用 1。" />
              }
            >
              <input
                className={fieldInputClass}
                value={weight}
                onChange={(e) => setWeight(e.target.value)}
                inputMode="numeric"
              />
            </CompactField>
          </form>
        </ComposerPanel>
      ) : null}

      <SectionPanel
        title="密钥列表"
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
                { value: "enabled", label: "启用" },
                { value: "disabled", label: "禁用" },
                { value: "cooling", label: "冷却" },
              ]}
            />
          }
          trailing={
            <>
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
                  { value: "label_desc", label: "标签 Z→A" },
                  { value: "weight_desc", label: "权重高→低" },
                  { value: "weight_asc", label: "权重低→高" },
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
            title="池是空的"
            description={
              provider === "opencode"
                ? "先添加至少一把 OpenCode 密钥，付费模型才能转发。"
                : "先添加至少一把 Ollama 上游密钥。"
            }
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                添加密钥
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
                    const healthy = key.enabled && !key.cooldown_until;
                    const share =
                      healthy && totalWeight > 0
                        ? `${Math.round((key.weight / totalWeight) * 100)}%`
                        : "-";
                    return (
                      <MobileEntityCard
                        key={key.id}
                        muted={!key.enabled}
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
                          key.cooldown_until ? (
                            <Badge kind="warning">冷却</Badge>
                          ) : key.enabled ? (
                            <Badge kind="healthy">启用</Badge>
                          ) : (
                            <Badge kind="neutral">禁用</Badge>
                          )
                        }
                        fields={[
                          { label: "权重", value: key.weight },
                          { label: "占比", value: share },
                          {
                            label: "冷却",
                            value: key.cooldown_until
                              ? new Date(key.cooldown_until).toLocaleString()
                              : "-",
                          },
                        ]}
                        actions={
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
                                  .setZenKeyEnabled(key.id, !key.enabled)
                                  .then(load)
                                  .catch((err) =>
                                    push(friendlyError(err, "操作失败"), "error"),
                                  )
                              }
                            >
                              {key.enabled ? "禁用" : "启用"}
                            </Button>
                            <DeleteButton
                              onClick={() => {
                                if (window.confirm(`删除 ${key.label}？`)) {
                                  void api
                                    .deleteZenKey(key.id)
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
                          </>
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
                            权重
                            <HelpTip content="启用且未冷却时参与加权轮询；权重大 → 选中概率高。" />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">占比</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">状态</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">冷却</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          操作
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {paged.map((key) => {
                        const healthy = key.enabled && !key.cooldown_until;
                        const share =
                          healthy && totalWeight > 0
                            ? `${Math.round((key.weight / totalWeight) * 100)}%`
                            : "-";
                        return (
                          <tr
                            key={key.id}
                            className={tableRowClass(!key.enabled)}
                            aria-disabled={!key.enabled}
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
                              className={`whitespace-nowrap px-3 py-2.5 font-medium ${key.enabled ? "text-ink" : "text-ink-faint"}`}
                            >
                              {key.label}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 font-mono text-[12px] text-ink-muted">
                              {key.prefix}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              <span
                                className={`inline-flex items-center gap-1 font-mono tabular-nums ${key.enabled ? "text-ink" : "text-ink-faint"}`}
                              >
                                <Coins
                                  size={14}
                                  className={
                                    key.enabled ? "text-accent" : "text-ink-faint"
                                  }
                                  weight="duotone"
                                />
                                {key.weight}
                              </span>
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 tabular-nums text-ink-muted">
                              {share}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              {key.cooldown_until ? (
                                <Badge kind="warning">冷却</Badge>
                              ) : key.enabled ? (
                                <Badge kind="healthy">启用</Badge>
                              ) : (
                                <Badge kind="neutral">禁用</Badge>
                              )}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5 text-[12px] text-ink-muted">
                              {key.cooldown_until
                                ? new Date(key.cooldown_until).toLocaleString()
                                : "-"}
                            </td>
                            <td className="px-3 py-2.5">
                              <div className="flex justify-end gap-1.5">
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
                                      .setZenKeyEnabled(key.id, !key.enabled)
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
                                  onClick={() => {
                                    if (window.confirm(`删除 ${key.label}？`)) {
                                      void api
                                        .deleteZenKey(key.id)
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
        title={`编辑 ${providerLabel} 密钥`}
        description="可改标签与权重；密钥留空则保持原值"
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
              <HelpTip content="加权轮询：权重大 → 被选中概率高。" />
            </label>
            <input
              className="h-10 w-full rounded-none border border-border bg-paper-0 px-3 text-sm text-ink"
              value={editWeight}
              onChange={(e) => setEditWeight(e.target.value)}
              inputMode="numeric"
            />
          </div>
          <SecretInput
            label="新密钥（可选）"
            value={editSecret}
            onChange={(e) => setEditSecret(e.target.value)}
            placeholder="留空 = 不更换"
          />
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
