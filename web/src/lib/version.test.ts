import { describe, expect, it } from "vitest";
import { APP_VERSION } from "@/lib/version";

describe("APP_VERSION", () => {
  it("matches product release 1.5.0", () => {
    expect(APP_VERSION).toBe("1.5.0");
  });
});
