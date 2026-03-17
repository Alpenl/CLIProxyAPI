# Docker Data Layout Design

**Goal**

把 Docker/Sealos 运行时的所有用户态数据统一收敛到 `/data`，让挂载、备份、迁移和排障都只围绕一个卷目录展开。

**Chosen Layout**

采用扁平根目录方案：

```text
/app
  CLIProxyAPI
  config.example.yaml

/data
  config.yaml
  auths/
  import/
  logs/
```

说明：

- `/app` 只放镜像内只读资产。
- `/data` 只放用户配置、账号文件、导入目录和日志。
- Docker 默认入口固定读取 `/data/config.yaml`。
- 首次启动生成的 `auth-dir` 固定落到 `/data/auths`。
- 管理台“服务器目录导入”的默认建议路径固定为 `/data/import`。
- 运行日志和请求日志统一落到 `/data/logs`。

**Why This Layout**

- Sealos 只需要挂一个 `/data`，不会再为配置、账号、日志分别找映射点。
- 用户态文件与镜像文件彻底分离，升级镜像不会污染持久化数据。
- 当前代码已经有“配置文件目录派生 auth-dir”的能力，落地成本最低。

**Behavior Boundaries**

- Docker 镜像默认行为强制统一到 `/data`。
- 非 Docker 的本地开发场景，仍然允许通过显式 `-config` 使用任意配置文件路径。
- 现网兼容性这次不保留，按用户要求直接统一新结构。

**Implementation Notes**

- Dockerfile 的可执行文件与示例配置改放到 `/app`。
- Docker 默认命令改为 `-config /data/config.yaml`。
- 日志目录解析不能只看当前工作目录，否则主日志可能写到 `/app/logs`。需要把主日志和请求日志都锚定到配置文件所在目录。
- 文档与 `docker-compose.yml` 需要同步成 `/data` 结构，避免镜像行为和说明不一致。
