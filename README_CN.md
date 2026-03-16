# CLIProxyAPI

Codex 专用代理与管理控制台。

这个分支已经刻意删除所有非 Codex provider 的产品路径，只保留：

- Codex OAuth 账号
- Codex API Key
- 面向 Codex 的 OpenAI 兼容 `/v1/*` 接口
- 内嵌中文管理面板 `/management.html`
- 账号导入、额度刷新、失效清理、用量统计

## 项目定位

这是一个面向 Codex 账号池的轻量代理：

- 对外暴露单一 OpenAI 兼容接口
- 管理多份 `codex-*.json` OAuth 账号文件
- 自动刷新账号状态并清理失效账号
- 在 Web 控制台中查看请求量、账号健康和额度状态

## 快速开始

1. 复制 [config.example.yaml](config.example.yaml) 作为自己的配置文件。
2. 设置 `remote-management.secret-key`。
3. 至少配置一个下游 `api-keys`。
4. 启动服务。
5. 访问 `http://127.0.0.1:8317/management.html`。

## 当前支持的配置

示例配置 [config.example.yaml](config.example.yaml) 已经裁剪到最小可用集合：

- `auth-dir`
- `api-keys`
- `codex-api-key`（可选）
- 重试 / 路由 / 管理面板设置
- 可选的 Codex Header 默认值

旧的多 provider 配置块在加载时会被忽略，在保存时会被清理掉。

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

## 开发验证

常用验证命令：

```bash
go test ./... -count=1
go build -o ./bin/codex-proxy ./cmd/server
docker build -t codex-proxy:codex-only .
```

## 许可证

MIT，详见 [LICENSE](LICENSE)。
