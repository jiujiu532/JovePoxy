import { Trash } from "@phosphor-icons/react";
import type { ReactNode } from "react";
import { Button } from "@/components/Button";
import { useI18n } from "@/lib/i18n";

export type DeleteButtonProps = {
  readonly onClick: () => void;
  readonly loading?: boolean;
  readonly disabled?: boolean;
  /** Visible label; defaults to common.delete. */
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
  children,
  "aria-label": ariaLabel,
  className,
}: DeleteButtonProps) {
  const { t } = useI18n();
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
      {children ?? t("common.delete")}
    </Button>
  );
}
