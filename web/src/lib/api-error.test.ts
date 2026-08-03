import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { handleUnauthorized } from "@/lib/api-error";
import { hasSessionHint, setSessionHint } from "@/lib/auth-session";

describe("handleUnauthorized", () => {
  afterEach(() => {
    setSessionHint(false);
  });

  it("clears session hint and navigates on 401", () => {
    setSessionHint(true);
    const navigate = vi.fn();
    const handled = handleUnauthorized(new ApiError(401, "expired"), navigate);
    expect(handled).toBe(true);
    expect(hasSessionHint()).toBe(false);
    expect(navigate).toHaveBeenCalledWith("/login");
  });

  it("ignores non-401 errors", () => {
    setSessionHint(true);
    const navigate = vi.fn();
    expect(handleUnauthorized(new ApiError(500, "boom"), navigate)).toBe(false);
    expect(handleUnauthorized(new Error("network"), navigate)).toBe(false);
    expect(hasSessionHint()).toBe(true);
    expect(navigate).not.toHaveBeenCalled();
  });
});
