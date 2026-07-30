import { useEffect, useState, type ReactNode } from "react";
import {
  ChartLine,
  ClipboardText,
  Coins,
  GearSix,
  GithubLogo,
  Globe,
  Key,
  Moon,
  SidebarSimple,
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
import { VersionBadge } from "@/components/VersionBadge";
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
  readonly theme: "light" | "dark";
  readonly onToggleTheme: () => void;
  readonly onLogout: () => void;
};

function IconTile({
  IconComp,
  active,
  size = 18,
}: {
  readonly IconComp: Icon;
  readonly active: boolean;
  readonly size?: number;
}) {
  return (
    <span
      className={cn(
        "inline-flex h-8 w-8 shrink-0 items-center justify-center border-2 border-border",
        "shadow-[2px_2px_0_var(--border)] transition-colors duration-150",
        active
          ? "bg-accent text-black"
          : "bg-paper-0 text-ink group-hover:bg-paper-1",
      )}
    >
      <IconComp size={size} weight={active ? "fill" : "regular"} aria-hidden />
    </span>
  );
}

function FooterIconButton({
  label,
  onClick,
  children,
  href,
}: {
  readonly label: string;
  readonly onClick?: () => void;
  readonly children: ReactNode;
  readonly href?: string;
}) {
  const className = cn(
    "inline-flex h-9 w-9 shrink-0 items-center justify-center",
    "border-2 border-border bg-paper-0 text-ink-muted",
    "shadow-[2px_2px_0_var(--border)]",
    "transition-[transform,background-color,color] duration-150",
    "hover:bg-paper-1 hover:text-ink active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
  );

  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        aria-label={label}
        title={label}
        className={className}
      >
        {children}
      </a>
    );
  }

  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={className}
    >
      {children}
    </button>
  );
}

export function Sidebar({
  open,
  onClose,
  theme,
  onToggleTheme,
  onLogout,
}: SidebarProps) {
  const { t, lang, setLang } = useI18n();
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
          // Mobile drawer always expanded width; desktop respects collapse.
          "w-60",
          collapsed ? "md:w-[72px]" : "md:w-60",
          open ? "translate-x-0" : "-translate-x-full md:translate-x-0",
        )}
        aria-label={t("nav.console")}
        data-collapsed={collapsed ? "true" : "false"}
      >
        {/* Brand */}
        <div
          className={cn(
            "flex h-14 shrink-0 items-center border-b-2 border-border",
            collapsed ? "justify-center px-2" : "justify-between px-3.5",
          )}
        >
          <div
            className={cn(
              "flex min-w-0 items-center",
              collapsed ? "justify-center" : "gap-2.5",
            )}
          >
            <BrandMark size={collapsed ? 36 : 32} className="rounded-none shadow-none" />
            <div className={cn("min-w-0", collapsed && "md:hidden")}>
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

        {/* Nav */}
        <nav
          className={cn(
            "min-h-0 flex-1 overflow-y-auto py-2.5",
            collapsed ? "px-2" : "px-2.5",
          )}
        >
          <div className="flex flex-col gap-3">
            {NAV_SECTIONS.map((section) => (
              <div key={section.labelKey ?? section.ids.join("-")}>
                {section.labelKey ? (
                  collapsed ? (
                    <div
                      className="mx-auto mb-1.5 hidden h-px w-6 bg-border md:block"
                      aria-hidden
                    />
                  ) : (
                    <p className="mb-1.5 px-2 text-[11px] font-medium tracking-wide text-ink-faint">
                      {t(section.labelKey)}
                    </p>
                  )
                ) : null}
                <ul className="flex flex-col gap-1.5">
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
                              "group relative flex items-center border-2 transition-colors duration-150",
                              collapsed
                                ? "justify-center px-1 py-1"
                                : "gap-2.5 px-1.5 py-1",
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
                                    "absolute left-0 top-1 bottom-1 w-1 bg-accent",
                                    collapsed && "md:hidden",
                                  )}
                                  aria-hidden
                                />
                              ) : null}
                              <IconTile IconComp={IconComp} active={isActive} />
                              <span
                                className={cn(
                                  "truncate text-[13px] font-medium",
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

        {/* Footer */}
        <div
          className={cn(
            "shrink-0 border-t-2 border-border bg-paper-1 pb-[max(0.625rem,env(safe-area-inset-bottom))]",
            collapsed ? "px-2 py-2" : "px-2.5 py-2.5",
          )}
        >
          <div
            className={cn(
              "flex gap-1.5",
              collapsed
                ? "flex-col items-center"
                : "flex-wrap items-center",
            )}
          >
            <FooterIconButton
              label={theme === "dark" ? t("shell.toLight") : t("shell.toDark")}
              onClick={onToggleTheme}
            >
              {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </FooterIconButton>
            <FooterIconButton
              label={t("shell.switchLang")}
              onClick={() => setLang(lang === "zh" ? "en" : "zh")}
            >
              <Translate size={18} />
            </FooterIconButton>
            <FooterIconButton
              label={t("shell.github")}
              href="https://github.com/jiujiu532/JovePoxy"
            >
              <GithubLogo size={18} />
            </FooterIconButton>

            {collapsed ? (
              <FooterIconButton label={t("shell.logout")} onClick={onLogout}>
                <SignOut size={18} />
              </FooterIconButton>
            ) : (
              <button
                type="button"
                onClick={onLogout}
                className={cn(
                  "inline-flex h-9 min-w-0 flex-1 items-center justify-center gap-1.5",
                  "border-2 border-border bg-paper-0 px-2 text-[12px] font-semibold text-ink",
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

            {/* Desktop collapse toggle — matches reference rail control */}
            <button
              type="button"
              className={cn(
                "hidden md:inline-flex h-9 w-9 shrink-0 items-center justify-center",
                "border-2 border-border bg-paper-0 text-ink",
                "shadow-[2px_2px_0_var(--border)]",
                "transition-[transform,background-color] duration-150",
                "hover:bg-paper-1 active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
                collapsed ? "" : "ml-auto",
              )}
              aria-label={collapsed ? t("shell.expand") : t("shell.collapse")}
              aria-pressed={collapsed}
              title={collapsed ? t("shell.expand") : t("shell.collapse")}
              onClick={toggleCollapsed}
            >
              <SidebarSimple size={18} weight={collapsed ? "fill" : "regular"} />
            </button>
          </div>

          {!collapsed ? <VersionBadge className="mt-2" /> : null}
        </div>
      </aside>
    </>
  );
}
