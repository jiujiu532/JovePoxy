import { useEffect, useState } from "react";
import {
  ChartLine,
  ClipboardText,
  Coins,
  GearSix,
  Globe,
  Key,
  SidebarSimple,
  SignOut,
  SquaresFour,
  Stack,
  UsersThree,
  X,
} from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { NavLink } from "react-router-dom";
import { BrandMark } from "@/components/BrandMark";
import { NAV_ROUTES, type NavRouteId } from "@/lib/routes";
import { useI18n, type MessageKey } from "@/lib/i18n";
import { cn } from "@/lib/cn";
import { assertNever } from "@/lib/assertNever";

const COLLAPSE_KEY = "jovepoxy_sidebar_collapsed";

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

function readCollapsed(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(COLLAPSE_KEY) === "1";
  } catch {
    return false;
  }
}

export type SidebarProps = {
  readonly open: boolean;
  readonly onClose: () => void;
  readonly onLogout: () => void;
};

function IconTile({
  IconComp,
  active,
}: {
  readonly IconComp: Icon;
  readonly active: boolean;
}) {
  return (
    <span
      className={cn(
        "nav-icon-tile inline-flex h-9 w-9 shrink-0 items-center justify-center",
        "border-2 border-border bg-paper-0 text-ink",
        "shadow-[2px_2px_0_var(--border)]",
        "transition-[transform,background-color,color,box-shadow] duration-200 ease-out",
        "group-hover:-translate-y-px group-hover:shadow-[3px_3px_0_var(--border)]",
        active && "bg-accent text-black shadow-[3px_3px_0_var(--border)]",
      )}
      data-active={active ? "true" : "false"}
    >
      <IconComp
        size={20}
        weight={active ? "fill" : "duotone"}
        className={cn(
          "nav-icon-svg transition-[transform] duration-200 ease-out",
          "group-hover:scale-110",
          active && "nav-icon-pop",
        )}
        aria-hidden
      />
    </span>
  );
}

export function Sidebar({ open, onClose, onLogout }: SidebarProps) {
  const { t } = useI18n();
  const [collapsed, setCollapsed] = useState(readCollapsed);

  useEffect(() => {
    try {
      window.localStorage.setItem(COLLAPSE_KEY, collapsed ? "1" : "0");
    } catch {
      /* ignore quota / private mode */
    }
  }, [collapsed]);

  function toggleCollapsed() {
    setCollapsed((v) => !v);
  }

  return (
    <>
      <style>{`
        @keyframes nav-icon-pop {
          0% { transform: scale(0.72) rotate(-8deg); }
          55% { transform: scale(1.14) rotate(4deg); }
          100% { transform: scale(1) rotate(0deg); }
        }
        @keyframes nav-active-bar {
          0% { transform: scaleY(0); opacity: 0; }
          100% { transform: scaleY(1); opacity: 1; }
        }
        .nav-icon-pop {
          animation: nav-icon-pop 0.38s cubic-bezier(0.34, 1.4, 0.64, 1) both;
        }
        .nav-active-bar {
          transform-origin: center;
          animation: nav-active-bar 0.22s cubic-bezier(0.16, 1, 0.3, 1) both;
        }
        @media (prefers-reduced-motion: reduce) {
          .nav-icon-pop,
          .nav-active-bar {
            animation: none !important;
          }
          .nav-icon-tile,
          .nav-icon-svg {
            transition: none !important;
          }
        }
      `}</style>

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
          "flex h-[100dvh] max-h-[100dvh] shrink-0 flex-col overflow-hidden border-r-2 border-border bg-paper-1",
          "fixed inset-y-0 left-0 z-50 transition-[width,transform] duration-200",
          "md:static md:z-0 md:translate-x-0",
          "w-64",
          collapsed ? "md:w-[76px]" : "md:w-64",
          open ? "translate-x-0" : "-translate-x-full md:translate-x-0",
        )}
        aria-label={t("nav.console")}
        data-collapsed={collapsed ? "true" : "false"}
      >
        {/* Brand — h-16 aligns with TopBar rule */}
        <div
          className={cn(
            "flex h-16 shrink-0 items-center border-b-2 border-border",
            collapsed ? "justify-center px-2" : "gap-3 px-4",
          )}
        >
          <BrandMark size={40} className="shrink-0 rounded-none shadow-none" />
          <div className={cn("min-w-0", collapsed && "md:hidden")}>
            <p className="truncate text-[15px] font-semibold tracking-tight text-ink">
              JovePoxy
            </p>
            <p className="truncate text-[11px] font-medium uppercase tracking-[0.12em] text-ink-faint">
              {t("shell.subtitle")}
            </p>
          </div>
          <button
            type="button"
            className="ml-auto inline-flex h-8 w-8 items-center justify-center border-2 border-transparent text-ink-muted transition-colors duration-150 hover:border-border hover:bg-paper-0 hover:text-ink md:hidden"
            aria-label={t("shell.closeNav")}
            onClick={onClose}
          >
            <X size={16} weight="bold" />
          </button>
        </div>

        {/* Nav */}
        <nav
          className={cn(
            "min-h-0 flex-1 overflow-y-auto py-3",
            collapsed ? "px-2" : "px-3",
          )}
        >
          <div className="flex flex-col gap-4">
            {NAV_SECTIONS.map((section) => (
              <div key={section.labelKey ?? section.ids.join("-")}>
                {section.labelKey ? (
                  collapsed ? (
                    <div
                      className="mx-auto mb-2 hidden h-px w-7 bg-border md:block"
                      aria-hidden
                    />
                  ) : (
                    <p className="mb-2 px-2 text-[11px] font-semibold tracking-wide text-ink-faint">
                      {t(section.labelKey)}
                    </p>
                  )
                ) : null}
                <ul className="flex flex-col gap-2">
                  {section.ids.map((id) => {
                    const route = routeFor(id);
                    const IconComp = iconFor(id);
                    const label = t(route.labelKey);
                    return (
                      <li key={id}>
                        <NavLink
                          to={route.path}
                          onClick={onClose}
                          title={label}
                          className={({ isActive }) =>
                            cn(
                              "group relative flex items-center border-2 transition-[background-color,border-color,color,transform] duration-150",
                              collapsed
                                ? "justify-center px-1.5 py-1.5"
                                : "gap-3 px-2 py-1.5",
                              isActive
                                ? "border-border bg-accent-soft text-ink"
                                : "border-transparent text-ink-muted hover:border-border hover:bg-paper-0 hover:text-ink",
                            )
                          }
                        >
                          {({ isActive }) => (
                            <>
                              {isActive ? (
                                <span
                                  className={cn(
                                    "nav-active-bar absolute left-0 top-1.5 bottom-1.5 w-1 bg-accent",
                                    collapsed && "md:hidden",
                                  )}
                                  aria-hidden
                                />
                              ) : null}
                              <IconTile IconComp={IconComp} active={isActive} />
                              <span
                                className={cn(
                                  "truncate text-[14px] font-medium",
                                  collapsed && "md:hidden",
                                )}
                              >
                                {label}
                              </span>
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

        {/* Footer: logout + desktop collapse (JoveMage-style) */}
        <div
          className={cn(
            "shrink-0 border-t-2 border-border bg-paper-1",
            "pb-[max(0.75rem,env(safe-area-inset-bottom))]",
            collapsed ? "px-2 py-2.5" : "px-3 py-3",
          )}
        >
          <div
            className={cn(
              "flex gap-2",
              collapsed ? "flex-col items-center" : "items-center",
            )}
          >
            {collapsed ? (
              <button
                type="button"
                onClick={onLogout}
                aria-label={t("shell.logout")}
                title={t("shell.logout")}
                className={cn(
                  "inline-flex h-10 w-10 shrink-0 items-center justify-center",
                  "border-2 border-border bg-paper-0 text-ink",
                  "shadow-[2px_2px_0_var(--border)]",
                  "transition-[transform,background-color] duration-150",
                  "hover:bg-accent-yellow hover:text-black",
                  "active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
                )}
              >
                <SignOut size={18} />
              </button>
            ) : (
              <button
                type="button"
                onClick={onLogout}
                className={cn(
                  "inline-flex h-10 min-w-0 flex-1 items-center justify-center gap-2",
                  "border-2 border-border bg-paper-0 px-3 text-[13px] font-semibold text-ink",
                  "shadow-[2px_2px_0_var(--border)]",
                  "transition-[transform,background-color] duration-150",
                  "hover:bg-accent-yellow hover:text-black",
                  "active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
                )}
              >
                <SignOut size={16} aria-hidden />
                <span className="truncate">{t("shell.logout")}</span>
              </button>
            )}

            <button
              type="button"
              className={cn(
                "hidden md:inline-flex h-10 w-10 shrink-0 items-center justify-center",
                "border-2 border-border bg-paper-0 text-ink",
                "shadow-[2px_2px_0_var(--border)]",
                "transition-[transform,background-color] duration-150",
                "hover:bg-paper-1 active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
              )}
              aria-label={collapsed ? t("shell.expand") : t("shell.collapse")}
              aria-pressed={collapsed}
              title={collapsed ? t("shell.expand") : t("shell.collapse")}
              onClick={toggleCollapsed}
            >
              <SidebarSimple size={18} weight={collapsed ? "fill" : "regular"} />
            </button>
          </div>
        </div>
      </aside>
    </>
  );
}
