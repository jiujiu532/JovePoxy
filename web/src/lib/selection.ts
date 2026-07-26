import { useCallback, useEffect, useMemo, useState } from "react";

/** Row multi-select helpers for admin list tables. */
export function useRowSelection(visibleIds: readonly string[]) {
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());

  useEffect(() => {
    setSelected((prev) => {
      if (prev.size === 0) return prev;
      const allow = new Set(visibleIds);
      let changed = false;
      const next = new Set<string>();
      for (const id of prev) {
        if (allow.has(id)) next.add(id);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [visibleIds]);

  const allSelected = useMemo(
    () => visibleIds.length > 0 && visibleIds.every((id) => selected.has(id)),
    [visibleIds, selected],
  );

  const someSelected = useMemo(
    () => visibleIds.some((id) => selected.has(id)),
    [visibleIds, selected],
  );

  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      const every = visibleIds.length > 0 && visibleIds.every((id) => prev.has(id));
      return every ? new Set() : new Set(visibleIds);
    });
  }, [visibleIds]);

  const toggleOne = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const invert = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const id of visibleIds) {
        if (next.has(id)) next.delete(id);
        else next.add(id);
      }
      return next;
    });
  }, [visibleIds]);

  const clear = useCallback(() => setSelected(new Set()), []);

  const selectAllVisible = useCallback(() => setSelected(new Set(visibleIds)), [visibleIds]);

  return {
    selected,
    setSelected,
    allSelected,
    someSelected,
    toggleAll,
    toggleOne,
    invert,
    clear,
    selectAllVisible,
  } as const;
}

export type StatusFilter = "all" | "enabled" | "disabled" | "cooling" | "revoked";

export type WeightFilter = "all" | "1" | "ge2" | "ge5";
export type LimitFilter = "all" | "unlimited" | "has_rpm" | "has_daily";
export type SortKey =
  | "label_asc"
  | "label_desc"
  | "weight_desc"
  | "weight_asc"
  | "status"
  | "host_asc";

export function matchWeight(weight: number, filter: WeightFilter): boolean {
  if (filter === "all") return true;
  if (filter === "1") return weight === 1;
  if (filter === "ge2") return weight >= 2;
  return weight >= 5;
}

export function compareBySort<T>(
  a: T,
  b: T,
  sort: SortKey,
  getters: {
    label: (item: T) => string;
    weight?: (item: T) => number;
    host?: (item: T) => string;
    statusRank?: (item: T) => number;
  },
): number {
  switch (sort) {
    case "label_asc":
      return getters.label(a).localeCompare(getters.label(b), "zh");
    case "label_desc":
      return getters.label(b).localeCompare(getters.label(a), "zh");
    case "weight_desc":
      return (getters.weight?.(b) ?? 0) - (getters.weight?.(a) ?? 0);
    case "weight_asc":
      return (getters.weight?.(a) ?? 0) - (getters.weight?.(b) ?? 0);
    case "host_asc":
      return (getters.host?.(a) ?? "").localeCompare(getters.host?.(b) ?? "", "zh");
    case "status":
      return (getters.statusRank?.(a) ?? 0) - (getters.statusRank?.(b) ?? 0);
    default:
      return 0;
  }
}
