import { useCallback, useState } from "react";
import { assertNever } from "@/lib/assertNever";

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

export function viewModeLabel(mode: ViewMode): string {
  switch (mode) {
    case "grid":
      return "网格视图";
    case "compact":
      return "紧凑视图";
    case "table":
      return "表格视图";
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
