import type { ReactNode } from "react";
import {
  ChartLine,
  ClipboardText,
  Coins,
  GearSix,
  GithubLogo,
  Globe,
  Key,
  List,
  Moon,
  SquaresFour,
  Stack,
  Sun,
  Translate,
  UsersThree,
} from "@phosphor-icons/react";
import type { Icon } from "@phosphor-icons/react";
import { useLocation } from "react-router-dom";
import { BrandMark } from "@/components/BrandMark";
import { VersionBadge } from "@/components/VersionBadge";
import { useI18n } from "@/lib/i18n";
import { NAV_ROUTES, type NavRouteId } from "@/lib/routes";
import { cn } from "@/lib/cn";

export type TopBarProps = {
  readonly onMenuClick: () => void;
  readonly theme: "light" | "dark";
  readonly onToggleTheme: () => void;
};

const ROUTE_ICONS: Record<NavRouteId, Icon> = {
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

function routeIdForPath(pathname: string): NavRouteId | null {
  const hit = NAV_ROUTES.find((r) => pathname.startsWith(r.path));
  return hit?.id ?? null;
}

function HeaderIconButton({
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
    /* paper-2 抬升，避免 dark 顶栏 paper-1 上 hover 无感 */
    "hover:bg-paper-2 hover:text-ink active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
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

/**
 * Fixed shell title bar: hamburger (mobile) + enlarged route icon tile + page title + action cluster.
 * Inspired by JoveMage header hierarchy & Image #9/#10 icon button transplantation.
 */
export function TopBar({
  onMenuClick,
  theme,
  onToggleTheme,
}: TopBarProps) {
  const { t, lang, setLang } = useI18n();
  const { pathname } = useLocation();

  const routeId = routeIdForPath(pathname);
  const route = NAV_ROUTES.find((r) => r.id === routeId);
  const IconComp = routeId ? ROUTE_ICONS[routeId] : null;
  const title = route ? t(route.labelKey) : t("nav.console");

  return (
    <header className="flex h-16 shrink-0 items-center gap-3 border-b-2 border-border bg-paper-1 px-3.5 sm:px-5">
      <button
        type="button"
        className={cn(
          "inline-flex h-10 w-10 shrink-0 items-center justify-center md:hidden",
          "border-2 border-border bg-paper-0 text-ink",
          "shadow-[2px_2px_0_var(--border)]",
          "transition-[transform,background-color] duration-150",
          "hover:bg-paper-2 active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
        )}
        aria-label={t("shell.openNav")}
        onClick={onMenuClick}
      >
        <List size={20} weight="bold" />
      </button>

      {/* Brand / Route title group (enlarged for hierarchy) */}
      <div className="flex min-w-0 flex-1 items-center gap-3">
        {IconComp ? (
          <span
            className={cn(
              "inline-flex h-10 w-10 shrink-0 items-center justify-center",
              "border-2 border-border bg-accent text-black",
              "shadow-[2px_2px_0_var(--border)]",
            )}
            aria-hidden
          >
            <IconComp size={20} weight="fill" />
          </span>
        ) : (
          <BrandMark size={36} className="rounded-none shadow-none" />
        )}
        <h1 className="truncate text-[16px] font-semibold tracking-tight text-ink sm:text-[18px]">
          {title}
        </h1>
      </div>

      {/* Right action cluster: Theme, Lang, GitHub, Version (logout lives in sidebar) */}
      <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
        <HeaderIconButton
          label={theme === "dark" ? t("shell.toLight") : t("shell.toDark")}
          onClick={onToggleTheme}
        >
          {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
        </HeaderIconButton>

        <HeaderIconButton
          label={t("shell.switchLang")}
          onClick={() => setLang(lang === "zh" ? "en" : "zh")}
        >
          <Translate size={18} />
        </HeaderIconButton>

        <HeaderIconButton
          label={t("shell.github")}
          href="https://github.com/jiujiu532/JovePoxy"
        >
          <GithubLogo size={18} />
        </HeaderIconButton>

        <VersionBadge className="!h-9 !w-auto min-w-[4.5rem] shrink-0 px-2.5" />
      </div>
    </header>
  );
}
