import { Eye, EyeSlash } from "@phosphor-icons/react";
import { useState } from "react";
import { TextInput, type TextInputProps } from "@/components/TextInput";
import { useI18n } from "@/lib/i18n";

export type SecretInputProps = Omit<TextInputProps, "type" | "trailing">;

export function SecretInput(props: SecretInputProps) {
  const { t } = useI18n();
  const [revealed, setRevealed] = useState(false);

  return (
    <TextInput
      {...props}
      type={revealed ? "text" : "password"}
      autoComplete={props.autoComplete ?? "current-password"}
      trailing={
        <button
          type="button"
          className="inline-flex h-8 w-8 items-center justify-center rounded-none border-2 border-transparent text-ink-muted hover:border-border hover:bg-paper-0 hover:text-ink"
          aria-label={revealed ? t("login.hidePassword") : t("login.showPassword")}
          aria-pressed={revealed}
          onClick={() => {
            setRevealed((v) => !v);
          }}
        >
          {revealed ? <EyeSlash size={18} /> : <Eye size={18} />}
        </button>
      }
    />
  );
}
