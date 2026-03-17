# Codex-Only Rewrite Design

## Goal

将当前 `alpen-rewrite` 分支从“路由层偏向 Codex”进一步收敛为“仓库和运行时都只保留 Codex”：

- 只保留 Codex OAuth / Codex auth 文件 / Codex API key
- 只保留 OpenAI 风格的请求协议外壳，作为 Codex 对外接口
- 删除所有非 Codex provider 的配置、调度、鉴权、管理实现与测试

## Current Problem

当前仓库虽然已经删除了大部分旧路由，但核心仍然保留以下多 provider 逻辑：

- `internal/config` 仍接受 Gemini / Claude / Vertex / OpenAI-compat / Amp 等配置
- `internal/watcher/synthesizer` 仍会从配置和 auth 文件合成非 Codex auth
- `sdk/cliproxy/service` 和 `sdk/cliproxy/auth/conductor` 仍为 Gemini / Claude / Vertex 等注册 executor 与模型映射
- `internal/api/handlers/management` 仍包含通用 auth / OAuth / API key 管理代码
- `internal/auth/*` 目录中仍保留多个非 Codex provider 实现

这会导致当前分支仍然不是严格意义上的 Codex-only。

## Scope

### 保留

- `codex` provider 相关 auth 读写、刷新、额度检查、账号导入与清理
- OpenAI-compatible `/v1/*` 路由，作为 Codex 对外接口
- 管理页登录、概览、日志、Codex 账号列表、导入、清理
- 最小必要的公共基础设施：
  - 请求日志
  - usage 统计
  - token store
  - watcher 的 Codex-only 子集

### 删除

- Gemini / Claude / Vertex / OpenAI-compat / Qwen / IFlow / Antigravity / Kimi / Amp 的：
  - 配置项
  - executor 注册
  - auth 文件合成逻辑
  - auth provider 实现目录
  - 管理 API 与工具代码
  - 测试与示例

## Recommended Approach

推荐直接做“彻底切断 + 物理删除”。

原因：

- 单纯隐藏路由并不能阻止 core service 接受其他 provider
- 单纯保留旧配置字段会持续拖着 watcher / synthesizer / conductor 的分支逻辑
- 既然当前分支就是重写版，应该让代码结构本身表达 Codex-only，而不是靠约定

## Architecture

### 1. Config 收窄

`internal/config.Config` 改为只保留 Codex 运行需要的字段：

- 通用服务字段
- 管理端字段
- `codex-api-key`
- 与 Codex 请求有关的最小 payload / retry / routing 配置

删除其他 provider 专属配置字段和对应 sanitize / migration 逻辑。

### 2. Auth 与 Synthesizer 收窄

- `internal/watcher/synthesizer/config.go` 只生成 Codex API key auth
- `internal/watcher/synthesizer/file.go` 只识别 Codex auth JSON
- watcher 的统计日志只报告 Codex auth entries + Codex API keys

### 3. Runtime 收窄

- `sdk/cliproxy/service.go` 只注册 Codex executor
- `sdk/cliproxy/auth/conductor.go` 只处理 Codex 的模型 alias / upstream model 解析
- `internal/runtime/executor` 仅保留 Codex 与必要的 OpenAI 协议转换路径

### 4. Management 收窄

管理层从“通用管理后台”收敛为“Codex 控制台后端”：

- 保留：
  - usage
  - Codex accounts
  - import directory
  - import files
  - cleanup invalid
  - delete account
- 删除：
  - 通用 auth files 管理
  - OAuth session provider 归一化
  - 多 provider API key 列表
  - Gemini / Claude / Antigravity 工具逻辑
  - Vertex 导入逻辑

### 5. 物理删除

当依赖切断后，直接删除以下目录和文件：

- `internal/auth/{antigravity,claude,gemini,iflow,kimi,qwen,vertex}`
- 不再被引用的 management 文件
- 不再被引用的测试与示例

## Testing Strategy

采用 TDD 锁定这几个长期边界：

1. 配置层忽略所有非 Codex provider 配置
2. auth 文件合成层忽略所有非 Codex auth JSON
3. OAuth provider 归一化只接受 Codex
4. service / watcher 只统计并注册 Codex
5. 全量 `go test ./...` 与 `go build ./cmd/server`

## Risks

- `internal/config` 很大，删字段时容易牵连到管理层 patch/list handler
- watcher/synthesizer 删除其他 provider 后，大量旧测试需要一并改写或删除
- 若误删 OpenAI 协议外壳，会影响 Codex 对外 API 兼容性

## Success Criteria

- 仓库内不再有非 Codex provider 的运行时代码路径
- `internal/auth` 只剩 `codex` 与必要公共接口
- 配置示例只包含 Codex 相关项
- 全量测试和构建通过
- 当前本地 8317 服务仍可正常进入管理面板并提供 Codex API
