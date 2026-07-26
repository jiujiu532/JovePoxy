import { cn } from "@/lib/cn";

export type SkeletonProps = {
  readonly className?: string;
};

export function Skeleton({ className }: SkeletonProps) {
  return (
    <div
      className={cn(
        "skeleton-shimmer rounded-none bg-border/70",
        className,
      )}
      aria-hidden
    />
  );
}
