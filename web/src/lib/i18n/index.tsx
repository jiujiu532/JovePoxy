import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { zh, type MessageKey } from "./zh";
import { en } from "./en";

export type Lang = "zh" | "en";

const STORAGE_KEY = "jovepoxy_lang";

const DICTS: Record<Lang, Record<MessageKey, string>> = { zh, en };

export function readLang(): Lang {
  if (typeof window === "undefined") return "zh";
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw === "zh" || raw === "en") return raw;
  } catch {
    /* ignore private mode */
  }
  return "zh";
}

function applyLangAttr(lang: Lang): void {
  if (typeof document !== "undefined") {
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  }
}

/** t() 的插值参数：{key} 占位符替换。 */
export type TArgs = Record<string, string | number>;

export type Translate = (key: MessageKey, args?: TArgs) => string;

type I18nContextValue = {
  readonly lang: Lang;
  readonly setLang: (lang: Lang) => void;
  readonly t: Translate;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function format(template: string, args?: TArgs): string {
  if (!args) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in args ? String(args[name]) : match,
  );
}

export function translate(lang: Lang, key: MessageKey, args?: TArgs): string {
  const hit = DICTS[lang][key] ?? zh[key] ?? key;
  return format(hit, args);
}

export function I18nProvider({ children }: { readonly children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    const initial = readLang();
    applyLangAttr(initial);
    return initial;
  });

  const setLang = useCallback((next: Lang) => {
    setLangState(next);
    applyLangAttr(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* ignore */
    }
  }, []);

  const t = useCallback<Translate>(
    (key, args) => translate(lang, key, args),
    [lang],
  );

  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within I18nProvider");
  }
  return ctx;
}

export type { MessageKey };
