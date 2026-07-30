import { cn } from "@/lib/cn";

export type SkeletonProps = {
  readonly className?: string;
};

export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      className={cn(
        /* ink 透明度双模式自适应，避免 dark 下 border/70 变成惨白条 */
        "skeleton-shimmer rounded-none bg-ink/10",
        className,
      )}
      aria-hidden
    />
  );
}
