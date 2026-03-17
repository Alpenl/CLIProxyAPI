# 自动补号可靠性修复设计

## 背景

2026-03-17 的线上排障暴露了三条独立但串联的故障链路：

1. `cdx-rt` 的 `jobs.json` 直接覆盖写，异常中断后可能留下截断 JSON，服务重启时在 `JSON.parse` 处崩溃。
2. `cdx-rt` 的 job 队列恢复逻辑没有处理旧的 `running` job，进程重启后旧 job 会永久占住并发，后续自动补号任务一直排队。
3. 协议链路默认 `fetch` 没有全局超时，单个账号在 OpenAI / Cloudflare / DuckMail 任一请求挂住时，整个 job 会长时间停在 `running`。
4. `CLIProxyAPI` 只监听配置文件本身，不监听父目录；当配置通过原子替换写入时，热重载可能完全收不到事件。

这些问题叠加后，表现为：

- `cpa` 账号池只有 6 个可用账号；
- 另外 4 个补号文件只是空占位；
- `rt` 队列不断累积 `queued` / `running` job；
- 修改目标值后不一定立即生效。

## 目标

把自动补号链路修到以下可验收状态：

1. `cdx-rt` 的状态文件写入具备原子性，异常中断不会把服务打崩。
2. `cdx-rt` 重启后不会被旧 `running` job 永久堵塞。
3. 单个协议请求和单个 job 都有明确超时边界，挂死任务最终会失败并释放队列。
4. `CLIProxyAPI` 的配置热重载对原子替换写入可靠。
5. 以上行为都有自动化测试覆盖。

## 方案

### 1. `cdx-rt` 状态文件原子化

在 `src/service/job-store.ts` 和 `src/service/config-store.ts` 引入共享的原子写工具：

- 写入同目录临时文件；
- `fsync` 后 `rename` 覆盖目标文件；
- 读取时如果主文件为空或 JSON 无法解析，优先尝试 `.bak` 备份；
- 若备份也不可用，则回退到受控的空状态并立即落盘。

这样 `jobs.json` 和 `service-config.json` 都不会因为半写入而让服务无法启动。

### 2. `cdx-rt` 队列恢复与超时

对 job runner 做两层兜底：

- **启动恢复**：每次 runner 启动前，把遗留的 `running` job 统一标记为 `failed`，写入明确的 `lastError`，释放队列。
- **执行超时**：
  - 给 fetch 传输层增加默认超时；
  - 在 runner 级别对整批 job 增加 `jobTimeoutSeconds` 兜底，超时后把 job 标记为 `failed`。

这里不做复杂的断点续跑，而是优先保证“不会永久卡住”。因为当前自动补号的业务目标是保持池子健康，而不是保证单个 job 必然继续执行。

### 3. `CLIProxyAPI` 配置监听修复

Watcher 同时监听：

- 配置文件本身；
- 配置文件所在目录。

事件匹配时只要命中目标配置文件路径就触发 reload，并在收到 `Rename` / `Remove` 后重新添加 watch，保证原子替换后的新 inode 仍被持续监听。

## 测试策略

### `cdx-rt`

- `job-store`：
  - 损坏的 `jobs.json` 可从备份恢复；
  - 无法恢复时回退到空状态；
  - 原子写后主文件始终是完整 JSON。
- `job-runner`：
  - 启动时会释放 stale `running` job；
  - 超时 job 会被标记为 `failed` 并保留错误信息。
- `transport`：
  - fetch 传输层会在超时时间后拒绝并返回明确错误。

### `CLIProxyAPI`

- watcher 测试覆盖“配置文件原子替换后仍然会触发 reload”。

## 非目标

- 这轮不改协议注册本身的成功率算法。
- 这轮不做真正的任务断点续跑。
- 这轮不改变管理 UI 的交互，只修可靠性。
