# JovePoxy Admin Design System

## Design Read

Reading this as: **运维/技术操作者的产品管理台重设计**（非营销落地页），风格为 **Neo-Brutalist Playful（俏皮野兽派）**。  
**Dials:** `DESIGN_VARIANCE: 7` / `MOTION_INTENSITY: 4` / `VISUAL_DENSITY: 7`  
登录页可略降密度、略增趣味（`VARIANCE 8` / `MOTION 5` / `DENSITY 5`），仍共用同一套 token。

**Signature:** 纯黑纯白结构 + 五色硬强调 + 直角硬边 + 硬 offset 阴影 + Toy Spring / Joyful Press 微交互。  
**Sources:** Stylekit Neo-Brutalist Playful showcase + 项目 `参考/neo-brutalist-playful-design-spec.md`（只读）+ 本仓库 token 实现（`web/src/index.css`）。

**不克隆营销站信息架构**；dense 表格/表单区域禁止倾斜与彩色 ping-pong 阴影，保证扫读。

---

## 1. Atmosphere & Identity

硬边、高对比、俏皮但不喧宾夺主的操作台。主结构永远是黑/白几何块；彩色只用于 CTA、状态徽章、硬阴影 accent 与少量装饰几何。无 kraft 纸纹、无柔和暖灰堆叠、无模糊 elevation。

---

## 2. Color

### 2.1 结构色 + 强调色板

| 角色 | Token | Light | Dark（Slate Ops） | 用途 |
|------|-------|-------|------|------|
| App bg | `--paper-0` | `#ffffff` | `#12161c` | 应用背景（冷灰蓝 canvas） |
| Surface | `--paper-1` | `#ffffff` | `#1b222c` | 卡片、面板、顶栏/侧栏 |
| Elevated | `--paper-2` | `#ffffff` | `#273140` | 模态、弹出层、hover 抬升 |
| Ink primary | `--ink` | `#000000` | `#eef3f8` | 标题、正文（冷浅蓝白） |
| Ink muted | `--ink-muted` | `#1a1a1a` | `#b7c3d1` | 次级标签（高对比，禁止泥灰） |
| Ink faint | `--ink-faint` | `#333333` | `#8492a6` | 占位、分组标题 |
| Border | `--border` | `#000000` | `#c8d2de` | 硬边框（冷银纸边） |
| Border strong | `--border-strong` | `#000000` | `#dae4f0` | 强调/选中边框 |
| Accent primary | `--accent` | `#ff6b6b` | `#ff7b7b` | 主 CTA（Playful Red，dark 略提亮） |
| Accent hover | `--accent-hover` | `#e85a5a` | `#ff9191` | dark 用更亮纯色反馈 |
| Accent fg | `--accent-fg` | `#000000` | `#000000` | 强调色上的文字（主 CTA 黑字） |
| Accent soft | `--accent-soft` | `#ffe0e0` | `#3a1d24` | 浅/深纯色块（冷砖红 soft，禁止死灰） |
| Success | `--status-success` | `#4ecdc4` | `#5fd4cc` | 健康 / teal 倾向 |
| Warning | `--status-warning` | `#ffe66d` | `#ffe27a` | 警告 / 黄 |
| Error | `--status-error` | `#ff6b6b` | `#ff7b7b` | 错误 / playful red |
| Info | `--status-info` | `#95e1d3` | `#9fe0d4` | 信息 / mint |
| Focus ring | `--focus-ring` | `#ffe66d` | `#ffe27a` | 键盘焦点（黄，高对比） |

### 2.2 五色强调（Playful Accents）

| 名称 | Token | Hex | 典型用途 |
|------|-------|-----|----------|
| Playful Red | `--accent` | `#ff6b6b` | 主 CTA、error、红硬阴影 |
| Vibrant Teal | `--accent-teal` | `#4ecdc4` | 徽章、成功、青硬阴影 |
| Bold Yellow | `--accent-yellow` | `#ffe66d` | 高亮、focus、黄硬阴影 |
| Soft Mint | `--accent-mint` | `#95e1d3` | 信息、浅装饰块 |
| Coral Pink | `--accent-coral` | `#f38181` | 标签、次强调 |

Primary / Secondary 结构色：`#000000` / `#ffffff`。

### 2.3 固定色值（source of truth）

```
--paper-0:          #ffffff / #12161c
--paper-1:          #ffffff / #1b222c
--paper-2:          #ffffff / #273140
--ink:              #000000 / #eef3f8
--ink-muted:        #1a1a1a / #b7c3d1
--ink-faint:        #333333 / #8492a6
--border:           #000000 / #c8d2de
--border-strong:    #000000 / #dae4f0
--accent:           #ff6b6b / #ff7b7b
--accent-hover:     #e85a5a / #ff9191
--accent-fg:        #000000 / #000000
--accent-soft:      #ffe0e0 / #3a1d24
--accent-teal:      #4ecdc4 / #5fd4cc
--accent-yellow:    #ffe66d / #ffe27a
--accent-mint:      #95e1d3 / #9fe0d4
--accent-coral:     #f38181 / #ff8f8f
--status-success:   #4ecdc4 / #5fd4cc
--status-warning:   #ffe66d / #ffe27a
--status-error:     #ff6b6b / #ff7b7b
--status-info:      #95e1d3 / #9fe0d4
--focus-ring:       #ffe66d / #ffe27a
--shadow-hard:      4px 4px 0 #000000 / 4px 4px 0 #c8d2de
--shadow-paper:     同 --shadow-hard（兼容旧类名）
--shadow-accent-red:    4px 4px 0 #ff6b6b / #ff7b7b
--shadow-accent-teal:   4px 4px 0 #4ecdc4 / #5fd4cc
--shadow-accent-yellow: 4px 4px 0 #ffe66d / #ffe27a
--ease-toy-spring:  cubic-bezier(0.34, 1.56, 0.64, 1)
```

### 2.4 颜色规则

- 黑/白为结构色；五强调色为交互与装饰。
- 禁止柔和灰色正文/边框；次级文字必须高对比深色/浅色。
- 禁止渐变、禁止 `color-mix` 做「模糊 soft tint」；soft 仅用纯色块（如 light `#ffe0e0` / dark `#4a221c` 咖啡红块，**禁止** `#222` 死灰 soft）。
- Dark 必须保留 **paper-0 < paper-1 < paper-2** 三层 elevation；禁止把三层都压成近纯黑或中性炭灰。
- Dark 基调为 **Espresso Night**（暖褐/焦糖）；硬边/硬阴影用奶油纸边 `#c8d2de`，避免纯 `#ffffff` chalk outline。
- 状态徽章：硬黑/白边 + 纯色填充，不靠圆角区分。
- 新增 hex 必须先扩展本表再使用。

---

## 3. Typography

### Font stack

- **UI / body:** `"Geist Variable", "Geist", "Segoe UI", system-ui, sans-serif`
- **Mono:** `"Geist Mono Variable", "Geist Mono", "Cascadia Code", ui-monospace, monospace`
- 不引入 Inter 默认、不引入装饰衬线。

### Scale

| Level | Token | Size | Weight | Line | Usage |
|-------|-------|------|--------|------|-------|
| Display | `--text-display` | 32px | 700 | 1.2 | Login 标题 |
| H1 | `--text-h1` | 24px | 700 | 1.25 | 页面标题 |
| H2 | `--text-h2` | 18px | 700 | 1.3 | 区块 / 卡片标题 |
| Body | `--text-body` | 14px | 400 | 1.5 | 默认正文 |
| Body sm | `--text-sm` | 13px | 400 | 1.45 | 表格、元信息 |
| Caption | `--text-caption` | 12px | 600 | 1.4 | 标签、徽章 |
| Mono | `--text-mono` | 13px | 400 | 1.45 | 密钥、模型 id |

### Rules

- 长文不低于 13px；caption 12px 可用。
- 产品文案中文；技术 ID 保持英文。
- 可见文案禁止 emoji / Unicode 装饰符号。

---

## 4. Spacing & Layout

### Base unit: 4px

| Token | Value | Usage |
|-------|-------|-------|
| `--space-1` | 4px | 图标间距 |
| `--space-2` | 8px | 紧凑行内 |
| `--space-3` | 12px | 输入 pad、密组 |
| `--space-4` | 16px | 卡片默认 padding |
| `--space-5` | 20px | 区块内 |
| `--space-6` | 24px | 面板 padding |
| `--space-8` | 32px | 区块间距 |
| `--space-10` | 40px | 页头间距 |
| `--space-12` | 48px | Login 纵轴节奏 |

### Shape rule（锁定：全 0）

| Element | Radius token | Value |
|---------|--------------|-------|
| Cards / dialogs | `--radius-card` | **0** |
| Inputs / controls | `--radius-input` | **0** |
| Buttons / pills | `--radius-pill` | **0** |
| Badges | `--radius-badge` | **0** |

全部等价 `rounded-none`。禁止 `rounded-sm` 及以上、禁止 pill。

### 边框

| 场景 | 规范 |
|------|------|
| 卡片 / 主按钮 / Dialog | `border-4` + black（dark 为 white） |
| Dense 输入 / 表格控件 | 可用 `border-2` 硬黑边，避免 4px 撑破高度 |
| 分割线 | 硬色边，禁止柔灰 hairline |

### Grid & shell

- 主内容最大宽：`1280px`
- 侧栏桌面：`240px`；`< md` 抽屉
- 顶栏高度：`56px`（≤ 80px）
- 断点：sm 640 / md 768 / lg 1024 / xl 1280
- 触控目标 ≥ 44px（主操作）

### Depth strategy：**hard offset only**

| Token | Light | Dark |
|-------|-------|------|
| `--shadow-hard` / `--shadow-paper` | `4px 4px 0 #000` | `4px 4px 0 #c8d2de` |
| `--shadow-accent-red` | `4px 4px 0 #ff6b6b` | `4px 4px 0 #ff7a6e` |
| `--shadow-accent-teal` | `4px 4px 0 #4ecdc4` | `4px 4px 0 #6fd9c8` |
| `--shadow-accent-yellow` | `4px 4px 0 #ffe66d` | `4px 4px 0 #ffd978` |

- **禁止**模糊阴影、`backdrop-blur`、多层 soft elevation。
- Dark 默认硬阴影用奶油纸边 offset（对齐 `--border` / Espresso Night）；彩色 shadow 仅 accent-heavy 点缀。
- 已废弃 kraft **Grain**：`.paper-grain` 空规则保留以免旧引用报错，视觉无效果。

---

## 5. Motion & Interaction（四规则 + 管理台矩阵）

### 5.1 四规则（Stylekit）

| 规则 | 定义 |
|------|------|
| **Toy Spring** | `cubic-bezier(0.34, 1.56, 0.64, 1)`（token `--ease-toy-spring`） |
| **Tilt** | 旋转 ≤ **3°**（品牌块 / 登录点缀 / 空状态几何；dense 区禁止） |
| **Color Ping-Pong** | 硬阴影色循环 Red → Teal → Yellow；仅 Brand / 主 CTA / 登录装饰 |
| **Joyful Press** | `active`: 阴影归零 + `translate` 对齐 offset + `scale(0.95)` |

Joyful Press 目标态示例：`active: translate(4px, 4px) shadow-none scale-95`。

### 5.2 动效矩阵

| 规则 | 按钮/控件 | Login / 空状态 | Dense table 行 |
|------|-----------|----------------|----------------|
| Toy Spring | 是 | 是 | **否** |
| Joyful Press | 是 | 是 | 仅图标按钮 |
| Tilt ≤3° | 慎用 | 是 | **禁止** |
| Color Ping-Pong | 主 CTA 可选 | 是 | **禁止** |
| Hover scale-105 | 按钮/图标 | 是 | **整行禁止** |
| reduced-motion | 关弹簧/位移 | 同 | 同 |

### 5.3 时长

| Type | Duration | Easing | Usage |
|------|----------|--------|-------|
| Micro | 120–180ms | toy-spring 或 ease-out | 按钮 press / hover |
| Standard | 200–250ms | ease-in-out | Drawer、Dialog、Tab |

### 5.4 Motion 规则

- 优先 `transform` + `opacity`（+ 交互色切换）
- 无 scroll-hijack、无跑马灯
- `prefers-reduced-motion: reduce` → 禁用弹簧、位移、shimmer；过渡近瞬时

---

## 6. Components（视觉合同摘要）

公共 props API（variant/size 等）保持不变，只换外观。

| 组件 | 要点 |
|------|------|
| **Button** | 直角；`border-2\|4` 硬边；hard shadow；Joyful Press；无 pill |
| **TextInput / SecretInput** | 直角硬边；focus 黄 ring（可叠黑边）；高度兼顾 44px 触控 |
| **Card / SectionPanel / StatCard** | 白底 + 黑边 + hard/彩色 offset；无 soft shadow |
| **Badge / Tabs / ViewModeToggle / Pagination** | 去 pill；硬边纯色 |
| **Dialog / Toast** | 直角 elevated；硬边；无 blur 遮罩柔化 |
| **Table / MobileEntityCard** | 无旋转；hover 仅纯色背景；禁止行 scale |
| **Sidebar / TopBar** | 去 blur；选中态色块/硬左边线或色块 |
| **Shell / Login** | 无 Grain、无渐变背景、无大圆角 |
| **Skeleton / Empty / Error** | 同一语言；Empty 可用 ≤3° 几何装饰（非 emoji） |
| **BrandMark** | 可轻微 tilt + 可选 ping-pong 阴影 |

状态全集：default / hover / active / focus / disabled / loading / empty / error。

---

## 7. Icons（Phosphor only）

- **仅** `@phosphor-icons/react`（建议 weight `bold`，或与现有 `regular`/`fill` 全站统一后文档化）
- 默认尺寸 18–20px
- **禁止** Lucide、手绘装饰 SVG 充当第二图标库
- 规范示例中的 Lucide 名必须映射为 Phosphor 等价图标

---

## 8. Accessibility

- WCAG AA：正文与主 CTA
- 可见 `:focus-visible`（`--focus-ring` 黄）
- 键盘：侧栏、Tabs、Dialog、密钥显隐
- 触控目标 ≥ 44px
- `lang="zh-CN"`
- 表单：禁止 placeholder 当 label
- 可见文案不用 em dash（`—`/`–`），改用 `-` 或改写

---

## 9. DO / DON'T

### DO

- `rounded-none` / 全 token radius `0`
- 硬黑边 `border-2|4`（dark 硬白边）
- 硬阴影 `Npx Npx 0 color`（零 blur）
- 五强调色丰富但分区克制
- Hover 可用 `scale-105`（控件级）
- 几何装饰（方块、三角、线条）
- Toy Spring + Joyful Press
- Phosphor only

### DON'T

- 任何 `rounded-sm+` / `rounded-full` pill
- 模糊阴影 / `backdrop-blur` / 渐变
- 旋转 > 3°
- 柔和灰色正文与边框
- emoji / Unicode 装饰符号
- Lucide 或第二图标库
- dense 表上滥用 tilt / 彩色 ping-pong / 整行 scale
- kraft Grain / soft paper 材质
- 改后端 API、路由 path、鉴权 cookie 语义

---

## 10. Screens / 范围

全站管理台视觉：Login + Shell + `lib/routes.ts` 全部 feature 页。  
业务数据流、`api.ts` 契约、AuthGate、cookie `jovepoxy_admin` **不改**。

---

## 11. Token 实现位置

| 文件 | 职责 |
|------|------|
| `web/DESIGN.md` | 设计 source of truth（本文） |
| `web/src/index.css` | CSS 变量、`@theme`、reduced-motion、空 grain |
| `web/src/lib/theme.ts` | 仅 light/dark 切换逻辑（不承载色值） |

发布嵌入 UI：`make embed-web-win` 后再 Go build。
