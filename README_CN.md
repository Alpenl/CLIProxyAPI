# CLIProxyAPI

Codex 专用代理与管理控制台。

这个分支已经刻意删除所有非 Codex 的产品路径，只保留：

- Codex OAuth 账号
- 仅保留 `/v1/models`、`/v1/responses`、`/v1/responses/compact` 和 websocket `GET /v1/responses`
- 内嵌中文管理面板 `/management.html`
- 账号导入、额度刷新、失效清理、用量统计

## 项目定位

这是一个面向 Codex 账号池的轻量代理：

- 对外暴露最小可用的 Codex 请求接口面
- 管理多份 `codex-*.json` OAuth 账号文件
- 自动刷新账号状态并清理失效账号
- 在 Web 控制台中查看请求量、账号健康和额度状态

## 快速开始

1. 直接启动程序，即使还没有 `config.yaml` 也可以。
2. 首次启动会自动生成 bootstrap 配置。
3. 打开 `http://127.0.0.1:8317/`，程序会自动跳转到 `/management.html`。
4. 第一次进入时在网页里完成初始化，至少配置一个下游 `api-keys`。
5. 初始化完成后用管理密钥登录后台。

## 当前支持的配置

示例配置 [config.example.yaml](config.example.yaml) 已经裁剪到最小可用集合：

- `auth-dir`
- `api-keys`
- 重试 / 路由 / 管理面板设置
- 可选的 Codex Header 默认值

不受支持的配置块在加载时会被忽略。

## 管理面板

管理面板资源直接内嵌在二进制中，不依赖外部下载。

主要页面：

- 仪表盘
- 账号额度
- 账号导入
- 操作日志

主要操作：

- 从服务器目录批量导入
- 上传导出的 Codex OAuth JSON 文件
- 手动刷新额度
- 清理失效账号
- 删除本地账号文件

当前 API 面：

- `GET /v1/models`
- `POST /v1/responses`
- `POST /v1/responses/compact`
- websocket 升级使用 `GET /v1/responses`

## 开发验证

常用验证命令：

```bash
go test ./... -count=1
./build-release.sh
docker build -t codex-proxy:codex-only .
```

## 许可证

MIT，详见 [LICENSE](LICENSE)。
