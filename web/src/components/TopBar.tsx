import { List } from "@phosphor-icons/react";
import { BrandMark } from "@/components/BrandMark";

export type TopBarProps = {
  readonly onMenuClick: () => void;
};

/** Mobile-only bar: hamburger. Desktop chrome lives in the sidebar. */
export function TopBar({ onMenuClick }: TopBarProps) {
  return (
    <header className="flex h-12 shrink-0 items-center gap-3 border-b-2 border-border bg-paper-1 px-3 md:hidden">
      <button
        type="button"
        className="inline-flex h-9 w-9 items-center justify-center border-2 border-transparent text-ink-muted hover:border-border hover:bg-paper-0 hover:text-ink"
        aria-label="打开导航"
        onClick={onMenuClick}
      >
        <List size={20} weight="bold" />
      </button>
      <BrandMark size={28} className="rounded-none shadow-none" />
      <p className="truncate text-[13px] font-semibold tracking-tight text-ink">
        JovePoxy
      </p>
    </header>
  );
}
