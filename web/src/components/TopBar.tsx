import {
  ChartLine,
  ClipboardText,
  Coins,
  GearSix,
  Globe,
  Key,
  List,
  SquaresFour,
  Stack,
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

/**
 * Fixed shell title bar: hamburger (mobile) + route icon tile + page title + version.
 * Always visible; only main content below scrolls.
 */
export function TopBar({ onMenuClick }: TopBarProps) {
  const { t } = useI18n();
  const { pathname } = useLocation();

  const routeId = routeIdForPath(pathname);
  const route = NAV_ROUTES.find((r) => r.id === routeId);
  const IconComp = routeId ? ROUTE_ICONS[routeId] : null;
  const title = route ? t(route.labelKey) : t("nav.console");

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b-2 border-border bg-paper-1 px-3 sm:px-4">
      <button
        type="button"
        className={cn(
          "inline-flex h-9 w-9 shrink-0 items-center justify-center md:hidden",
          "border-2 border-border bg-paper-0 text-ink",
          "shadow-[2px_2px_0_var(--border)]",
          "transition-[transform,background-color] duration-150",
          "hover:bg-paper-1 active:translate-x-[1px] active:translate-y-[1px] active:shadow-none",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus-ring",
        )}
        aria-label={t("shell.openNav")}
        onClick={onMenuClick}
      >
        <List size={18} weight="bold" />
      </button>

      <div className="flex min-w-0 flex-1 items-center gap-2.5">
        {IconComp ? (
          <span
            className={cn(
              "inline-flex h-9 w-9 shrink-0 items-center justify-center",
              "border-2 border-border bg-accent text-black",
              "shadow-[2px_2px_0_var(--border)]",
            )}
            aria-hidden
          >
            <IconComp size={18} weight="fill" />
          </span>
        ) : (
          <BrandMark size={32} className="rounded-none shadow-none" />
        )}
        <h1 className="truncate text-[15px] font-semibold tracking-tight text-ink sm:text-[16px]">
          {title}
        </h1>
      </div>

      <VersionBadge className="!h-9 !w-auto min-w-[4.5rem] shrink-0 px-2.5" />
    </header>
  );
}
