import { useCallback, useState } from "react";
import { assertNever } from "@/lib/assertNever";
import type { MessageKey } from "@/lib/i18n/zh";

export type ViewMode = "grid" | "compact" | "table";

const STORAGE_PREFIX = "jovepoxy.viewMode.";

export function isViewMode(value: string): value is ViewMode {
  return value === "grid" || value === "compact" || value === "table";
}

export function readViewMode(storageKey: string, fallback: ViewMode = "grid"): ViewMode {
  if (typeof window === "undefined") return fallback;
  try {
    const raw = window.localStorage.getItem(`${STORAGE_PREFIX}${storageKey}`);
    if (raw && isViewMode(raw)) return raw;
  } catch {
    /* ignore quota / private mode */
  }
  return fallback;
}

export function writeViewMode(storageKey: string, mode: ViewMode): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(`${STORAGE_PREFIX}${storageKey}`, mode);
  } catch {
    /* ignore */
  }
}

/** 返回视图模式标签的 i18n key；由渲染处 t()。 */
export function viewModeLabelKey(mode: ViewMode): MessageKey {
  switch (mode) {
    case "grid":
      return "viewmode.grid";
    case "compact":
      return "viewmode.compact";
    case "table":
      return "viewmode.table";
    default:
      return assertNever(mode);
  }
}

/** 返回视图模式短标签（无"视图"/"view"后缀）的 i18n key；由渲染处 t()。 */
export function viewModeShortLabelKey(mode: ViewMode): MessageKey {
  switch (mode) {
    case "grid":
      return "viewmode.gridShort";
    case "compact":
      return "viewmode.compactShort";
    case "table":
      return "viewmode.tableShort";
    default:
      return assertNever(mode);
  }
}

export function useViewMode(
  storageKey: string,
  fallback: ViewMode = "grid",
): readonly [ViewMode, (mode: ViewMode) => void] {
  const [mode, setModeState] = useState<ViewMode>(() => readViewMode(storageKey, fallback));

  const setMode = useCallback(
    (next: ViewMode) => {
      setModeState(next);
      writeViewMode(storageKey, next);
    },
    [storageKey],
  );

  return [mode, setMode] as const;
}
