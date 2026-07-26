import { Trash } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { Button } from "@/components/Button";

export type DeleteButtonProps = {
  readonly onClick: () => void;
  readonly loading?: boolean;
  readonly disabled?: boolean;
  /** Visible label; defaults to 删除. */
  readonly children?: ReactNode;
  readonly "aria-label"?: string;
  readonly className?: string;
};

/**
 * Project-wide destructive row action: secondary small button with trash icon + label.
 */
export function DeleteButton({
  onClick,
  loading,
  disabled,
  children = "删除",
  "aria-label": ariaLabel,
  className,
}: DeleteButtonProps) {
  return (
    <Button
      variant="secondary"
      size="sm"
      loading={loading === true}
      disabled={disabled === true}
      onClick={onClick}
      {...(ariaLabel !== undefined ? { "aria-label": ariaLabel } : {})}
      {...(className !== undefined ? { className } : {})}
    >
      <Trash size={14} className="mr-1" weight="regular" aria-hidden />
      {children}
    </Button>
  );
}
