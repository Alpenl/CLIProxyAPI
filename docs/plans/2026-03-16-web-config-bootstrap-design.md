# Bootstrap 配置与网页配置管理设计

## Goal

把当前 Codex 专用代理改成“无预置配置即可启动”的产品形态：

- 启动时若不存在 `config.yaml`，自动生成最小可运行的 bootstrap 配置文件
- 第一次访问 `management.html` 时，不进入常规登录，而是直接进入首次初始化引导
- 初始化完成后，切换回正式的管理密钥鉴权模式
- 在网页端提供最小必要的配置管理页面，覆盖监听地址、TLS、管理密钥、客户端 API Key、账号目录、代理、日志和重试策略

## Current Problem

当前实现仍然假设部署前必须手工准备 `config.yaml`：

- `cmd/server/main.go` 直接调用 `LoadConfig`，缺少配置文件时进程启动失败
- 管理接口只有在存在管理密钥时才注册，导致“无配置首次启动”无法通过网页完成初始化
- 管理面板没有配置管理页面，也没有“首次配置”状态机
- `host` / `port` / `tls` 等关键配置虽然存在，但既无法在网页端维护，也没有“变更后需要重启”的反馈

## Approaches Considered

### 方案 A：启动时写入完整默认配置并生成随机密钥

优点：

- 后端改动最少
- 保持现有登录页逻辑

缺点：

- 用户第一次访问前必须从日志里找随机密钥
- Docker 直接部署时体验很差
- 不符合“第一次进入后台直接提示配置”的要求

### 方案 B：增加 bootstrap 模式与首次初始化接口

优点：

- 最符合用户预期
- 支持“零配置启动”
- 不需要依赖外部环境变量或交互式 CLI
- 初始化后可以无缝切回正式鉴权

缺点：

- 需要新增 bootstrap 状态判断、未初始化接口和前端状态机

### 方案 C：完全依赖环境变量启动，网页端只做后续编辑

优点：

- 服务端逻辑简单

缺点：

- 仍然要求部署者先知道一组必要环境变量
- 不符合“直接拉 docker 启动即可”的目标

## Recommendation

采用方案 B。

它是唯一同时满足以下条件的方案：

- 二进制与 Docker 都能直接启动
- 第一次进入后台就能完成配置
- 不需要预先手写配置文件
- 初始化完成后仍然保留正式的管理密钥保护

## Configuration Surface

### 网页端保留并可编辑

这些字段在当前 Codex-only 产品中有明确运行价值，且值得暴露给运营界面：

- `host`
- `port`
- `tls.enable`
- `tls.cert`
- `tls.key`
- `remote-management.allow-remote`
- `remote-management.secret-key`
- `auth-dir`
- `api-keys`
- `proxy-url`
- `logging-to-file`
- `logs-max-total-size-mb`
- `error-logs-max-files`
- `usage-statistics-enabled`
- `request-log`
- `disable-cooling`
- `request-retry`
- `max-retry-credentials`
- `max-retry-interval`
- `routing.strategy`

### 保留运行时支持，但不放进网页端

这些字段要么偏底层，要么是低频高级能力，本轮不进入网页配置页：

- `force-model-prefix`
- `passthrough-headers`
- `ws-auth`
- `nonstream-keepalive-interval`
- `streaming.keepalive-seconds`
- `streaming.bootstrap-retries`
- `quota-exceeded.*`
- `codex-header-defaults`
- `codex-api-key`

### 不应继续强化的遗留项

这些字段不是当前产品主路径，不进入新配置页，也不作为首次引导的一部分：

- `remote-management.disable-control-panel`
- `remote-management.panel-github-repository`
- 历史配置文件里的 `pprof`、`commercial-mode` 等遗留键

## Architecture

### 1. 启动阶段：自动生成 bootstrap 配置

新增配置引导函数：

- 若配置文件存在且可解析，按现有逻辑加载
- 若配置文件不存在或为空，则写入一份最小 bootstrap 配置
- bootstrap 配置包含：
  - `host: ""`
  - `port: 8317`
  - `remote-management.allow-remote: true`
  - `remote-management.secret-key: ""`
  - `auth-dir: "./auths"`
  - `api-keys: []`
  - 其余必要运行默认值

同时自动创建配置目录和 `auth-dir` 目录。

### 2. 后端：bootstrap 状态与配置管理接口

新增两个层次的管理接口：

- 无需登录即可访问：
  - `GET /v0/management/bootstrap/status`
  - `PUT /v0/management/bootstrap/config`
- 需要管理密钥：
  - `GET /v0/management/config`
  - `PUT /v0/management/config`

行为规则：

- 当系统尚未完成初始化时，普通管理接口返回 `409 bootstrap required`
- 首次配置接口要求至少填写：
  - 管理密钥
  - 至少一个客户端 API Key
  - 非空 `auth-dir`
- 管理密钥保存前即在后端完成 bcrypt 哈希，避免等待 watcher 二次转换

### 3. 前端：首次配置向导 + 配置管理页面

管理面板增加新的状态机：

- 打开页面先请求 bootstrap 状态
- 若未初始化：
  - 展示首次配置向导
  - 阻断常规登录流程
  - 保存成功后自动使用新密钥进入后台
- 若已初始化：
  - 维持现有登录页逻辑

新增独立页面“配置管理”，分为三组：

- 服务监听：`host`、`port`、TLS
- 访问与存储：管理密钥、允许远程、`auth-dir`、`api-keys`、`proxy-url`
- 运行策略：日志、用量统计、重试、路由

### 4. 生效模型

保存配置后分两类反馈：

- 热生效项：由 watcher 自动 reload，立即生效
- 监听相关项：`host`、`port`、`tls.*`
  - 写入配置文件
  - 在网页端提示“已保存，重启服务后生效”

## Error Handling

- bootstrap 保存时若缺少必要字段，返回明确字段级错误
- 若启用 TLS 但证书或私钥为空，拒绝保存
- 若 API Key 列表去重清洗后为空，拒绝保存
- 若路由策略非法，拒绝保存
- 若日志或重试数值为负，拒绝保存

## Testing Strategy

本轮使用 TDD 锁定四条边界：

1. 缺少 `config.yaml` 时会自动生成 bootstrap 配置
2. 无管理密钥时 bootstrap 状态接口可用，但普通管理接口不可用
3. 首次配置保存后，密钥会被哈希写入配置文件
4. 已初始化时配置读取与更新接口只暴露精简后的配置面

## Success Criteria

- 删除手工预置 `config.yaml` 后，二进制和 Docker 都能直接启动
- 首次打开 `management.html` 时直接进入配置引导
- 引导保存后能自动进入后台
- 后台新增“配置管理”页面
- 改动通过全量测试、二进制构建和 Docker 构建
