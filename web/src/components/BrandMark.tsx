import { cn } from "@/lib/cn";

export type BrandMarkProps = {
  readonly size?: number;
  readonly className?: string;
  /** Filled accent tile (login/sidebar) vs bare mark on surface. */
  readonly variant?: "tile" | "plain";
  readonly title?: string;
};

/** 四芒星路径（viewBox 0 0 40 40，与品牌画廊 35-spark 同源） */
const SPARK_D =
  "M20 4l3.5 12.5L36 20l-12.5 3.5L20 36l-3.5-12.5L4 20l12.5-3.5z";

/**
 * JovePoxy mark：Neo-Brutalist 四芒星（spark）。
 * tile 变体由外层提供红底 + 硬影；plain 仅描符号。
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
    <svg
      width={iconSize}
      height={iconSize}
      viewBox="0 0 40 40"
      aria-hidden={title ? undefined : true}
      role={title ? "img" : undefined}
      aria-label={title || undefined}
    >
      <path d={SPARK_D} fill="currentColor" />
    </svg>
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
