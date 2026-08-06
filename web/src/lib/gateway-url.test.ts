import { describe, expect, it } from "vitest";
import { gatewayOpenAIBaseURL } from "./gateway-url";

describe("gatewayOpenAIBaseURL", () => {
  it("appends /v1 to the browser origin", () => {
    expect(gatewayOpenAIBaseURL("https://api.example.com")).toBe(
      "https://api.example.com/v1",
    );
  });

  it("strips trailing slashes on the origin", () => {
    expect(gatewayOpenAIBaseURL("https://api.example.com/")).toBe(
      "https://api.example.com/v1",
    );
    expect(gatewayOpenAIBaseURL("http://127.0.0.1:6446///")).toBe(
      "http://127.0.0.1:6446/v1",
    );
  });

  it("falls back to a relative /v1 when origin is empty", () => {
    expect(gatewayOpenAIBaseURL("")).toBe("/v1");
  });
});
