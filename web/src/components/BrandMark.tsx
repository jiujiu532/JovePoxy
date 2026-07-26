import { Swap } from "@phosphor-icons/react";
import { cn } from "@/lib/cn";

export type BrandMarkProps = {
  readonly size?: number;
  readonly className?: string;
  /** Filled accent tile (login/sidebar) vs bare mark on surface. */
  readonly variant?: "tile" | "plain";
  readonly title?: string;
};

/**
 * JovePoxy mark: Phosphor Swap = 协议双向转换网关。
 * DESIGN.md 规则：Phosphor only，不手绘装饰 SVG。
 */
export function BrandMark({
  size = 40,
  className,
  variant = "tile",
  title = "JovePoxy",
}: BrandMarkProps) {
  const iconSize =
    variant === "tile" ? Math.round(size * 0.58) : size;

  const icon = (
    <Swap
      size={iconSize}
      weight="bold"
      aria-hidden={title ? undefined : true}
      role={title ? "img" : undefined}
      aria-label={title || undefined}
    />
  );

  if (variant === "plain") {
    return (
      <span
        className={cn(
          "inline-flex items-center justify-center text-accent",
          className,
        )}
        style={{ width: size, height: size }}
      >
        {icon}
      </span>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        "border-2 border-border bg-accent text-accent-fg",
        "shadow-[var(--shadow-hard)]",
        className,
      )}
      style={{ width: size, height: size }}
      aria-hidden={title ? undefined : true}
    >
      {icon}
    </span>
  );
}
