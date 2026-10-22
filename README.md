# frp 定制版管理面板

基于 [fatedier/frp](https://github.com/fatedier/frp) 的非官方定制分支，提供中文
管理面板、会话认证、在线配置、ACME 证书、流量捕获，以及独立的 frps/frpc
Docker 镜像。

[GitHub](https://github.com/lonquan/frp-control) |
[CNB](https://cnb.cool/antcool/frp-control) |
[Docker 部署文档](deploy/README.md) |
[上游文档](https://gofrp.org/zh-cn/docs/)

> 本仓库不是 frp 官方发行版。生产使用前请自行完成安全评估、备份和升级演练。

当前发布版本：`v0.0.2`。

## 主要能力

- 提供 `lonquan/frp-frps` 和 `lonquan/frp-frpc`，支持 Linux amd64/arm64。
- frps/frpc 管理面板使用管理员会话、CSRF 校验和 Argon2id 密码哈希。
- frps 支持 TOML 在线编辑、校验、历史版本、保存重启和失败回滚。
- frpc 支持主配置与 include 文件管理，以及代理、访问者和插件可视化配置。
- 支持 ACME DNS-01、通配符证书、自动续期、单条/全部续期和证书下载。
- 支持 HTTP/HTTPS/WSS 流量捕获、敏感内容遮蔽和同目标 HTTP 重放。
- 支持运行指标、日志诊断和按需下载 pprof 性能文件。
- 保留 TCP、UDP、HTTP、HTTPS、TCPMUX、STCP、XTCP、SUDP 等代理类型。

## 快速开始

运行 Docker Hub 镜像只需要 Docker。源码构建和本地开发需要 Go 1.26、Node.js
22+、Docker Compose v2 和 Make。

### 1. 拉取镜像

```bash
docker pull lonquan/frp-frps:v0.0.2
docker pull lonquan/frp-frpc:v0.0.2
```

国内网络可以改用 DaoCloud 镜像：

```bash
docker pull docker.m.daocloud.io/lonquan/frp-frps:v0.0.2
docker pull docker.m.daocloud.io/lonquan/frp-frpc:v0.0.2
```

`latest` 会随正式版本移动，生产环境应固定版本。

### 2. 启动 frps

先在公网服务器生成认证 token：

```bash
openssl rand -hex 32
```

创建持久化目录：

```bash
mkdir -p "$HOME/frp/frps/config" "$HOME/frp/frps/logs" "$HOME/frp/frps/data"
sudo chown -R 10001:10001 "$HOME/frp/frps/config" \
  "$HOME/frp/frps/logs" "$HOME/frp/frps/data"
```

启动服务端，并替换密码、token 和目录：

```bash
docker run -d --name frps \
  --restart unless-stopped \
  --stop-timeout 20 \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m,mode=1777 \
  --tmpfs /run:rw,noexec,nosuid,size=16m,mode=1777 \
  -p 7000:7000/tcp \
  -p 7000:7000/udp \
  -p 80:8080 \
  -p 443:8443 \
  -p 7500:7500 \
  -e FRP_ADMIN_USERNAME=admin \
  -e FRP_ADMIN_PASSWORD='替换为管理员密码' \
  -e FRP_AUTH_TOKEN='替换为刚生成的 token' \
  -v "$HOME/frp/frps/config:/etc/frp" \
  -v "$HOME/frp/frps/logs:/var/log/frp" \
  -v "$HOME/frp/frps/data:/var/lib/frp" \
  --log-opt max-size=20m \
  --log-opt max-file=5 \
  lonquan/frp-frps:v0.0.2
```

检查状态：

```bash
docker ps --filter name=frps
docker inspect --format '{{json .State.Health.Status}}' frps
docker logs --tail=200 frps
```

### 3. 启动 frpc

客户端 `FRP_AUTH_TOKEN` 必须与服务端一致：

```bash
mkdir -p "$HOME/frp/frpc/config" "$HOME/frp/frpc/logs" "$HOME/frp/frpc/data"
sudo chown -R 10001:10001 "$HOME/frp/frpc/config" \
  "$HOME/frp/frpc/logs" "$HOME/frp/frpc/data"

docker run -d --name frpc \
  --restart unless-stopped \
  --stop-timeout 20 \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --add-host host.docker.internal:host-gateway \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m,mode=1777 \
  --tmpfs /run:rw,noexec,nosuid,size=16m,mode=1777 \
  -p 127.0.0.1:7400:7400 \
  -e FRP_ADMIN_USERNAME=admin \
  -e FRP_ADMIN_PASSWORD='替换为客户端管理员密码' \
  -e FRP_AUTH_TOKEN='与服务端完全一致的 token' \
  -e FRP_SERVER_ADDR='your-server.example.com' \
  -e FRP_SERVER_PORT=7000 \
  -v "$HOME/frp/frpc/config:/etc/frp" \
  -v "$HOME/frp/frpc/logs:/var/log/frp" \
  -v "$HOME/frp/frpc/data:/var/lib/frp" \
  --log-opt max-size=20m \
  --log-opt max-file=5 \
  lonquan/frp-frpc:v0.0.2
```

### 4. 登录面板

- 服务端：`http://服务器地址:7500/static/`
- 客户端：`http://127.0.0.1:7400/static/`

首次登录使用启动参数中的管理员账号。修改密码后，环境变量中的初始密码不会
覆盖已有认证文件。

### 5. 添加代理

在客户端面板的“代理”页面创建 TCP、UDP 或 HTTP 代理。代理后端运行在宿主机时，
Docker Desktop 使用 `host.docker.internal`；frpc 与后端在同一 Docker 网络时
使用服务名。

远程端口监听在 frps 容器内。例如代理使用 `remotePort = 6000` 时，frps 启动
参数必须包含：

```text
-p 6000:6000/tcp
-p 6000:6000/udp
```

Docker 不会自动修改已有容器的端口映射。完整部署、端口、Compose、Docker
secrets 和排障说明见 [deploy/README.md](deploy/README.md)。

## 关键配置

### 持久化目录

| 容器目录       | 用途                                        |
| -------------- | ------------------------------------------- |
| `/etc/frp`     | 主配置、include 文件、配置历史              |
| `/var/log/frp` | 文件日志                                    |
| `/var/lib/frp` | 管理员会话、动态配置、证书、捕获记录和诊断设置 |

### 常用变量

| 变量                          | 服务  | 说明                                      |
| ----------------------------- | ----- | ----------------------------------------- |
| `FRP_ADMIN_USERNAME`          | 两者  | 首次初始化管理员用户名                    |
| `FRP_ADMIN_PASSWORD`          | 两者  | 首次初始化管理员密码                      |
| `FRP_ADMIN_PASSWORD_FILE`     | 两者  | 从文件读取管理员密码，优先于直接值        |
| `FRP_AUTH_TOKEN`              | 两者  | 隧道认证 token，frps/frpc 必须一致        |
| `FRP_AUTH_TOKEN_FILE`         | 两者  | 从文件读取 token，优先于直接值            |
| `FRP_SERVER_ADDR`             | frpc  | frps 地址，首次生成配置时必填             |
| `FRP_SERVER_PORT`             | frpc  | frps 端口，默认 7000                      |
| `FRP_REGENERATE_CONFIG`       | 两者  | 设为 `1` 时按环境变量重建已有主配置       |
| `FRP_ALLOW_DASHBOARD_NETWORK_CHANGES` | 两者 | 是否允许面板修改 WebServer 网络字段 |

已有认证文件时不需要再次提供管理员密码。直接变量和 `_FILE` 同时设置时文件优先。

## 管理功能

### 配置与代理

- frps 配置支持在线编辑、校验、历史恢复、保存重启和 Supervisor 回滚。
- frpc 支持 `frpc.toml`、include 文件、代理、访问者和插件配置。
- 客户端代理和访问者页面分别展示运行状态与全部配置；状态列表在页面可见时每
  10 秒自动刷新。
- 动态配置只能在独立的 Store JSON 中启用/禁用，配置文件来源仍通过配置中心维护。
- 可视化动态配置管理要求 `store.path` 指向可写的客户端数据文件。
- 修改会切断面板连接的网络字段时，需要临时设置
  `FRP_ALLOW_DASHBOARD_NETWORK_CHANGES=1`。

### ACME 证书

- 所有托管证书统一使用 DNS-01。
- 支持 lego 全量 DNS provider，`manual` 和 `exec` 除外。
- DNS provider 和凭据在客户端“证书 → DNS 设置”中保存，不通过环境变量读取。
- 凭据保存在 `/var/lib/frp/acme/dns-provider.json`，文件权限为 0600。
- 新建证书后立即签发，内容创建后不可修改。
- 行内“续期”处理单个证书，“全部续期”串行处理全部非签发中证书。
- 支持下载包含 `cert.pem` 和 `key.pem` 的 ZIP，请妥善保护私钥。

### 流量捕获

捕获默认关闭，可按代理启用。捕获内容可能包含 Cookie、Authorization 和请求
正文，详情默认遮蔽，显示敏感内容会写入审计日志。捕获数据以明文保存在
`/var/lib/frp/captures`，必须限制数据目录访问权限。

### 监控与诊断

运行指标集中在总览页。pprof 诊断默认关闭，管理员开启后可在当前会话内下载
goroutine、heap 和 CPU profile。`/healthz` 保持匿名可访问。

## 安全建议

- 不要把管理面板直接暴露在公网；使用 HTTPS 反向代理或为 `webServer.tls`
  配置证书。
- 使用随机强密码和至少 32 字节的 `FRP_AUTH_TOKEN`。
- 优先使用 `_FILE` 或 Docker secrets，避免密码出现在进程参数中。
- 定期备份 `/etc/frp` 和 `/var/lib/frp`，尤其是配置、认证文件、证书和私钥。
- 登录失败会按来源 IP 和用户名限流；管理请求使用 HttpOnly、SameSite Cookie
  和 CSRF Token。
- 删除 `data/admin/auth.json` 会清除管理员账号，下次启动时重新使用环境变量
  初始化。

## 源码构建

```bash
git clone https://github.com/lonquan/frp-control.git
cd frp-control

make docker-build
```

只构建单个镜像：

```bash
make docker-build-frps
make docker-build-frpc
```

中国大陆网络可使用：

```bash
make docker-build-frps-cn
make docker-build-frpc-cn
```

本地检查和构建：

```bash
make test
make vet
make web-ci
make build
```

推送 `v*` Git 标签会触发 Docker Hub 发布工作流，构建 amd64/arm64 镜像。发布
工作流需要仓库 Secrets：`DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`。

## 兼容性

- 管理面板只支持会话认证，不再支持 Basic Auth。
- legacy INI 已移除，请使用 TOML、YAML 或 JSON。
- `webServer.user/password/pprofEnable`、`metricsServer` 和
  `enablePrometheus` 已移除。
- 上游代理类型和主要数据面协议保持兼容，但旧版管理 API 和 CLI 管理命令不再
  适用。
- 升级前备份 `config`、`data`，并固定 Docker 镜像版本。

## 相关文档

- [完整 Docker 部署与运维](deploy/README.md)
- [frps 完整配置示例](conf/frps_full_example.toml)
- [frpc 完整配置示例](conf/frpc_full_example.toml)
- [上游 frp 中文文档](https://gofrp.org/zh-cn/docs/)
- [Apache License 2.0](LICENSE)
