# JovePoxy

把 OpenCode Zen 的**免费模型**与**付费 Zen Key 池**转成 OpenAI / Anthropic 兼容 API，并提供 Claude 冷静风 + 牛皮纸质感的管理台。

## 架构要点

| 凭证 | 用途 | 是否走 chat |
|------|------|-------------|
| 本地 API Key (`sk-oc-...`) | 客户端 → 本网关 | 是（入站） |
| `Bearer public` | Zen 免费模型 | 是（出站 free） |
| Zen API Key 池 | Zen 付费模型 | 是（出站 paid） |
| OpenCode `auth_cookie` | 额度/用量抓取 | **否**（仅控制面） |
| `ADMIN_PASSWORD` | 管理台登录 | 否 |

- 模型列表动态拉取 `ZEN_BASE/models`，free 启发式：`*-free` 或 `big-pickle`
- 单进程：数据面 `/v1/*` + 管理面 `/api/admin/*` + 嵌入 SPA
- **不依赖本机 OpenCode 进程**

## 快速启动（无 Docker）

### Windows 一键脚本

| 脚本 | 用途 |
|------|------|
| `start.bat` | 启动（无 exe 时自动编译；仅用 ASCII，避免 cmd 中文乱码） |
| `stop.bat` | 结束占用 6446 端口的进程 |

双击 `start.bat`，浏览器打开 `http://127.0.0.1:6446/`。  
默认管理员密码：`admin123456`（可直接改 `start.bat` 里的两行 `set`）。

### 命令行

```bash
# 前端构建并嵌入（改过 UI 后）
make embed-web-win

go build -o bin/server.exe ./cmd/server
set ADMIN_PASSWORD=your-admin-password
set ADMIN_SECRET=至少32字符的密钥材料xxxxxxxxxxxx
set DATA_DIR=./data
set LISTEN=127.0.0.1:6446
bin\jovepoxy.exe
```

## 客户端示例

### OpenAI 兼容

```bash
curl -s http://127.0.0.1:6446/v1/models
curl -s http://127.0.0.1:6446/v1/chat/completions \
  -H "Authorization: Bearer sk-oc-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"big-pickle","messages":[{"role":"user","content":"hi"}]}'
```

### Anthropic 兼容

```bash
curl -s http://127.0.0.1:6446/v1/messages \
  -H "x-api-key: sk-oc-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"big-pickle","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}'
```

Cursor / Claude Code 将 Base URL 指到本服务，API Key 填管理台创建的**本地密钥**。

## 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `LISTEN` | `0.0.0.0:6446` | 监听地址 |
| `DATA_DIR` | `./data` | SQLite 与数据目录 |
| `ADMIN_PASSWORD` | 必填 | 管理台密码 |
| `ADMIN_SECRET` | 必填 ≥32 | AES-GCM 密钥材料 |
| `ZEN_BASE` | `https://opencode.ai/zen/v1` | Zen API 根 |
| `MODEL_CACHE_TTL` | `5m` | 模型缓存 TTL |
| `UPSTREAM_TIMEOUT` | `120s` | 上游超时 |
| `OC_VERSION` | `1.15.0` | 兼容 UA 版本号 |
| `SHOW_ALL_MODELS` | `false` | 是否展示更多模型 |
| `HTTP_PROXY` / `HTTPS_PROXY` | 空 | 上游代理 |
| `COOKIE_SECURE` | `false` | HTTPS 反代时设 `true`，管理 cookie 加 Secure |
| `SHOW_ALL_MODELS` | `false` | `true` 时 `/v1/models` 同时列出 paid 模型（chat 仍按 free/paid 分流） |

### 出口代理池（free IP 限流）

free 模型走 `Bearer public`，易被按 IP 限流。可在管理台 **出口代理池** 添加节点：

- `http://user:pass@host:8080`
- `socks5://host:1080`（本地解析 DNS）
- `socks5h://user:pass@host:1080`（远端解析 DNS，推荐）

行为：
- 未配置代理时：free 仍直连
- 配置后：free 请求经加权轮询出口
- 上游 429 / 5xx / 连接失败：冷却当前节点，并换另一个节点最多再试 1 次
- 与 **Zen 密钥池** 独立（Key = 身份，Proxy = 出口 IP）

## Docker

仓库提供 `Dockerfile` 与 `docker-compose.yml`。在已安装 Docker 的环境：

```bash
docker compose up --build -d
```

**注意：** 本项目代理工作流禁止在宿主机安装 Docker Desktop/Engine；仅交付文件。

## 安全

- 本地 Key 仅存 SHA-256；完整密钥只在创建时返回一次
- Zen Key / OpenCode cookie AES-GCM 加密存储，列表仅掩码
- 管理会话 httpOnly cookie `jovepoxy_admin`，登录限流
- 请求日志不落 prompt/response 正文

## 开发

```bash
make test    # go test
make build
cd web && pnpm test && pnpm build
```

## 许可与参考

`参考/` 目录仅作只读对照，请勿修改。
