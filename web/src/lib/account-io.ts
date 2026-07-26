export type OpenCodeImportItem = {
  readonly name: string;
  readonly workspace_id: string;
  readonly auth_cookie: string;
  readonly enabled: boolean;
  readonly show_rolling: boolean;
  readonly show_weekly: boolean;
  readonly show_monthly: boolean;
};

export type OllamaImportItem = {
  readonly name: string;
  readonly session_cookie: string;
  readonly enabled: boolean;
  readonly show_session: boolean;
  readonly show_weekly: boolean;
};

export type OpenCodeExportBundle = {
  readonly version: 1;
  readonly provider: "opencode";
  readonly exported_at: string;
  readonly include_secrets: boolean;
  readonly accounts: ReadonlyArray<{
    readonly name: string;
    readonly workspace_id: string;
    readonly enabled: boolean;
    readonly show_rolling: boolean;
    readonly show_weekly: boolean;
    readonly show_monthly: boolean;
    readonly masked_cookie?: string;
    readonly auth_cookie?: string;
  }>;
};

export type OllamaExportBundle = {
  readonly version: 1;
  readonly provider: "ollama";
  readonly exported_at: string;
  readonly include_secrets: boolean;
  readonly accounts: ReadonlyArray<{
    readonly name: string;
    readonly enabled: boolean;
    readonly show_session: boolean;
    readonly show_weekly: boolean;
    readonly masked_cookie?: string;
    readonly session_cookie?: string;
  }>;
};

function asRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function asString(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function asBool(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function extractAccountsArray(parsed: unknown): unknown[] {
  if (Array.isArray(parsed)) {
    return parsed;
  }
  const record = asRecord(parsed);
  if (!record) {
    return [];
  }
  const accounts = record["accounts"];
  return Array.isArray(accounts) ? accounts : [];
}

export function parseOpenCodeImportJSON(raw: string): {
  readonly items: OpenCodeImportItem[];
  readonly errors: string[];
} {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return { items: [], errors: ["JSON 无法解析"] };
  }
  const record = asRecord(parsed);
  if (record?.["provider"] !== undefined && record["provider"] !== "opencode") {
    return { items: [], errors: ["provider 必须是 opencode"] };
  }
  const rows = extractAccountsArray(parsed);
  if (rows.length === 0) {
    return { items: [], errors: ["未找到 accounts 数组"] };
  }
  const items: OpenCodeImportItem[] = [];
  const errors: string[] = [];
  rows.forEach((row, index) => {
    const obj = asRecord(row);
    if (!obj) {
      errors.push(`第 ${index + 1} 条不是对象`);
      return;
    }
    const name = asString(obj["name"])?.trim() || `import-${index + 1}`;
    const workspace = asString(obj["workspace_id"])?.trim() ?? "";
    const cookie = asString(obj["auth_cookie"])?.trim() ?? "";
    if (!workspace.startsWith("wrk_") || workspace.length <= 4) {
      errors.push(`第 ${index + 1} 条 workspace_id 无效`);
      return;
    }
    if (!cookie) {
      errors.push(`第 ${index + 1} 条缺少 auth_cookie`);
      return;
    }
    items.push({
      name,
      workspace_id: workspace,
      auth_cookie: cookie,
      enabled: asBool(obj["enabled"], true),
      show_rolling: asBool(obj["show_rolling"], true),
      show_weekly: asBool(obj["show_weekly"], true),
      show_monthly: asBool(obj["show_monthly"], true),
    });
  });
  return { items, errors };
}

export function parseOllamaImportJSON(raw: string): {
  readonly items: OllamaImportItem[];
  readonly errors: string[];
} {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return { items: [], errors: ["JSON 无法解析"] };
  }
  const record = asRecord(parsed);
  if (record?.["provider"] !== undefined && record["provider"] !== "ollama") {
    return { items: [], errors: ["provider 必须是 ollama"] };
  }
  const rows = extractAccountsArray(parsed);
  if (rows.length === 0) {
    return { items: [], errors: ["未找到 accounts 数组"] };
  }
  const items: OllamaImportItem[] = [];
  const errors: string[] = [];
  rows.forEach((row, index) => {
    const obj = asRecord(row);
    if (!obj) {
      errors.push(`第 ${index + 1} 条不是对象`);
      return;
    }
    const name = asString(obj["name"])?.trim() || `ollama-${index + 1}`;
    const cookie = asString(obj["session_cookie"])?.trim() ?? "";
    if (!cookie) {
      errors.push(`第 ${index + 1} 条缺少 session_cookie`);
      return;
    }
    items.push({
      name,
      session_cookie: cookie,
      enabled: asBool(obj["enabled"], true),
      show_session: asBool(obj["show_session"], true),
      show_weekly: asBool(obj["show_weekly"], true),
    });
  });
  return { items, errors };
}

/** Multi-line batch: name|cookie or bare cookie per line. */
export function parseOpenCodeBatchLines(raw: string): {
  readonly items: OpenCodeImportItem[];
  readonly errors: string[];
} {
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#"));
  const items: OpenCodeImportItem[] = [];
  const errors: string[] = [];
  lines.forEach((line, index) => {
    const parts = line.split("|").map((p) => p.trim());
    let name = `batch-${index + 1}`;
    let workspace = "wrk_";
    let cookie = line;
    if (parts.length >= 3) {
      name = parts[0] || name;
      workspace = parts[1] || workspace;
      cookie = parts.slice(2).join("|");
    } else if (parts.length === 2) {
      name = parts[0] || name;
      cookie = parts[1] || "";
    }
    if (!cookie.includes("auth=")) {
      errors.push(`第 ${index + 1} 行需包含 auth= cookie`);
      return;
    }
    if (!workspace.startsWith("wrk_") || workspace === "wrk_") {
      errors.push(`第 ${index + 1} 行需要 workspace：名称|wrk_xxx|auth=...`);
      return;
    }
    items.push({
      name,
      workspace_id: workspace,
      auth_cookie: cookie,
      enabled: true,
      show_rolling: true,
      show_weekly: true,
      show_monthly: true,
    });
  });
  return { items, errors };
}

export function parseOllamaBatchLines(raw: string): {
  readonly items: OllamaImportItem[];
  readonly errors: string[];
} {
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#"));
  const items: OllamaImportItem[] = [];
  const errors: string[] = [];
  lines.forEach((line, index) => {
    const parts = line.split("|").map((p) => p.trim());
    let name = `ollama-${index + 1}`;
    let cookie = line;
    if (parts.length >= 2) {
      name = parts[0] || name;
      cookie = parts.slice(1).join("|");
    }
    if (!cookie) {
      errors.push(`第 ${index + 1} 行 cookie 为空`);
      return;
    }
    items.push({
      name,
      session_cookie: cookie,
      enabled: true,
      show_session: true,
      show_weekly: true,
    });
  });
  return { items, errors };
}

export function downloadJSON(filename: string, data: unknown): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}
