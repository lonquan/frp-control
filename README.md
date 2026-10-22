# frp 定制版管理面板

本仓库是基于 [fatedier/frp](https://github.com/fatedier/frp) 的非官方定制分支，
主要增加中文 Web 管理面板、会话认证、在线 TOML 配置管理、ACME 证书托管、
流量捕获与重放，以及 frps/frpc 两套独立的 Docker 部署方案。

[GitHub 仓库](https://github.com/lonquan/frp-control) |
[CNB 备用仓库](https://cnb.cool/antcool/frp) |
[完整 Docker 文档](deploy/README.md) |
[上游 frp 文档](https://gofrp.org/zh-cn/docs/)

## 仓库声明

- 本仓库不是 frp 官方项目，不代表上游团队或维护者的官方发行版本。
- 上游 frp 的版权、商标和原始代码归其各自作者所有，本项目继续遵循
  [Apache License 2.0](LICENSE)。
- 本仓库保留了 Go 模块路径 `github.com/fatedier/frp`，用于兼容原有 import
  路径和代理实现。它只是模块标识，不会在构建本仓库时下载上游同名包；所有
  `github.com/fatedier/frp/...` 包均编译当前工作树中的源码。
- 定制版保留了上游代理类型和数据面能力，但管理界面、认证方式和管理 API
  已不再保持上游旧版 Web 控制台的兼容性。
- 在生产环境使用前，请自行完成安全评估、配置备份和升级演练。不要将未经
  HTTPS 保护的管理面板直接暴露到公网。

## 主要能力

- 提供 `lonquan/frp-frps` 与 `lonquan/frp-frpc` 多架构镜像，可直接通过
  `docker run` 启动，也保留独立的 Docker Compose 源码构建方案。
- 前端使用 Vue 3、Vue Router、Pinia、Arco Design 和 CodeMirror 6 重写。
- 服务端和客户端面板均使用管理员会话认证，不再提供管理 Web Basic Auth。
- 服务端支持 TOML 在线编辑、校验、版本历史、保存后重启、健康检查和失败回滚。
- 客户端支持主配置和 `includes` 多文件编辑，以及代理、访问者、插件的可视化
  表单配置。
- 客户端集成 ACME DNS-01、通配符证书、可用证书目录、自定义 CA 和自动续期。
- 支持 HTTP、HTTPS 转换插件及 WSS 消息帧的流量捕获和同目标 HTTP 重放。
- 监控、运行指标和诊断集中在会话保护的管理面板中。
- 保留 TCP、UDP、HTTP、HTTPS、TCPMUX、STCP、XTCP、SUDP 全部代理类型，
  以及现有访问者和客户端插件类型。

## 运行架构

```text
浏览器
  │
  ├── frps 管理面板 :7500
  │      └── frp-supervisor -> frps
  │                              ▲
  │                              │ frp 控制连接
  └── frpc 管理面板 :7400          │
         └── frp-supervisor -> frpc
```

容器 PID 1 为 `frp-supervisor`。它负责启动和监控 frps/frpc、处理面板发起的
重启、等待子进程就绪，并在新配置启动失败时恢复上一份可用配置。

## 环境要求

- macOS/Windows 使用 Docker Desktop Linux 容器模式，Linux 使用 Docker Engine
- 运行 Docker Hub 镜像只需要 Docker；不需要安装 Git、Go、Node.js 或 Make
- 只有源码构建和本地开发需要 Docker Compose v2、Git、Go 1.26 和 Node.js 22+

快速开始中的 token 示例使用 `openssl`；也可以使用密码管理器或其他工具生成
至少 32 字节的随机值并保存到 Docker secret。

两个 Dockerfile 都包含 Node.js 多阶段构建，直接从容器上下文生成前端资源。
构建镜像不要求宿主机预装 Node.js 或提前生成 `dist`。Windows 未安装 Make 时，
可以直接执行 `docker compose -f deploy/.../compose.yaml build`。

## 快速开始

镜像版本与内置 frp 上游版本独立演进。当前示例固定使用首个镜像版本
`v0.0.1`；`latest` 会随正式版本移动，因此生产环境应固定版本，避免无意升级。

| Docker Hub 仓库                   | 用途                              |
| --------------------------------- | --------------------------------- |
| `lonquan/frp-frps:v0.0.1`         | 公网服务端                        |
| `lonquan/frp-frpc:v0.0.1`         | 内网或本地客户端                  |

发布镜像同时支持 Linux amd64 和 arm64。Docker Desktop 会按宿主机架构自动选择。

### 1. 拉取镜像

```bash
docker pull lonquan/frp-frps:v0.0.1
docker pull lonquan/frp-frpc:v0.0.1
```

查看镜像中的分发版本和 frp 上游版本：

```bash
docker run --rm --entrypoint /usr/bin/frps \
  lonquan/frp-frps:v0.0.1 --version
docker run --rm --entrypoint /usr/bin/frpc \
  lonquan/frp-frpc:v0.0.1 --version
```

输出格式为 `0.0.1 (frp 0.71.1)`。管理面板会分别显示这两个版本。

### 2. 启动 frps

在具有公网 IP 或公网入口的服务器上生成认证 token：

```bash
openssl rand -hex 32
```

创建持久化目录：

```bash
mkdir -p "$HOME/frp/frps/config" "$HOME/frp/frps/logs" "$HOME/frp/frps/data"
```

Linux 需要将目录交给容器用户 UID/GID 10001：

```bash
sudo chown -R 10001:10001 "$HOME/frp/frps/config" \
  "$HOME/frp/frps/logs" "$HOME/frp/frps/data"
```

启动服务端，将密码、token、公网入口和宿主机目录替换为实际值：

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
  -e FRP_ADMIN_PASSWORD='替换为足够长的管理员密码' \
  -e FRP_AUTH_TOKEN='替换为刚生成的 token' \
  -v "$HOME/frp/frps/config:/etc/frp" \
  -v "$HOME/frp/frps/logs:/var/log/frp" \
  -v "$HOME/frp/frps/data:/var/lib/frp" \
  --log-opt max-size=20m \
  --log-opt max-file=5 \
  lonquan/frp-frps:v0.0.1
```

检查状态和健康检查：

```bash
docker ps --filter name=frps
docker inspect --format '{{json .State.Health.Status}}' frps
docker logs --tail=200 frps
```

首次启动且配置目录为空时，容器会根据环境变量生成 `frps.toml`。之后配置目录
和面板是主配置的权威来源，重启不会覆盖已有 TOML。只有显式设置
`FRP_REGENERATE_CONFIG=1` 才会根据环境变量重建配置，并将旧文件备份为
`frps.toml.env-backup-<UTC时间>`。

如已有认证文件 `/var/lib/frp/admin/auth.json`，重启时不再要求
`FRP_ADMIN_PASSWORD`。验证 Docker secrets 时可以使用：

```bash
mkdir -p "$HOME/frp/secrets"
printf '%s' '替换为管理员密码' > "$HOME/frp/secrets/admin-password"
printf '%s' '替换为认证 token' > "$HOME/frp/secrets/auth-token"
chmod 0600 "$HOME/frp/secrets/"*
```

再将运行参数中的直接值替换为文件变量，并挂载 secrets 目录：

```text
-e FRP_ADMIN_PASSWORD_FILE=/run/secrets/frp_admin_password
-e FRP_AUTH_TOKEN_FILE=/run/secrets/frp_auth_token
-v "$HOME/frp/secrets/admin-password:/run/secrets/frp_admin_password:ro"
-v "$HOME/frp/secrets/auth-token:/run/secrets/frp_auth_token:ro"
```

直接变量和 `_FILE` 同时设置时文件优先。敏感变量仍会出现在容器运行配置中；
需要降低暴露面时优先使用 `_FILE`。

也可以把运行变量写入权限为 `0600` 的 env 文件，并在两个 `docker run` 命令中
使用 `--env-file ./frp.env`。env 文件只保存运行变量，Docker 宿主机端口仍必须
通过 `-p` 指定。

### 3. 启动 frpc

在需要穿透的客户端机器上创建同名 token，并启动客户端：

```bash
mkdir -p "$HOME/frp/frpc/config" "$HOME/frp/frpc/logs" "$HOME/frp/frpc/data"
```

Linux 同样需要：

```bash
sudo chown -R 10001:10001 "$HOME/frp/frpc/config" \
  "$HOME/frp/frpc/logs" "$HOME/frp/frpc/data"
```

```bash
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
  lonquan/frp-frpc:v0.0.1
```

如果本地后端运行在宿主机，代理表单中的 `localIP` 使用
`host.docker.internal`。如果后端在另一个 Docker 容器中，请让 frpc 加入同一
Docker 网络并使用服务名或容器名，而不是发布后端端口。

### 4. 登录面板

- 服务端面板：`http://服务器地址:7500/static/`
- 客户端面板：`http://127.0.0.1:7400/static/`

首次登录使用启动参数中的用户名和密码。认证成功后使用 HttpOnly、SameSite
Cookie 和 CSRF Token 保护管理请求。默认会话有效期为 7 天。

### 5. 配置 TCP/UDP 6000 代理

在 frpc 面板中新建 TCP 或 UDP 代理，设置本地后端和 `remotePort = 6000`。
该远程端口监听在 **frps 容器**，所以需要在 frps 启动参数中发布：

```bash
-p 6000:6000/tcp
-p 6000:6000/udp
```

frpc 不需要发布 6000。Docker 不会自动改变运行中容器的端口映射；启用新的远程
端口后，必须使用包含该 `-p` 参数的配置重新创建 frps 容器。

修改 token、服务器地址等首次生成参数时，需要在 frps 和 frpc 两侧都设置
`FRP_REGENERATE_CONFIG=1` 并重新创建容器。重建配置不会重置管理员密码；
忘记密码需按后文删除 `data/admin/auth.json` 后重新初始化。

## 默认端口

### frps

| 默认宿主机端口 | 容器端口 | 用途                        |
| -------------- | -------- | --------------------------- |
| 7000/tcp       | 7000/tcp | frpc 主连接                 |
| 7000/udp       | 7000/udp | KCP/QUIC 可选入口           |
| 80/tcp         | 8080/tcp | HTTP Vhost                  |
| 443/tcp        | 8443/tcp | HTTPS Vhost                 |
| 7500/tcp       | 7500/tcp | 服务端管理面板和 `/healthz` |
| 6000/tcp       | 6000/tcp | 可选 TCP 代理远程端口       |
| 6000/udp       | 6000/udp | 可选 UDP 代理远程端口       |

6000 不会由默认 Compose 发布，也不会自动创建代理；只有文档中的
`docker run -p` 示例或用户自行调整 Compose 时才映射。

### frpc

| 默认宿主机端口 | 容器端口 | 用途                        |
| -------------- | -------- | --------------------------- |
| 7400/tcp       | 7400/tcp | 客户端管理面板和 `/healthz` |

`docker run` 的宿主机端口完全由 `-p 宿主机端口:容器端口` 决定，修改端口映射
后必须删除并重建容器。`FRPS_BIND_PORT`、`FRPS_HTTP_PORT`、
`FRPS_HTTPS_PORT`、`FRPS_DASHBOARD_PORT`、`FRPC_DASHBOARD_PORT` 只用于
Compose 发布端口，不等同于进程内部端口。

## 目录挂载

frps 和 frpc 都使用相同的三个容器目录，配置目录必须整体挂载，不能只挂载单个
TOML 文件：

| 容器目录       | Docker Run bind mount          | 用途                                      |
| -------------- | ------------------------------ | ----------------------------------------- |
| `/etc/frp`     | `$HOME/frp/<role>/config`      | TOML 主配置、include 文件、配置历史       |
| `/var/log/frp` | `$HOME/frp/<role>/logs`        | frps/frpc 文件日志                        |
| `/var/lib/frp` | `$HOME/frp/<role>/data`        | 管理会话、Store、证书、捕获记录、诊断设置 |

如果不希望处理宿主机 uid/gid，可以使用 named volume：

```bash
docker volume create frps-config
docker volume create frps-logs
docker volume create frps-data

# 将对应 docker run 参数替换为：
-v frps-config:/etc/frp
-v frps-logs:/var/log/frp
-v frps-data:/var/lib/frp
```

named volume 适合快速部署，bind mount 更适合备份和直接编辑。使用源码构建时，
Compose 默认挂载 `deploy/<role>/{config,logs,data}`。

## 核心环境变量

### frps

| 变量                                  | 必填         | 说明                                   |
| ------------------------------------- | ------------ | -------------------------------------- |
| `FRP_ADMIN_USERNAME`                  | 首次启动     | 面板管理员用户名                       |
| `FRP_ADMIN_PASSWORD`                  | 首次启动     | 认证文件不存在时用于初始化管理员密码   |
| `FRP_ADMIN_PASSWORD_FILE`             | 否           | 从文件读取管理员密码，优先于直接变量   |
| `FRP_AUTH_TOKEN`                      | 首次生成配置 | frpc/frps 隧道认证 token               |
| `FRP_AUTH_TOKEN_FILE`                 | 否           | 从文件读取 token，优先于直接变量       |
| `FRP_LOG_LEVEL`                       | 否           | 生成配置时的日志级别，默认 `info`      |
| `FRP_LOG_MAX_DAYS`                    | 否           | 日志保留天数，默认 `7`                 |
| `FRP_REGENERATE_CONFIG`               | 否           | 设为 `1` 时按环境变量重建已有主配置    |
| `FRPS_BIND_PORT`                      | 否           | 宿主机 frps 控制端口，默认 7000        |
| `FRPS_HTTP_PORT`                      | 否           | 宿主机 HTTP Vhost 端口，默认 80        |
| `FRPS_HTTPS_PORT`                     | 否           | 宿主机 HTTPS Vhost 端口，默认 443      |
| `FRPS_DASHBOARD_PORT`                 | 否           | 宿主机面板端口，默认 7500              |
| `FRP_ALLOW_DASHBOARD_NETWORK_CHANGES` | 否           | 是否允许面板修改 WebServer 网络字段    |

### frpc

| 变量                  | 必填         | 说明                                   |
| --------------------- | ------------ | -------------------------------------- |
| `FRP_ADMIN_USERNAME`  | 首次启动     | 面板管理员用户名                       |
| `FRP_ADMIN_PASSWORD`  | 首次启动     | 认证文件不存在时用于初始化管理员密码   |
| `FRP_ADMIN_PASSWORD_FILE` | 否       | 从文件读取管理员密码，优先于直接变量   |
| `FRP_AUTH_TOKEN`      | 首次生成配置 | 必须与服务端一致                       |
| `FRP_AUTH_TOKEN_FILE` | 否           | 从文件读取 token，优先于直接变量       |
| `FRP_SERVER_ADDR`     | 首次生成配置 | frps 地址                              |
| `FRP_SERVER_PORT`     | 否           | frps 端口，默认 7000                   |
| `FRP_LOG_LEVEL`       | 否           | 生成配置时的日志级别，默认 `info`      |
| `FRP_LOG_MAX_DAYS`    | 否           | 日志保留天数，默认 `7`                 |
| `FRP_REGENERATE_CONFIG` | 否         | 设为 `1` 时按环境变量重建已有主配置    |
| `FRPC_DASHBOARD_PORT` | 否           | 宿主机客户端面板端口，默认 7400        |

`FRP_SERVER_ADDR` 在 frpc 首次生成配置时必填。端口、日志保留天数和
`FRP_REGENERATE_CONFIG` 会在启动时校验，非法值会使容器以明确的错误退出。

## 源码构建（备用）

只有在需要修改代码、使用未发布分支或自行发布镜像时才需要克隆源码。

### 1. 获取源码

```bash
git clone --branch master https://github.com/lonquan/frp-control.git frp-control
cd frp-control
```

GitHub 无法访问时，可改用 CNB 克隆：

```bash
git clone --branch master https://cnb.cool/antcool/frp frp-control
cd frp-control
```

只需要最新代码、不需要完整历史时可使用浅克隆：

```bash
git clone --depth 1 --single-branch --no-tags --branch master \
  https://github.com/lonquan/frp-control.git frp-control
cd frp-control
```

以后需要完整历史时执行 `git fetch --unshallow`。生产部署不建议直接跟随
未经验证的分支更新，升级前请备份挂载目录中的配置、数据和证书。

### 2. 本地构建镜像

```bash
make docker-build
```

也可以只构建单个角色：

```bash
make docker-build-frps
make docker-build-frpc
```

中国大陆网络环境可使用独立的构建配置：

```bash
make docker-build-frps-cn
make docker-build-frpc-cn
```

`deploy/build.cn.env` 只影响 Docker 构建源，不会与运行配置合并。

### 3. 使用 Compose 启动

服务端：

```bash
cp deploy/frps/.env.example deploy/frps/.env
# 修改管理员密码和 FRP_AUTH_TOKEN
docker compose -f deploy/frps/compose.yaml up -d
docker compose -f deploy/frps/compose.yaml ps
```

客户端：

```bash
cp deploy/frpc/.env.example deploy/frpc/.env
# 修改管理员密码、FRP_AUTH_TOKEN、FRP_SERVER_ADDR 和 FRP_SERVER_PORT
docker compose -f deploy/frpc/compose.yaml up -d
docker compose -f deploy/frpc/compose.yaml ps
```

Compose 首次启动也会根据环境变量生成最小配置。目录权限、端口和完整变量说明见
[完整 Docker 文档](deploy/README.md)。

### 4. Docker Hub 自动发布

推送形如 `v0.0.1` 的 Git 标签会触发
`.github/workflows/docker-publish.yml`。工作流先执行 Web CI、Go 单测和
`go vet`，通过后构建 amd64/arm64 镜像并推送：

| 发布标签 | 用途                         |
| -------- | ---------------------------- |
| `vX.Y.Z` | 固定版本，推荐生产使用       |
| `latest` | 最新正式版本，预发布不更新   |

发布仓库需要在 GitHub Actions Secrets 中配置 `DOCKERHUB_USERNAME` 和
`DOCKERHUB_TOKEN`；token 应使用 Docker Hub PAT，而不是账号密码。

## 管理面板

### 认证与安全

- 管理员密码使用 Argon2id 哈希保存。
- 浏览器会话使用 HttpOnly、SameSite Cookie。
- 所有修改请求必须携带 CSRF Token。
- 登录失败按来源 IP 和用户名限流。
- 面板不使用 TOTP。
- `webServer.user/password` 和 Web Basic Auth 已移除，旧字段会导致配置校验失败。
- 直接暴露公网时，应使用 HTTPS 反向代理，或为 `webServer.tls` 配置证书。

修改面板密码后，环境变量中的密码不会覆盖现有认证文件。忘记密码时，停止容器，
删除对应的 `data/admin/auth.json`，再重新启动容器，即可重新使用启动参数中的
管理员密码初始化账号。

### 配置中心

frps 支持：

- 在线编辑 TOML
- 服务端严格校验
- 保存配置版本历史
- 保存并重启
- Supervisor 启动失败自动回滚

frpc 支持：

- 主 `frpc.toml`
- `includes` 引入的多个 TOML 文件
- 创建、重命名、拆分、编辑和删除 include 文件
- 配置校验和历史版本
- 保存并热加载
- 公共配置变化后的完整重启

配置加载支持 TOML、YAML 和 JSON。legacy INI 格式已移除；`.ini` 文件会返回
明确的迁移错误，新面板只编辑 TOML。

默认禁止在线修改会切断面板连接的网络字段，包括 `webServer.addr`、
`webServer.port`、`webServer.tls` 和 `webServer.assetsDir`。确实需要修改时，
在 `.env` 中临时设置：

```dotenv
FRP_ALLOW_DASHBOARD_NETWORK_CHANGES=1
```

修改完成并确认 Docker 端口映射后，建议恢复为 `0`。

完整配置样例：

- [frps_full_example.toml](conf/frps_full_example.toml)
- [frpc_full_example.toml](conf/frpc_full_example.toml)

### 代理和访问者

客户端“代理”和“访问者”页面支持可视化创建、编辑、启停和删除 Store 配置。
可视化编辑依赖以下配置：

```toml
store.path = "/var/lib/frp/frpc_store.json"
```

容器首次生成的 `frpc.toml` 已包含该设置。已有配置必须保留或增加 Store 配置，
才能使用可视化代理管理。

### ACME 证书

客户端面板支持：

- DNS-01
- 单域名、多域名和通配符证书
- Let's Encrypt 正式环境、staging 和自定义 ACME CA
- RSA 2048、RSA 4096、ECDSA P-256、ECDSA P-384
- 到期前自动续期
- 续期失败时保留上一份有效证书
- 续期成功后自动重载 HTTP/HTTPS/WSS 插件

DNS 提供商由固定版本的 lego 提供，支持该版本注册的全部 DNS 提供商。DNS 凭据
不通过面板录入，而是从 `deploy/frpc/.env` 读取对应环境变量。

HTTP-01 已移除，所有托管证书统一通过 DNS-01 签发。可用证书目录仅返回文件
可读、密钥匹配且未过期的证书；代理表单默认选择可用证书，也允许切换为手工
证书路径或空路径自签证书。托管证书通过 `certificateId` 和
`frp.certificate.id` metadata 建立引用，后端自动写入规范化文件路径。

`customDomains` 必须被证书 SAN 覆盖，支持 `*.example.com` 一级通配符；
`subdomain` 的最终域名依赖 frps `subDomainHost`，面板会显示人工确认警告。
删除仍被在线、离线或禁用代理引用的证书会返回 `409 Conflict`。

### 流量捕获与重放

捕获默认关闭，需要在“流量捕获”页面按代理启用。

支持捕获：

- HTTP 代理直接访问本地 HTTP 服务
- HTTPS + `https2http`
- HTTP/HTTPS 转换插件的 HTTP 请求和响应
- WSS 握手及升级后的 WebSocket 消息帧

列表不显示 Authorization、Cookie 等敏感请求头，详情默认遮蔽。点击“显示敏感
内容”后才显示原值，并写入包含用户名、时间和来源 IP 的审计日志。

捕获数据包含完整请求头、Cookie、认证信息和正文，并以明文保存在
`deploy/frpc/data/captures`，必须保护好数据集目录。

默认限制：

- TTL：24 小时
- 单条捕获：16 MiB
- 总容量：500 MiB
- WebSocket：消息帧总负载计入单条捕获上限

超过容量时优先删除最旧记录。内容被截断的捕获不能重放。重放只允许发送到捕获
时相同的代理后端；后端变化后会拒绝重放。WebSocket 消息帧只支持查看，不支持
重放。面板不提供捕获数据的导出或备份功能。

### 监控与诊断

监控和运行指标在会话保护的面板中展示。独立的 `metricsServer`、Prometheus
`/metrics`、`enablePrometheus` 和公开 `/debug/pprof/*` 已移除。

服务端运行信息与性能诊断已合并到总览页面，客户端仍使用独立诊断页面。诊断默认
关闭，管理员显式开启后，可在会话内获取 goroutine、heap 和 CPU profile 下载。
`/healthz` 保持匿名可访问，供 Docker 和负载均衡器做存活检查。

## Docker 中的本地后端地址

frpc 容器内的 `127.0.0.1` 指向容器自身，不是宿主机。

- macOS / Windows Docker Desktop：使用 `host.docker.internal`
- 同一个 Docker Compose 项目中的服务：使用 Compose 服务名
- Linux 访问宿主机：Docker Run 示例和 Compose 都使用
  `host.docker.internal:host-gateway`

例如代理宿主机 `18888` 端口的服务：

```toml
[[proxies]]
name = "web"
type = "http"
localIP = "host.docker.internal"
localPort = 18888
customDomains = ["app.example.com"]
```

## 常用运维

直接运行镜像时，查看状态和日志：

```bash
docker ps --filter name=frps
docker ps --filter name=frpc
docker logs --tail=200 frps
docker logs --tail=200 frpc
```

重启、停止和删除容器：

```bash
docker restart frps
docker restart frpc

docker stop frps frpc
docker rm frps frpc
```

删除容器不会删除 bind mount 或 named volume 中的数据。重新执行 `docker run`
前先确认配置、数据和日志目录仍使用原先的挂载。

源码构建并使用 Compose 时：

```bash
docker compose -f deploy/frps/compose.yaml ps
docker compose -f deploy/frps/compose.yaml logs --tail=200

docker compose -f deploy/frpc/compose.yaml ps
docker compose -f deploy/frpc/compose.yaml logs --tail=200
```

重启容器：

```bash
docker compose -f deploy/frps/compose.yaml restart
docker compose -f deploy/frpc/compose.yaml restart
```

停止：

```bash
docker compose -f deploy/frps/compose.yaml down
docker compose -f deploy/frpc/compose.yaml down
```

### Docker Hub 镜像更新

更新前先备份 `$HOME/frp/frps/{config,data}` 或对应 named volume，并记录当前
镜像摘要：

```bash
docker image inspect lonquan/frp-frps:v0.0.1 \
  --format '{{index .RepoDigests 0}}'

docker pull lonquan/frp-frps:v0.0.2
docker stop frps
docker rename frps frps-previous
```

使用新版本标签重新执行前面的 `docker run` 命令，保留原配置、数据和日志挂载。
确认面板、`/healthz` 和代理连接正常后再删除回退容器：

```bash
docker inspect --format '{{json .State.Health.Status}}' frps
docker logs --tail=200 frps
docker rm frps-previous
```

客户端升级相同：先拉取新版本，保留原挂载并重新创建 `frpc`。旧容器可在验证
失败时用于回退。

### 源码构建方式更新服务端

更新前请自行备份 `deploy/frps/.env`、`deploy/frps/config` 和
`deploy/frps/data`。确认工作区干净后，在仓库根目录执行：

```bash
git status --short

STAMP="$(date +%Y%m%d-%H%M%S)"
docker image tag frp-control-frps:local "frp-control-frps:rollback-$STAMP"

git fetch origin
git pull --ff-only origin master

make docker-build-frps
docker compose -f deploy/frps/compose.yaml config --quiet
docker compose -f deploy/frps/compose.yaml up -d --force-recreate

docker compose -f deploy/frps/compose.yaml ps
docker compose -f deploy/frps/compose.yaml exec -T frps /usr/bin/frps --version
docker compose -f deploy/frps/compose.yaml logs --tail=200 frps
```

国内网络可将构建命令换成 `make docker-build-frps-cn`。本地源码构建默认只显示
frp 上游版本；发布流水线通过 `FRP_VERSION` 写入镜像分发版本。更新失败时，用
自行准备的备份和 `frp-control-frps:rollback-$STAMP` 镜像恢复后重新启动容器。

GitHub 无法访问时，将上面的 `origin` 替换为 `cnb`：

```bash
git fetch cnb
git pull --ff-only cnb master
```

### 源码构建方式更新客户端

重新构建并替换 frpc 容器：

```bash
make docker-build-frpc
docker compose -f deploy/frpc/compose.yaml up -d --force-recreate
```

面板发起重启失败时，检查：

- `<config>.last-good`
- `<config>.failed-<时间>`
- `docker logs frpc` 或 `docker compose logs`

## 已移除的旧管理能力

以下上游旧能力已从本定制版移除：

- 带 Basic Auth 的旧 Web 管理界面
- legacy INI 配置解析
- `frpc reload`、`frpc status`、`frpc stop` 管理 CLI
- `pkg/sdk/client` 旧 frpc 管理 API 客户端
- 独立 `metricsServer` 监听
- `webServer` 上的公开 Prometheus `/metrics`
- 公开 pprof 路由

`frps verify` 和 `frpc verify` 仍可用于离线检查配置。运行期配置管理和重启请
使用面板。

## 本地开发

要求 Go 1.26 和 Node.js 22 或更高版本。

```bash
make web
make build
make test
make vet
make web-ci
make docker-entrypoint-test
```

单独运行前端：

```bash
cd web
npm ci
npm run dev --workspace frps
npm run dev --workspace frpc
```

前端检查：

```bash
cd web
npm run type-check --workspace frps
npm run type-check --workspace frpc
npm run lint:check --workspace frps
npm run lint:check --workspace frpc
npm run test:unit
```

构建 Docker 镜像：

```bash
make docker-build
```

当前主要验收覆盖 Go 单元测试、`go vet`、前端类型检查/Lint/Vitest/生产构建，
以及真实 Docker HTTP、HTTPS2HTTP、WSS、捕获、重放和 Supervisor 重启链路。
兼容性 e2e 已按当前会话认证和 v2 管理接口调整。

## 兼容性与升级说明

- 代理类型和主要数据面协议继续沿用上游实现。
- 管理面板只支持会话认证，不再接受管理 Web Basic Auth。
- legacy INI 配置已不再加载，请先转换为 TOML、YAML 或 JSON。
- `webServer.user/password/pprofEnable`、`metricsServer` 和
  `enablePrometheus` 已从配置类型中删除；旧配置包含这些字段时会返回迁移错误。
- 依赖可视化编辑时必须保留 `store.path`。
- 升级前备份 `$HOME/frp/frps/{config,data}`、`$HOME/frp/frpc/{config,data}`，
  或源码部署下的 `deploy/frps/{config,data}`、`deploy/frpc/{config,data}`。
- 上游 frp 的通用代理配置文档仍可参考，但管理面板相关章节不适用于本仓库。

## 相关文档

- [完整 Docker 部署、运维与安全说明](deploy/README.md)
- [上游 frp 中文文档](https://gofrp.org/zh-cn/docs/)
- [上游 frp 仓库](https://github.com/fatedier/frp)
- [本定制仓库 GitHub 地址](https://github.com/lonquan/frp-control)
- [本定制仓库 CNB 备用地址](https://cnb.cool/antcool/frp)
- [Apache License 2.0](LICENSE)
