import { useSearchParams } from "react-router-dom";
import { isProviderTab, type ProviderTab } from "@/lib/routes";

/**
 * URL `?tab=` for opencode | ollama provider switchers.
 * Default tab omits the query param (clean URL).
 */
export function useProviderTab(
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
