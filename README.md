<p align="center">
  <img src="web/public/favicon.svg" width="96" alt="JovePoxy logo">
</p>

<h1 align="center">JovePoxy</h1>

<p align="center">
  单进程网关：把 <strong>OpenCode Zen</strong> 免费模型 + 付费 Key 池，以及 <strong>Ollama Cloud</strong> Key 池，统一成 OpenAI / Anthropic 兼容 API，并内嵌 Neo-Brutalist 管理台
</p>

<p align="center">
  <a href="https://github.com/jiujiu532/JovePoxy"><img src="https://img.shields.io/github/stars/jiujiu532/JovePoxy?style=flat-square&logo=github" alt="GitHub stars"></a>
  <a href="https://github.com/jiujiu532/JovePoxy/releases"><img src="https://img.shields.io/github/v/release/jiujiu532/JovePoxy?style=flat-square&color=ff6b6b" alt="Release"></a>
  <a href="https://github.com/jiujiu532/JovePoxy/pkgs/container/jovepoxy"><img src="https://img.shields.io/badge/GHCR-jovepoxy-4ecdc4?style=flat-square&logo=docker&logoColor=white" alt="GHCR"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://react.dev/"><img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black" alt="React"></a>
  <img src="https://img.shields.io/badge/version-1.0.0-ff6b6b?style=flat-square" alt="Version">
</p>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="https://github.com/jiujiu532/JovePoxy/releases">版本发布</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#配置">配置</a>
</p>

---

## 这是什么

JovePoxy（`module jovepoxy`）是一个**单二进制**网关：

| 能力 | 说明 |
|------|------|
| 数据面 | `POST /v1/chat/completions`、`/v1/responses`、`/v1/messages`（含 SSE） |
| 模型目录 | 合并 **OpenCode Zen public** 与 **Ollama Cloud**（有健康 Ollama 池密钥时） |
| 上游 | Free → Zen `Bearer public`；Paid OpenCode → Zen Key 池；Paid Ollama → Ollama Cloud + Ollama Key 池 |
| 控制面 | `/api/admin/*` Cookie 会话 + 同端口内嵌管理台 SPA |
| 存储 | 单文件 SQLite；**不需要**本机 OpenCode / 本机 Ollama 进程 |

版本：`internal/version.Current`（默认 **1.0.0**）；`--version` 与 `/health` 输出 `jovepoxy 1.0.0`。

## 核心功能

- **三端点兼容**：OpenAI Chat Completions、OpenAI Responses、Anthropic Messages，一套本地密钥即可调用。
- **双源模型目录**：Zen 公共列表始终拉取；密钥池中存在健康 `provider=ollama` 的 API Key 时，合并 Ollama Cloud `/v1/models`。
- **按 provider 路由**：OpenCode free / OpenCode paid / Ollama paid 走不同上游与密钥池；Ollama 请求使用标准 Bearer，不加 OpenCode 兼容头。
- **本地密钥 `sk-oc-...`**：客户端只认网关密钥；支持 RPM / 日限额与并发会话；库内仅存 SHA-256。
- **密钥池 + 出口代理池**：Key = 身份，Proxy = 出口 IP，彼此独立；free 遇 429/5xx 可轮换出口重试；paid 支持 spread / sticky 与故障转移。
- **额度与用量**：OpenCode / Ollama **账号 Cookie 仅用于控制面刮取**，绝不进入聊天链路。
- **Neo-Brutalist 管理台**：概览、模型目录（来源筛选）、密钥池、账号、额度、本地密钥、代理、日志、设置；深色模式。
- **安全默认**：Zen/Ollama Key、Cookie、代理 URL 经 `ADMIN_SECRET` AES-GCM 加密；请求日志不落 prompt/response 正文。

## 请求流程

<details>
<summary><strong>聊天请求分流</strong></summary>

```mermaid
flowchart LR
    CLIENT["客户端<br/>OpenAI / Anthropic SDK"] -->|"sk-oc-... 本地密钥"| GW["JovePoxy /v1/*"]
    GW --> AUTH["本地密钥校验<br/>限流 / 并发会话"]
    AUTH --> CATALOG{"模型目录<br/>provider + free"}
    CATALOG -->|opencode free| FREE["Bearer public<br/>出口代理轮换"]
    CATALOG -->|opencode paid| PAID_OC["OpenCode Key 池<br/>+ Zen 上游"]
    CATALOG -->|ollama paid| PAID_OL["Ollama Key 池<br/>+ Ollama Cloud"]
    FREE --> ZEN["OpenCode Zen"]
    PAID_OC --> ZEN
    PAID_OL --> OLLAMA["Ollama Cloud"]
    ZEN --> RESP["响应 / SSE"]
    OLLAMA --> RESP
    RESP --> CLIENT
```

</details>

<details>
<summary><strong>控制面与数据面</strong></summary>

```mermaid
flowchart TB
    subgraph 数据面
        V1["/v1/* 聊天代理"]
        HEALTH["/health · /metrics"]
    end
    subgraph 控制面
        ADMIN["/api/admin/* 管理 API<br/>Cookie 会话"]
        SPA["内嵌管理台 SPA"]
    end
    DB[("SQLite<br/>密钥哈希 · 加密凭据 · 用量")]
    V1 --> DB
    ADMIN --> DB
    SPA --> ADMIN
```

</details>

## 快速开始

### Docker（推荐）

```bash
docker run -d --name jovepoxy \
  -p 6446:6446 \
  -e ADMIN_PASSWORD=your-password \
  -e ADMIN_SECRET=please-use-a-32-plus-char-secret \
  -v jovepoxy-data:/data \
  ghcr.io/jiujiu532/jovepoxy:1.0.0
# 或 :latest
```

打开 `http://127.0.0.1:6446/`，用 `ADMIN_PASSWORD` 登录 → 创建本地密钥 → 在密钥池按需添加 OpenCode / Ollama API Key。

### docker-compose

```yaml
services:
  jovepoxy:
    image: ghcr.io/jiujiu532/jovepoxy:1.0.0
    ports:
      - "6446:6446"
    environment:
      ADMIN_PASSWORD: your-password
      ADMIN_SECRET: please-use-a-32-plus-char-secret
      DATA_DIR: /data
      LISTEN: 0.0.0.0:6446
    volumes:
      - jovepoxy-data:/data
volumes:
  jovepoxy-data:
```

### 源码构建

```bash
# 前端
cd web && pnpm install --frozen-lockfile && pnpm build && cd ..

# 嵌入并编译（Windows 可用 make embed-web-win）
mkdir -p internal/webui/dist && cp -R web/dist/. internal/webui/dist/
go build -ldflags "-X jovepoxy/internal/version.Current=1.0.0" -o bin/jovepoxy ./cmd/server

# 运行（ADMIN_SECRET ≥ 32 字符）
export ADMIN_PASSWORD=...
export ADMIN_SECRET=...
export DATA_DIR=./data
export LISTEN=127.0.0.1:6446
./bin/jovepoxy
```

Windows 也可使用 `start.bat` / `stop.bat`（默认监听 `127.0.0.1:6446`）。

## 客户端接入

```bash
# OpenAI Chat Completions
curl http://127.0.0.1:6446/v1/chat/completions \
  -H "Authorization: Bearer sk-oc-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"big-pickle","messages":[{"role":"user","content":"hello"}]}'

# OpenAI Responses
curl http://127.0.0.1:6446/v1/responses \
  -H "Authorization: Bearer sk-oc-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"big-pickle","input":"hello"}'

# Anthropic Messages
curl http://127.0.0.1:6446/v1/messages \
  -H "x-api-key: sk-oc-..." \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5","max_tokens":256,"messages":[{"role":"user","content":"hello"}]}'
```

公开 `GET /v1/models`：默认仅 free；`SHOW_ALL_MODELS=true` 时包含 paid。`owned_by` 为 `zen`（OpenCode）或 `ollama`。

## 配置

| 环境变量 | 必填 | 默认 | 说明 |
|----------|------|------|------|
| `ADMIN_PASSWORD` | 是 | - | 管理台登录密码 |
| `ADMIN_SECRET` | 是 | - | ≥32 字符；加密池密钥 / Cookie / 代理 URL |
| `LISTEN` | 否 | `0.0.0.0:6446` | 监听地址；本机开发可设 `127.0.0.1:6446` |
| `DATA_DIR` | 否 | `./data` | SQLite 目录（容器内常用 `/data`） |
| `ZEN_BASE` | 否 | `https://opencode.ai/zen/v1` | OpenCode Zen OpenAI 兼容根路径 |
| `OLLAMA_BASE` | 否 | `https://ollama.com` | Ollama Cloud 根；进程会规范为 `…/v1` 再请求 |
| `MODEL_CACHE_TTL` | 否 | `5m` | 模型目录缓存 TTL |
| `UPSTREAM_TIMEOUT` | 否 | `120s` | 上游超时 |
| `SHOW_ALL_MODELS` | 否 | `false` | 为 `true` 时 `/v1/models` 展示全部模型 |
| `COOKIE_SECURE` | 否 | `false` | 管理台 Cookie 的 Secure 标记；HTTPS 部署建议 `true` |
| `VERSION_REPO` | 否 | `jiujiu532/JovePoxy` | 管理台版本检查对照的 GitHub 仓库 |
| `HTTP_PROXY` / `HTTPS_PROXY` | 否 | - | 进程级上游代理（与管理台「出口代理池」不同） |
| `ZEN_LOAD_POLICY` | 否 | `spread` | 付费池策略：`spread` \| `sticky`（运行时仍可在设置中改） |
| `ZEN_MAX_ATTEMPTS` | 否 | `2` | 付费故障转移尝试次数（2–4） |

## 凭据模型（切勿混用）

| 凭据 | 用途 | 进聊天链路 |
|------|------|:----------:|
| 本地密钥 `sk-oc-...` | 客户端 → 网关 | 是 |
| `Bearer public` | Zen free 出站 | 是 |
| 密钥池 `provider=opencode` | Zen paid 出站 | 是 |
| 密钥池 `provider=ollama` | Ollama Cloud 出站 + 拉 Ollama 模型目录 | 是 |
| OpenCode / Ollama **账号 Cookie** | 额度 / 用量刮取 | **否** |
| `ADMIN_PASSWORD` | 管理台登录 | 否 |

> **说明**：管理台里的「Ollama 账号 + Cookie」**不能**用来拉模型目录或聊天。要在 Models 页看到 Ollama 模型，请在**密钥池**添加启用中的 Ollama **API Key**。

## 技术栈

Go 1.25 · SQLite（modernc，无 CGO）· React 19 · Vite 7 · Tailwind CSS 4 · Phosphor Icons · Vitest · pnpm

## 开发

```bash
make test              # go test -shuffle=on -count=1 ./...
make vet
make build             # go build -o bin/jovepoxy(.exe) ./cmd/server
make embed-web-win     # 构建 web 并拷入 internal/webui/dist（Windows）
make smoke             # 本地 health / login / 建 key（不连真 Zen）

cd web
pnpm install --frozen-lockfile
pnpm dev               # :5173，代理 /api /v1 /health → :6446
pnpm test && pnpm typecheck
```

推送到 `main`（非纯文档）或打 `v*` 标签时，CI 构建并推送 **linux/amd64 + arm64** 镜像到 GHCR；镜像构建参数 `VERSION` 写入 `version.Current`。

## 许可

暂未指定许可证。使用前请遵守 OpenCode Zen 与 Ollama Cloud 的服务条款。
