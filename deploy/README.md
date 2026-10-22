# frp Docker 定制版使用文档

本部署方案提供两个完全独立的公网服务端和内网客户端镜像：

- `lonquan/frp-frps`：公网服务端
- `lonquan/frp-frpc`：客户端，支持 Linux、macOS 和 Windows Docker Desktop

对应的源码 Compose 项目位于 `deploy/frps` 和 `deploy/frpc`，只在自行构建时使用。

两个镜像均提供 Linux/amd64 与 Linux/arm64，并发布到 Docker Hub：

- `lonquan/frp-frps`
- `lonquan/frp-frpc`

推荐直接拉取固定版本 `v0.0.2` 并通过 `docker run` 启动。支持 Docker Desktop
的 Linux 容器，不支持原生 Windows 容器。

前端由 Dockerfile 内的 Node.js 多阶段构建生成，不要求宿主机安装 Node.js，
也不依赖本地 `web/*/dist`。只有修改源码或自行发布镜像时才需要克隆仓库并构建。

## 从 Docker Hub 直接运行（推荐）

镜像分发版本与内置 frp 上游版本独立演进。当前文档示例版本为 `v0.0.2`，
`latest` 会随正式版本移动，生产环境应固定版本。

```bash
docker pull lonquan/frp-frps:v0.0.2
docker pull lonquan/frp-frpc:v0.0.2
```

版本查看：

```bash
docker run --rm --entrypoint /usr/bin/frps \
  lonquan/frp-frps:v0.0.2 --version
docker run --rm --entrypoint /usr/bin/frpc \
  lonquan/frp-frpc:v0.0.2 --version
```

输出格式为 `0.0.2 (frp 0.71.1)`。

### 启动 frps

```bash
TOKEN="$(openssl rand -hex 32)"
mkdir -p "$HOME/frp/frps/config" "$HOME/frp/frps/logs" "$HOME/frp/frps/data"
sudo chown -R 10001:10001 "$HOME/frp/frps/config" \
  "$HOME/frp/frps/logs" "$HOME/frp/frps/data"

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
  -e FRP_AUTH_TOKEN="$TOKEN" \
  -v "$HOME/frp/frps/config:/etc/frp" \
  -v "$HOME/frp/frps/logs:/var/log/frp" \
  -v "$HOME/frp/frps/data:/var/lib/frp" \
  --log-opt max-size=20m \
  --log-opt max-file=5 \
  lonquan/frp-frps:v0.0.2
```

Linux 以外的 Docker Desktop 环境通常不需要 `chown`。如果系统没有 `sudo` 或
目录由其他用户管理，请确保容器 UID/GID 10001 对三个目录可写。

### 启动 frpc

```bash
TOKEN='与服务端完全一致的 token'
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
  -e FRP_AUTH_TOKEN="$TOKEN" \
  -e FRP_SERVER_ADDR='your-server.example.com' \
  -e FRP_SERVER_PORT=7000 \
  -v "$HOME/frp/frpc/config:/etc/frp" \
  -v "$HOME/frp/frpc/logs:/var/log/frp" \
  -v "$HOME/frp/frpc/data:/var/lib/frp" \
  --log-opt max-size=20m \
  --log-opt max-file=5 \
  lonquan/frp-frpc:v0.0.2
```

### Docker secrets

直接环境变量和 `_FILE` 同时设置时文件优先：

```bash
mkdir -p "$HOME/frp/secrets"
printf '%s' '管理员密码' > "$HOME/frp/secrets/admin-password"
printf '%s' '认证 token' > "$HOME/frp/secrets/auth-token"
chmod 0600 "$HOME/frp/secrets/"*
```

在 `docker run` 中使用：

```text
-e FRP_ADMIN_PASSWORD_FILE=/run/secrets/frp_admin_password
-e FRP_AUTH_TOKEN_FILE=/run/secrets/frp_auth_token
-v "$HOME/frp/secrets/admin-password:/run/secrets/frp_admin_password:ro"
-v "$HOME/frp/secrets/auth-token:/run/secrets/frp_auth_token:ro"
```

认证文件 `/var/lib/frp/admin/auth.json` 已存在时，
`FRP_ADMIN_PASSWORD`/`FRP_ADMIN_PASSWORD_FILE` 不再必填。

也可以将运行变量写入权限为 `0600` 的 env 文件，并通过 `--env-file` 传入。
宿主机端口不能用该文件代替 `-p`。

### 配置生成与重建

- 配置不存在时，使用环境变量生成最小 TOML。
- 已有 TOML 默认不覆盖。
- `FRP_REGENERATE_CONFIG=1` 时按环境变量重建，并备份为
  `<config>.env-backup-<UTC时间>`。
- `FRP_LOG_LEVEL` 和 `FRP_LOG_MAX_DAYS` 只在生成配置时生效。
- 修改 token 或服务器地址后，frps 和 frpc 两侧都要显式重建配置。

## 源码构建（备用）

需要修改源码、使用未发布分支或自行发布镜像时，再从仓库根目录构建。

只需要 `master` 分支的最新代码、不需要完整提交历史时，可以使用浅克隆：

```bash
git clone --depth 1 --single-branch --no-tags --branch master \
  https://github.com/lonquan/frp-control.git frp-control
cd frp-control
```

GitHub 无法访问时，可改用 CNB 浅克隆：

```bash
git clone --depth 1 --single-branch --no-tags --branch master \
  https://cnb.cool/antcool/frp-control frp-control
cd frp-control
```

以上命令固定使用 `master` 分支。克隆后可确认仓库为浅克隆：

```bash
git rev-parse --is-shallow-repository
```

以后需要完整历史时执行 `git fetch --unshallow`。

Docker 构建默认使用 Docker Hub、npm、Go 和 Alpine 的官方源。中国大陆网络环境可使用
仓库中独立的 `deploy/build.cn.env`，它只参与 Docker 构建，不会与
`deploy/frps/.env`、`deploy/frpc/.env` 中的运行配置合并：

```bash
make docker-build-frps-cn
make docker-build-frpc-cn
```

也可以直接执行：

```bash
docker compose --env-file deploy/build.cn.env \
  -f deploy/frps/compose.yaml build
```

公共容器镜像代理可能随网络环境变化，可直接修改 `deploy/build.cn.env` 中的完整镜像名称。
切换源后若仍使用旧层，可执行 `docker compose ... build --pull --no-cache`。

## 1. 目录与挂载

Docker Hub 镜像示例默认挂载 `$HOME/frp/<role>/{config,logs,data}`；源码
Compose 默认挂载 `deploy/<role>/{config,logs,data}`。也可以使用以下 named
volume：

```bash
docker volume create frps-config
docker volume create frps-logs
docker volume create frps-data
```

三种容器目录用途相同：

| 容器目录       | 用途                                        |
| -------------- | ------------------------------------------- |
| `/etc/frp`     | TOML 主配置、include 配置、配置版本         |
| `/var/log/frp` | frps/frpc 文件日志                          |
| `/var/lib/frp` | 管理员会话、证书、捕获记录、动态配置、诊断设置 |

配置必须挂载整个目录，不能只挂载单个 TOML 文件。面板需要原子替换配置并创建 include 文件。

首次启动时，如果配置目录为空，容器会生成最小配置：

- frps：监听 7000，HTTP Vhost 8080，HTTPS Vhost 8443，面板 7500。
- frpc：面板 7400，动态配置启用，登录失败后保持面板可用。

配置目录为空时必须提供 `FRP_AUTH_TOKEN`；认证文件
`/var/lib/frp/admin/auth.json` 不存在时必须提供 `FRP_ADMIN_PASSWORD` 或
`FRP_ADMIN_PASSWORD_FILE`。frpc 首次生成配置时还需要 `FRP_SERVER_ADDR`。

## 2. 源码构建方式启动服务端

```bash
cd deploy/frps
cp .env.example .env
```

编辑 `.env`，至少替换：

```dotenv
FRP_ADMIN_USERNAME=admin
FRP_ADMIN_PASSWORD=请替换为长密码
FRP_AUTH_TOKEN=请替换为随机长令牌
```

生成 token 可用：

```bash
openssl rand -hex 32
```

Docker Desktop 在 macOS 上通常不需要手动 chown。Linux VPS 建议先创建目录并设置容器用户权限：

```bash
mkdir -p config logs data
sudo chown -R 10001:10001 config logs data
```

Windows Docker Desktop 使用相对 bind mount 即可，不需要 Linux `chown`；请确保 Docker Desktop 对项目目录具有文件共享权限。

从仓库根目录构建并启动：

```bash
make docker-build-frps
docker compose -f deploy/frps/compose.yaml up -d
docker compose -f deploy/frps/compose.yaml ps
docker compose -f deploy/frps/compose.yaml logs -f frps
```

访问：

```text
http://服务器IP:7500
```

首次登录使用 `.env` 中的管理员账号和密码。修改面板密码后，`.env` 中的密码仅作为认证文件首次初始化使用。

默认端口：

| 宿主机端口 | 容器端口 | 用途          |
| ---------- | -------- | ------------- |
| 7000/tcp   | 7000/tcp | frpc 主连接   |
| 7000/udp   | 7000/udp | KCP/QUIC 可选 |
| 80         | 8080     | HTTP Vhost    |
| 443        | 8443     | HTTPS Vhost   |
| 7500       | 7500     | 管理面板      |
| 6000/tcp   | 6000/tcp | 可选 TCP 代理 |
| 6000/udp   | 6000/udp | 可选 UDP 代理 |

6000 不会由默认 Compose 自动发布，只在配置对应代理时添加端口。如宿主机端口
不同，修改 `.env` 后重建容器：

```bash
docker compose up -d --force-recreate
```

## 3. 源码构建方式启动客户端

```bash
cd deploy/frpc
cp .env.example .env
```

编辑 `.env`：

```dotenv
FRP_ADMIN_PASSWORD=客户端管理员密码
FRP_AUTH_TOKEN=与服务端完全一致的 token
FRP_SERVER_ADDR=your-server.example.com
FRP_SERVER_PORT=7000
```

Linux 或 Docker Desktop 都建议执行：

```bash
mkdir -p config logs data
sudo chown -R 10001:10001 config logs data
```

从仓库根目录构建并启动：

```bash
make docker-build-frpc
docker compose -f deploy/frpc/compose.yaml up -d
docker compose -f deploy/frpc/compose.yaml ps
docker compose -f deploy/frpc/compose.yaml logs -f frpc
```

访问：

```text
http://127.0.0.1:7400
```

### Docker 中的本地后端地址

frpc 容器内的 `127.0.0.1` 指向容器自身，不是宿主机。

- macOS / Windows Docker Desktop：使用 `host.docker.internal`
- 另一个 Docker Compose 服务：使用服务名，例如 `web`
- Linux 上访问宿主机：Compose 已提供 `host.docker.internal:host-gateway`

代理表单位置会提示使用 `host.docker.internal`。

## 4. 管理面板

浏览器只使用管理员会话认证：

- Argon2id 密码哈希
- HttpOnly/SameSite 会话 Cookie
- CSRF Token
- 登录失败限流
- 默认 7 天会话

面板不使用 TOTP。Basic Auth、`frpc reload/status/stop` 管理 CLI、`pkg/sdk/client`、Prometheus 和标准 pprof URL 均不再提供。

### 忘记密码

停止对应容器，删除宿主机认证文件后重启：

```bash
docker compose down
rm data/admin/auth.json
docker compose up -d
```

Docker Run 部署时，认证文件位于
`$HOME/frp/<role>/data/admin/auth.json`；停止并删除容器后删除该文件，再用
`FRP_ADMIN_USERNAME` 和 `FRP_ADMIN_PASSWORD` 重新创建。已有会话会全部失效。
源码 Compose 部署重启后会重新读取 `.env` 中的管理员变量。

## 5. 配置中心

frps 提供原始 TOML 编辑、校验、差异历史和重启应用。

frpc 提供：

- 主 `frpc.toml`
- `includes` 引入的多个 TOML 文件
- 新建、命名、拆分、编辑和删除文件
- 服务端严格校验
- 最近 20 个内部历史版本
- 保存并热加载
- 公共配置变化后的完整重启

所有文件限制在容器内 `/etc/frp`。源码 Compose 对应宿主机
`deploy/frpc/config`，Docker Run 对应 `$HOME/frp/frpc/config`。禁止路径穿越和
符号链接越界。

配置加载支持 TOML、YAML 和 JSON。legacy INI 格式已移除，`.ini` 文件会返回
迁移错误，新面板只负责编辑 TOML。

旧配置中的 `webServer.user`、`webServer.password`、`webServer.pprofEnable`、
`metricsServer` 和 `enablePrometheus` 也已移除。保存前必须删除这些字段，
面板校验器会返回明确错误。

### 面板网络字段保护

默认禁止在线修改：

- `webServer.addr`
- `webServer.port`
- `webServer.tls`
- `webServer.assetsDir`

如需修改，在 `.env` 设置：

```dotenv
FRP_ALLOW_DASHBOARD_NETWORK_CHANGES=1
```

修改后可能失去面板访问，需要同步确认 Docker 端口映射。修改完成后建议恢复为 `0`。

## 6. 代理配置

面板支持全部上游代理类型：

- TCP
- UDP
- HTTP
- HTTPS
- TCPMUX
- STCP
- XTCP
- SUDP

以及全部访问者和客户端插件。每种类型均可从表单选择，也可以直接编辑 TOML。

可视化创建依赖底层 Store。面板中将其称为“动态配置”，首次生成的 frpc 配置已经启用：

```toml
store.path = "/var/lib/frp/frpc_store.json"
```

如果使用已有配置，请保留或增加 Store 配置段。

代理和访问者页面均分为“状态”和“配置”两个列表。上方状态只展示运行实例，
下方配置包含配置文件和动态配置；动态配置行可直接启停，文件配置需前往配置中心修改。

### TCP/UDP 6000 端口

TCP 或 UDP 代理中的 `remotePort = 6000` 由 **frps** 监听。使用 Docker 时必须
在 frps 启动参数中发布：

```bash
-p 6000:6000/tcp
-p 6000:6000/udp
```

frpc 不需要发布 6000。它只需要能够出站连接 frps，并通过
`host.docker.internal`、共享 Docker 网络或 Compose 服务名访问本地后端。新增
远程端口后必须重新创建 frps 容器，Docker 不会自动增加已有容器的端口映射。

## 7. ACME 证书

客户端镜像包含独立 `frp-acme` 进程，固定使用 lego v5.5.2，并支持该版本的全部 DNS 提供商。

支持：

- DNS-01
- 通配符证书
- Let's Encrypt 正式环境
- Let's Encrypt staging 或自定义 CA
- RSA 2048、RSA 4096、ECDSA P-256、ECDSA P-384
- 到期前 30 天自动续期
- 续期失败时保留上一份有效证书
- 单证书续期、全部续期和证书/私钥 ZIP 下载

### DNS-01

DNS 凭据在客户端控制台的“证书 → DNS 设置”中录入。配置保存在
`/var/lib/frp/acme/dns-provider.json`，文件权限为 0600；不需要修改
`deploy/frpc/.env`。保存前会在 `frp-acme` 中构造 provider 做预检。

提供商名称和凭据字段以固定版本 lego 元数据为准：

<https://go-acme.github.io/lego/dns/>

面板支持除 `manual` 和 `exec` 外的全部 lego DNS provider。旧环境变量中的 DNS
凭据不会自动迁移，也不会作为文件配置的兜底。

HTTP-01 已完全移除，证书签发只接受 DNS-01。证书文件写入
`/var/lib/frp/acme/certificates/<id>/`。代理表单默认从可用证书目录选择托管
证书，也可以切换为手工路径模式。托管证书通过 `certificateId` 和
`frp.certificate.id` metadata 建立引用，证书续期成功后会自动重载相关插件。

新建证书后立即签发，证书名称不可重复，证书内容创建后不可修改。单行“续期”
只处理当前证书，“全部续期”串行处理所有非签发中证书。下载按钮生成包含证书
和私钥的 ZIP，请限制客户端数据目录和下载文件的访问权限。

可用证书目录只返回证书和私钥可读取、密钥匹配且未过期的证书。显式
`customDomains` 必须被证书 SAN 覆盖；`subdomain` 因依赖 frps 的
`subDomainHost`，面板会提示人工确认。删除仍被代理引用的证书会返回冲突错误。

## 8. 流量捕获与重放

捕获默认关闭。可在“流量捕获”页面按代理启用。

捕获范围：

- `type=http` 直连本地服务
- `type=https + https2http`
- HTTP/HTTPS2HTTP 的其他 HTTP 桥插件
- WSS 握手及升级后的文本、二进制、控制帧

请求和响应完整保存，包括 Authorization、Cookie 和正文。存储为明文，目录权限由容器和宿主机挂载控制。

捕获设置支持排除关键字，每行一条，不区分大小写匹配 `Path` 和 `Query`，支持
`*` 和 `?` 通配符。例如 `favicon.ico` 或 `/static/*`。命中的请求会正常转发，
但不会读取正文或保存捕获记录。

默认限制：

- TTL：24 小时
- 单条：16 MiB
- 总容量：500 MiB

超限时按时间淘汰最旧记录。请求过大时标记为“截断”，截断记录不允许重放。

列表不显示 Authorization、Cookie 等敏感请求头。详情默认遮蔽，点击“显示敏感内容”后查看原值，并记录审计。

重放只允许发送到捕获时对应的原本地后端。当前代理后端变化后会拒绝重放。
WebSocket 消息帧只查看，不重放。重放结果保存在原捕获下，最多保留最近 20 次，
不会生成新的捕获列表记录。捕获记录保存 FRPS 观察到的来源地址和端口，业务后端
会收到使用真实来源 IP 覆盖的 `X-Forwarded-For`。

捕获不提供导出、下载或备份功能。

## 9. 日志

默认文件日志：

- 服务端：`deploy/frps/logs/frps.log`
- 客户端：`deploy/frpc/logs/frpc.log`

Docker Run bind mount 部署对应
`$HOME/frp/frps/logs/frps.log` 和 `$HOME/frp/frpc/logs/frpc.log`。

示例配置：

```toml
log.to = "/var/log/frp/frps.log"
log.level = "info"
log.maxDays = 7
```

Supervisor 生命周期日志写入 Docker stdout：

```bash
docker logs -f frps
docker logs -f frpc
docker compose logs -f
```

Compose 和文档中的 Docker Run 示例都将 Docker JSON 日志限制为 5 个文件、
每个 20 MiB。

## 10. Supervisor 与自动回滚

容器 PID 1 是 `frp-supervisor`。它负责：

- 启动 frps/frpc
- 监控退出并重启
- 处理后端服务重启请求
- 等待子进程 ready 标记
- 新版配置启动失败时恢复 `<config>.last-good`
- 将失败配置保留为 `<config>.failed-<时间>`

面板修改配置后，重启过程中可能出现数秒连接中断。页面会轮询 `/healthz` 并提示恢复结果。

## 11. 安全与限制

- 容器默认以 UID/GID 10001 运行。
- 根文件系统只读，`/tmp` 和 `/run` 使用 tmpfs。
- 丢弃全部 Linux capabilities。
- 容器只发布必要端口。
- WebServer Basic Auth 已移除。
- 管理 API 仅接受面板会话。
- 直接通过公网访问管理面板时应在前方配置 HTTPS 反向代理，或为 `webServer` 配置 TLS；会话 Cookie 在 HTTPS 请求下会自动添加 Secure 标记。
- 独立 metricsServer、Prometheus `/metrics` 和外部 `/debug/pprof/*` 已移除。
- 服务端总览已合并运行指标、诊断和代理状态；客户端诊断仍在独立页面展示。pprof
  默认关闭，开启后只能通过会话 API 下载。
- Docker 无法自动更新宿主机端口映射，端口变化必须修改 `docker run -p` 或
  Compose 配置并重建容器。

## 12. 常用运维

直接运行镜像时查看状态：

```bash
docker ps --filter name=frps
docker ps --filter name=frpc
docker logs --tail=200 frps
docker logs --tail=200 frpc
```

重启和停止：

```bash
docker restart frps frpc
docker stop frps frpc
```

源码 Compose 方式查看状态：

```bash
docker compose ps
docker compose logs --tail=200
```

重启：

```bash
docker compose restart
```

停止：

```bash
docker compose down
```

升级本地代码后重新构建：

```bash
docker compose build --no-cache
docker compose up -d --force-recreate
```

进入容器执行上游只读/调试命令时，管理重启请使用面板：

```bash
docker compose exec frps frps verify -c /etc/frp/frps.toml
docker compose exec frpc frpc verify -c /etc/frp/frpc.toml
```
