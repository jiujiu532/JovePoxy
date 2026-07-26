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
import { useI18n, type Translate } from "@/lib/i18n";
import {
  compareBySort,
  useRowSelection,
  type LimitFilter,
  type SortKey,
  type StatusFilter,
} from "@/lib/selection";
import { tableRowClass } from "@/lib/table-row";

function friendlyError(err: unknown, fallback: string, t: Translate): string {
  if (err instanceof ApiError) {
    if (err.status === 401) return t("localkeys.sessionExpired");
    return err.message || fallback;
  }
  if (err instanceof TypeError) return t("localkeys.connectFailed");
  if (err instanceof Error) {
    if (/failed to fetch|networkerror|load failed/i.test(err.message)) {
      return t("localkeys.connectFailed");
    }
    return err.message || fallback;
  }
  return fallback;
}

function formatLimit(value: number, t: Translate): string {
  return value > 0 ? String(value) : t("localkeys.unlimited");
}

export function LocalKeysPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
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
      setListError(friendlyError(err, t("localkeys.loadListFailed"), t));
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
      push(t("localkeys.secretCreated"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("common.createFailed"), t), "error");
    } finally {
      setSaving(false);
    }
  }

  async function onRevoke(id: string, name: string) {
    if (!window.confirm(t("localkeys.revokeConfirm", { name }))) return;
    try {
      await api.revokeLocalKey(id);
      push(t("localkeys.deleted"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("localkeys.revokeFailed"), t), "error");
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
      push(t("localkeys.labelRequired"), "error");
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
      push(t("localkeys.saved"), "success");
      await load();
    } catch (err) {
      push(friendlyError(err, t("localkeys.saveFailed"), t), "error");
    } finally {
      setEditSaving(false);
    }
  }

  async function onCopy() {
    if (!createdSecret) return;
    try {
      await navigator.clipboard.writeText(createdSecret);
      setCopied(true);
      push(t("localkeys.copiedToClipboard"), "success");
    } catch {
      setCopied(false);
      push(t("localkeys.copyFailed"), "error");
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
      push(
        next ? t("localkeys.bulkEnabled", { n: ok }) : t("localkeys.bulkDisabled", { n: ok }),
        ok > 0 ? "success" : "error",
      );
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  async function bulkRevoke() {
    if (selection.selected.size === 0) return;
    if (!window.confirm(t("localkeys.bulkDeleteConfirm", { n: selection.selected.size }))) return;
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
      push(t("localkeys.bulkDeleted", { n: ok }), ok > 0 ? "success" : "error");
      selection.clear();
      await load();
    } finally {
      setBulkBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={t("localkeys.title")}
        description={t("localkeys.description")}
        meta={
          <>
            <MetaChip>{t("localkeys.metaCount", { n: keys.length })}</MetaChip>
            <MetaChip>{t("localkeys.metaAvailable", { n: active })}</MetaChip>
            <HelpTip content={t("localkeys.limitTip")} label={t("localkeys.limitTipLabel")} />
          </>
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={() => void load()}>
              {t("common.refresh")}
            </Button>
            <Button size="sm" onClick={() => setShowAdd((v) => !v)}>
              <Plus size={14} className="mr-1" weight="bold" />
              {showAdd ? t("localkeys.collapse") : t("localkeys.issue")}
            </Button>
          </div>
        }
      />

      {createdSecret ? (
        <ComposerPanel
          title={t("localkeys.newSecretTitle")}
          description={t("localkeys.newSecretDesc")}
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
                {copied ? t("common.copied") : t("localkeys.copySecret")}
              </Button>
            </>
          }
        >
          <div className="rounded-none border border-border bg-paper-0 px-3.5 py-3">
            <div className="mb-1.5 flex items-center gap-1 text-[11px] font-medium text-ink-muted">
              Secret
              <HelpTip content={t("localkeys.secretTip")} label={t("localkeys.secretTipLabel")} />
            </div>
            <code className="block break-all font-mono text-[13px] leading-relaxed text-ink">
              {createdSecret}
            </code>
          </div>
        </ComposerPanel>
      ) : null}

      {showAdd ? (
        <ComposerPanel
          title={t("localkeys.issueTitle")}
          description={t("localkeys.issueDesc")}
          onClose={() => setShowAdd(false)}
          footer={
            <>
              <p className="text-[12px] text-ink-faint">{t("localkeys.issueFooterHint")}</p>
              <Button type="submit" form="local-key-create" size="sm" loading={saving}>
                {t("localkeys.generate")}
              </Button>
            </>
          }
        >
          <form
            id="local-key-create"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr]"
            onSubmit={(e) => void onCreate(e)}
          >
            <CompactField label={t("localkeys.labelField")} className="sm:col-span-2 lg:col-span-1">
              <input
                className={fieldInputClass}
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder="cursor / claude-code"
              />
            </CompactField>
            <CompactField
              label="RPM"
              tip={<HelpTip content={t("localkeys.rpmTip")} />}
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
              label={t("localkeys.dailyField")}
              tip={<HelpTip content={t("localkeys.dailyTip")} />}
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
        title={t("localkeys.listTitle")}
        description={t("localkeys.listDesc", { filtered: filtered.length, total: keys.length })}
        bodyClassName="p-0"
      >
        <ListToolbar
          search={query}
          onSearchChange={setQuery}
          searchPlaceholder={t("localkeys.searchPlaceholder")}
          selectedCount={selection.selected.size}
          totalVisible={paged.length}
          allSelected={selection.allSelected}
          onSelectAll={selection.toggleAll}
          onInvert={selection.invert}
          onClear={selection.clear}
          filters={
            <SegmentedFilter
              aria-label={t("localkeys.statusAria")}
              value={status}
              onChange={(v) => setStatus(v as StatusFilter)}
              options={[
                { value: "all", label: t("common.all") },
                { value: "enabled", label: t("localkeys.statusAvailable") },
                { value: "disabled", label: t("common.disabled") },
                { value: "revoked", label: t("localkeys.statusRevoked") },
              ]}
            />
          }
          trailing={
            <>
              <FilterSelect
                label={t("localkeys.limitFilterLabel")}
                value={limitFilter}
                onChange={(v) => setLimitFilter(v as LimitFilter)}
                options={[
                  { value: "all", label: t("common.all") },
                  { value: "unlimited", label: t("localkeys.limitUnlimited") },
                  { value: "has_rpm", label: t("localkeys.limitHasRpm") },
                  { value: "has_daily", label: t("localkeys.limitHasDaily") },
                ]}
              />
              <FilterSelect
                label={t("localkeys.sortLabel")}
                value={sort}
                onChange={(v) => setSort(v as SortKey)}
                options={[
                  { value: "label_asc", label: t("localkeys.sortLabelAsc") },
                  { value: "label_desc", label: t("localkeys.sortLabelDesc") },
                  { value: "status", label: t("localkeys.sortStatus") },
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
                {t("common.enable")}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                loading={bulkBusy}
                onClick={() => void bulkSetEnabled(false)}
              >
                {t("common.disable")}
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
            title={t("localkeys.loadFailedTitle")}
            description={listError}
            action={
              <Button variant="secondary" size="sm" onClick={() => void load()}>
                {t("common.retry")}
              </Button>
            }
          />
        ) : keys.length === 0 ? (
          <EmptyState
            compact
            icon={Key}
            title={t("localkeys.emptyTitle")}
            description={t("localkeys.emptyDesc")}
            action={
              <Button size="sm" onClick={() => setShowAdd(true)}>
                <Plus size={14} className="mr-1" />
                {t("localkeys.issue")}
              </Button>
            }
          />
        ) : filtered.length === 0 ? (
          <EmptyState compact title={t("localkeys.noMatchTitle")} description={t("localkeys.noMatchDesc")} />
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
                aria-label={t("localkeys.selectAllAria")}
              />
              <span className="text-[12px] text-ink-muted">{t("localkeys.selectAllPage")}</span>
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
                            aria-label={t("localkeys.selectRowAria", { label: key.label })}
                          />
                        }
                        title={key.label}
                        subtitle={
                          <span className="font-mono text-[11px]">{key.prefix}</span>
                        }
                        badge={
                          key.revoked ? (
                            <Badge kind="error">{t("localkeys.statusRevoked")}</Badge>
                          ) : key.enabled ? (
                            <Badge kind="healthy">{t("localkeys.statusAvailable")}</Badge>
                          ) : (
                            <Badge kind="neutral">{t("common.disabled")}</Badge>
                          )
                        }
                        fields={[
                          {
                            label: t("localkeys.rpmDailyLabel"),
                            value: `${formatLimit(key.rpm_limit, t)} / ${formatLimit(key.daily_limit, t)}`,
                          },
                        ]}
                        actions={
                          key.revoked ? undefined : (
                            <>
                              <Button
                                variant="ghost"
                                size="sm"
                                aria-label={t("localkeys.editAria")}
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
                                      push(friendlyError(err, t("common.actionFailed"), t), "error"),
                                    )
                                }
                              >
                                {key.enabled ? t("common.disable") : t("common.enable")}
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
                            aria-label={t("localkeys.selectAllAria")}
                          />
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("localkeys.colLabel")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("localkeys.colPrefix")}</th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">
                          <span className="inline-flex items-center gap-1">
                            {t("localkeys.rpmDailyLabel")}
                            <HelpTip content={t("localkeys.limitTip")} />
                          </span>
                        </th>
                        <th className="whitespace-nowrap px-3 py-2 font-medium">{t("localkeys.colStatus")}</th>
                        <th className="whitespace-nowrap px-3 py-2 text-right font-medium">
                          {t("localkeys.colActions")}
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
                                aria-label={t("localkeys.selectRowAria", { label: key.label })}
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
                              {formatLimit(key.rpm_limit, t)} /{" "}
                              {formatLimit(key.daily_limit, t)}
                            </td>
                            <td className="whitespace-nowrap px-3 py-2.5">
                              {key.revoked ? (
                                <Badge kind="error">{t("localkeys.statusRevoked")}</Badge>
                              ) : key.enabled ? (
                                <Badge kind="healthy">{t("localkeys.statusAvailable")}</Badge>
                              ) : (
                                <Badge kind="neutral">{t("common.disabled")}</Badge>
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
                                      aria-label={t("localkeys.editAria")}
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
                                              friendlyError(err, t("common.actionFailed"), t),
                                              "error",
                                            ),
                                          )
                                      }
                                    >
                                      {key.enabled ? t("common.disable") : t("common.enable")}
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
        title={t("localkeys.editDialogTitle")}
        description={t("localkeys.editDialogDesc")}
        onClose={() => setEditing(null)}
      >
        <form className="flex flex-col gap-3" onSubmit={(e) => void onSaveEdit(e)}>
          <TextInput
            label={t("localkeys.labelField")}
            value={editLabel}
            onChange={(e) => setEditLabel(e.target.value)}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="mb-1 flex items-center gap-1 text-caption text-ink-muted">
                RPM
                <HelpTip content={t("localkeys.rpmTipShort")} />
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
                {t("localkeys.dailyField")}
                <HelpTip content={t("localkeys.dailyTipShort")} />
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
            <p className="font-mono text-[12px] text-ink-faint">{t("localkeys.prefixLabel", { prefix: editing.prefix })}</p>
          ) : null}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" size="sm" onClick={() => setEditing(null)}>
              {t("common.cancel")}
            </Button>
            <Button type="submit" size="sm" loading={editSaving}>
              {t("common.save")}
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
