import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Button } from "@/components/Button";
import { I18nProvider } from "@/lib/i18n";

describe("Button", () => {
  it("fires click when enabled", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <I18nProvider>
        <Button onClick={onClick}>提交</Button>
      </I18nProvider>,
    );
    await user.click(screen.getByRole("button", { name: "提交" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("does not fire when loading", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <I18nProvider>
        <Button loading onClick={onClick}>
          提交
        </Button>
      </I18nProvider>,
    );
    const btn = screen.getByRole("button");
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute("aria-busy", "true");
    await user.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });
});
