import {
  ChartLine,
  ClipboardText,
  Coins,
  GearSix,
  GithubLogo,
  Globe,
  Key,
  Moon,
  SignOut,
  SquaresFour,
  Stack,
  Sun,
  Translate,
  UsersThree,
  X,
} from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { NavLink } from "react-router-dom";
import { BrandMark } from "@/components/BrandMark";
import { Button } from "@/components/Button";
import { VersionBadge } from "@/components/VersionBadge";
import { NAV_ROUTES, type NavRouteId } from "@/lib/routes";
import { useI18n, type MessageKey } from "@/lib/i18n";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";

const ICONS: Record<NavRouteId, Icon> = {
  overview: SquaresFour,
  models: Stack,
  "key-pool": Coins,
  accounts: UsersThree,
  quotas: ChartLine,
  "local-keys": Key,
  proxies: Globe,
  logs: ClipboardText,
  settings: GearSix,
};

type NavSection = {
  readonly labelKey: MessageKey | null;
  readonly ids: readonly NavRouteId[];
};

const NAV_SECTIONS: readonly NavSection[] = [
  { labelKey: null, ids: ["overview"] },
  {
    labelKey: "shell.navSection.data",
    ids: ["models", "key-pool", "accounts", "quotas"],
  },
  {
    labelKey: "shell.navSection.control",
    ids: ["local-keys", "proxies"],
  },
  { labelKey: "shell.navSection.observe", ids: ["logs"] },
  { labelKey: "shell.navSection.system", ids: ["settings"] },
];

function iconFor(id: NavRouteId): Icon {
  const IconComp = ICONS[id];
  if (!IconComp) {
    return assertNever(id as never);
  }
  return IconComp;
}

function routeFor(id: NavRouteId): (typeof NAV_ROUTES)[number] {
  const hit = NAV_ROUTES.find((route) => route.id === id);
  if (!hit) {
    return assertNever(id as never);
  }
  return hit;
}

export type SidebarProps = {
  readonly open: boolean;
  readonly onClose: () => void;
  readonly theme: "light" | "dark";
  readonly onToggleTheme: () => void;
  readonly onLogout: () => void;
};

export function Sidebar({
  open,
  onClose,
  theme,
  onToggleTheme,
  onLogout,
}: SidebarProps) {
  const { t, lang, setLang } = useI18n();
  return (
    <>
      <button
        type="button"
        className={cn(
          "fixed inset-0 z-40 bg-black/40 transition-opacity duration-200 md:hidden",
          open ? "opacity-100" : "pointer-events-none opacity-0",
        )}
        aria-label={t("shell.closeNav")}
        onClick={onClose}
      />
      <aside
        className={cn(
          // Mobile: fixed drawer. Desktop: in-flow column fixed to viewport height
          // so brand + logout stay visible (never stretched with page content).
          "flex h-[100dvh] max-h-[100dvh] w-60 shrink-0 flex-col overflow-hidden border-r-2 border-border bg-paper-1",
          "fixed inset-y-0 left-0 z-50 transition-transform duration-200",
          "md:static md:z-0 md:translate-x-0",
          open ? "translate-x-0" : "-translate-x-full md:translate-x-0",
        )}
        aria-label={t("nav.console")}
      >
        <div className="flex h-14 shrink-0 items-center justify-between border-b-2 border-border px-3.5">
          <div className="flex min-w-0 items-center gap-2.5">
            <BrandMark size={32} className="rounded-none shadow-none" />
            <div className="min-w-0">
              <p className="truncate text-[13px] font-semibold tracking-tight text-ink">
                JovePoxy
              </p>
              <p className="truncate text-[11px] font-medium leading-tight text-ink-faint">
                {t("shell.subtitle")}
              </p>
            </div>
          </div>
          <button
            type="button"
            className="inline-flex h-8 w-8 items-center justify-center border-2 border-transparent text-ink-muted transition-colors duration-150 hover:border-border hover:bg-paper-0 hover:text-ink md:hidden"
            aria-label={t("shell.closeNav")}
            onClick={onClose}
          >
            <X size={16} weight="bold" />
          </button>
        </div>

        <nav className="min-h-0 flex-1 overflow-y-auto px-2.5 py-2.5">
          <div className="flex flex-col gap-3.5">
            {NAV_SECTIONS.map((section) => (
              <div key={section.labelKey ?? section.ids.join("-")}>
                {section.labelKey ? (
                  <p className="mb-1.5 px-2.5 text-[11px] font-medium tracking-wide text-ink-faint">
                    {t(section.labelKey)}
                  </p>
                ) : null}
                <ul className="flex flex-col gap-1">
                  {section.ids.map((id) => {
                    const route = routeFor(id);
                    const IconComp = iconFor(id);
                    return (
                      <li key={id}>
                        <NavLink
                          to={route.path}
                          onClick={onClose}
                          className={({ isActive }) =>
                            cn(
                              "group relative flex items-center gap-2.5 border-2 px-2.5 py-1.5 text-[13px] font-medium transition-colors duration-150",
                              isActive
                                ? "border-border bg-accent text-accent-fg shadow-[3px_3px_0_var(--border)]"
                                : "border-transparent text-ink-muted hover:border-border hover:bg-paper-0 hover:text-ink",
                            )
                          }
                        >
                          {({ isActive }) => (
                            <>
                              <IconComp
                                size={18}
                                weight={isActive ? "fill" : "regular"}
                                className="shrink-0"
                              />
                              <span className="truncate">{t(route.labelKey)}</span>
                            </>
                          )}
                        </NavLink>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
        </nav>

        <div className="shrink-0 border-t-2 border-border bg-paper-1 px-2.5 py-2.5 pb-[max(0.625rem,env(safe-area-inset-bottom))]">
          <div className="flex items-center gap-1.5">
            <Button
              variant="ghost"
              size="sm"
              className="!h-9 !w-9 shrink-0 !px-0"
              aria-label={theme === "dark" ? t("shell.toLight") : t("shell.toDark")}
              onClick={onToggleTheme}
            >
              {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="!h-9 !w-9 shrink-0 !px-0"
              aria-label={t("shell.switchLang")}
              title={t("shell.switchLang")}
              onClick={() => setLang(lang === "zh" ? "en" : "zh")}
            >
              <Translate size={18} />
            </Button>
            <a
              href="https://github.com/jiujiu532/JovePoxy"
              target="_blank"
              rel="noreferrer"
              aria-label={t("shell.github")}
              className="inline-flex h-9 w-9 shrink-0 items-center justify-center border-2 border-transparent text-ink-muted transition-colors duration-150 hover:border-border hover:bg-paper-0 hover:text-ink"
            >
              <GithubLogo size={18} />
            </a>
            <Button
              variant="secondary"
              size="sm"
              className="min-w-0 flex-1 justify-start"
              onClick={onLogout}
            >
              <SignOut size={16} />
              {t("shell.logout")}
            </Button>
          </div>
          <VersionBadge className="mt-2" />
        </div>
      </aside>
    </>
  );
}
