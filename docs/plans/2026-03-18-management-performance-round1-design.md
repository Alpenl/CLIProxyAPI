# 2026-03-18 Management Performance Round 1 Design

## Goal

在不改动核心调度与协议行为的前提下，优先优化 `CLIProxyAPI` 与 `cdx-rt` 的管理面性能，降低管理页刷新带来的重复计算、磁盘 IO 和轮询放大。

## Scope

本轮只做低风险、高收益的管理面优化：

1. `CLIProxyAPI`
   - 为管理页概览新增聚合接口，减少一次刷新触发多个 HTTP 请求。
   - 为 Codex 账号列表结果增加短 TTL 缓存，避免短时间内重复构建列表。
   - 避免账号列表接口中的重复 JWT 解析，优先复用已经沉淀到内存结构中的字段。

2. `cdx-rt`
   - 为 `service-config.json` 与 `jobs.json` 增加进程内缓存，避免每次 API 访问都重复读盘和反复 JSON 解析。
   - 将任务日志 tail 从“整文件读取后切片”改为“从文件尾部按块回读”，降低日志轮询时的大文件 IO 成本。

## Alternatives Considered

### Option A: 优先优化管理面读路径与轮询放大

优点：
- 风险低，不触碰跑号与补号主链路。
- 收益确定，尤其适合当前 Web 管理台和 Sealos 场景。
- 便于通过接口测试和基准验证。

缺点：
- 不会直接提升协议注册吞吐。

### Option B: 直接优化 `rt` 协议并发与传输层吞吐

优点：
- 如果网络侧是瓶颈，可能直接提升跑号速度。

缺点：
- 风险高，容易影响注册稳定性、Cloudflare 兼容性和补号成功率。
- 当前没有足够基准证明这是第一瓶颈。

### Option C: 先做镜像瘦身与前端拆包

优点：
- 能立刻减少镜像分发成本和页面首包大小。

缺点：
- 对当前最痛的“管理页刷新压磁盘/压 CPU”问题帮助有限。
- 不如 A 直接命中运行时瓶颈。

## Decision

采用 Option A 作为第一轮优化主线，同时顺手保留后续做镜像瘦身与前端拆包的接口空间。

## Architecture

### CLIProxyAPI

- 新增管理概览聚合接口，由服务端一次性返回账号列表、用量统计、补号状态。
- 在 `management.Handler` 内维护账号列表短 TTL 缓存。
- 账号列表构建继续以 `authManager.List()` 为输入，但缓存构建结果，避免短时间内重复执行 `os.Stat` 和 `id_token` 解析。
- 对 `id_token` 的派生字段优先复用 `Auth.Attributes` 中已有的 `plan_type`，只在缺失时解析 JWT。

### cdx-rt

- 在 `config-store` 与 `job-store` 内增加基于文件 `mtime` / `size` 的进程内缓存。
- 所有通过 store 发生的写操作都会回写缓存，保证进程内读取命中最新状态。
- 日志 tail 改为从文件末尾按块反向读取，直到得到足够多的行，避免任务日志越大接口越慢。

## Testing Strategy

- `CLIProxyAPI`
  - 新增管理概览接口测试。
  - 新增账号列表缓存行为测试。
  - 保持现有管理页与补号测试通过。

- `cdx-rt`
  - 新增 config cache / job store cache 测试。
  - 新增大日志 tail 读取正确性测试。
  - 保持现有 HTTP API 与 job runner 测试通过。

## Non-Goals

- 不修改 Codex 调度算法。
- 不调整协议注册步骤、Cloudflare 绕过逻辑或 OTP 轮询策略。
- 不在本轮做 Web UI 拆包、SSR 或镜像基底大迁移。
