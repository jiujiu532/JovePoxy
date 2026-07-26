import { describe, expect, it } from "vitest";
import {
  parseOllamaBatchLines,
  parseOllamaImportJSON,
  parseOpenCodeBatchLines,
  parseOpenCodeImportJSON,
} from "./account-io";

describe("parseOpenCodeImportJSON", () => {
  it("parses provider bundle", () => {
    const raw = JSON.stringify({
      version: 1,
      provider: "opencode",
      accounts: [
        {
          name: "a1",
          workspace_id: "wrk_abc123",
          auth_cookie: "auth=token",
          enabled: false,
        },
      ],
    });
    const result = parseOpenCodeImportJSON(raw);
    expect(result.errors).toEqual([]);
    expect(result.items).toHaveLength(1);
    expect(result.items[0]?.enabled).toBe(false);
    expect(result.items[0]?.workspace_id).toBe("wrk_abc123");
  });

  it("rejects wrong provider", () => {
    const result = parseOpenCodeImportJSON(
      JSON.stringify({ provider: "ollama", accounts: [{ name: "x" }] }),
    );
    expect(result.items).toHaveLength(0);
    expect(result.errors[0]).toContain("opencode");
  });
});

describe("parseOllamaImportJSON", () => {
  it("parses array of accounts", () => {
    const result = parseOllamaImportJSON(
      JSON.stringify({
        provider: "ollama",
        accounts: [{ name: "p1", session_cookie: "sess=1" }],
      }),
    );
    expect(result.errors).toEqual([]);
    expect(result.items[0]?.session_cookie).toBe("sess=1");
  });
});

describe("batch lines", () => {
  it("parses opencode name|workspace|cookie", () => {
    const result = parseOpenCodeBatchLines("main|wrk_deadbeef|auth=abc");
    expect(result.errors).toEqual([]);
    expect(result.items[0]?.name).toBe("main");
    expect(result.items[0]?.workspace_id).toBe("wrk_deadbeef");
  });

  it("parses ollama name|cookie", () => {
    const result = parseOllamaBatchLines("pro|__Secure-session=xyz");
    expect(result.errors).toEqual([]);
    expect(result.items[0]?.name).toBe("pro");
  });
});
