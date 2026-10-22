#!/bin/sh
set -eu

role="${1:-}"
case "$role" in
  frps|frpc) ;;
  *)
    echo "usage: entrypoint.sh frps|frpc" >&2
    exit 64
    ;;
esac

config_dir="${FRP_CONFIG_ROOT:-/etc/frp}"
config_path="${FRP_CONFIG_PATH:-${config_dir}/${role}.toml}"
data_dir="${FRP_DATA_DIR:-/var/lib/frp}"
log_dir="${FRP_LOG_DIR:-/var/log/frp}"
ready_file="${FRP_READY_FILE:-/run/frp/ready}"

mkdir -p "$config_dir" "$data_dir" "$log_dir" "$(dirname "$ready_file")"

read_secret_file() {
  secret_path="$1"
  secret_name="$2"
  if [ ! -f "$secret_path" ] || [ ! -r "$secret_path" ]; then
    echo "无法读取 ${secret_name}: ${secret_path}" >&2
    exit 78
  fi
  secret_value=$(cat "$secret_path")
  if [ -z "$secret_value" ]; then
    echo "${secret_name} 文件内容为空: ${secret_path}" >&2
    exit 78
  fi
  printf '%s' "$secret_value"
}

validate_single_line() {
  value="$1"
  name="$2"
  newline='
'
  carriage_return=$(printf '\r')
  case "$value" in
    *"$newline"*|*"$carriage_return"*)
      echo "${name} 不能包含换行符。" >&2
      exit 78
      ;;
  esac
}

validate_port() {
  value="$1"
  name="$2"
  case "$value" in
    ''|*[!0-9]*)
      echo "${name} 必须是 1-65535 之间的整数。" >&2
      exit 78
      ;;
  esac
  if [ "$value" -lt 1 ] || [ "$value" -gt 65535 ]; then
    echo "${name} 必须是 1-65535 之间的整数。" >&2
    exit 78
  fi
}

validate_nonnegative_integer() {
  value="$1"
  name="$2"
  case "$value" in
    ''|*[!0-9]*)
      echo "${name} 必须是非负整数。" >&2
      exit 78
      ;;
  esac
}

check_writable() {
  dir="$1"
  if ! touch "$dir/.frp-write-check" 2>/dev/null; then
    echo "目录不可写: $dir" >&2
    echo "请将宿主机目录所有权设置为容器用户 UID/GID，例如 chown -R 10001:10001 ./config ./data ./logs" >&2
    exit 73
  fi
  rm -f "$dir/.frp-write-check"
}

check_writable "$config_dir"
check_writable "$data_dir"
check_writable "$log_dir"

if [ -n "${FRP_ADMIN_PASSWORD_FILE:-}" ]; then
  FRP_ADMIN_PASSWORD=$(read_secret_file "$FRP_ADMIN_PASSWORD_FILE" "FRP_ADMIN_PASSWORD_FILE")
  export FRP_ADMIN_PASSWORD
fi
if [ -n "${FRP_AUTH_TOKEN_FILE:-}" ]; then
  FRP_AUTH_TOKEN=$(read_secret_file "$FRP_AUTH_TOKEN_FILE" "FRP_AUTH_TOKEN_FILE")
  export FRP_AUTH_TOKEN
fi

auth_file="${data_dir}/admin/auth.json"
if [ ! -f "$auth_file" ] && [ -z "${FRP_ADMIN_PASSWORD:-}" ]; then
  echo "缺少 FRP_ADMIN_PASSWORD，无法初始化管理面板。" >&2
  exit 78
fi

regenerate_config="${FRP_REGENERATE_CONFIG:-0}"
case "$regenerate_config" in
  0|1) ;;
  *)
    echo "FRP_REGENERATE_CONFIG 只能设置为 0 或 1。" >&2
    exit 78
    ;;
esac

log_level="${FRP_LOG_LEVEL:-info}"
case "$log_level" in
  trace|debug|info|warn|error) ;;
  *)
    echo "FRP_LOG_LEVEL 仅支持 trace、debug、info、warn 或 error。" >&2
    exit 78
    ;;
esac

log_max_days="${FRP_LOG_MAX_DAYS:-7}"
validate_nonnegative_integer "$log_max_days" "FRP_LOG_MAX_DAYS"

generate_config=0
if [ ! -f "$config_path" ] || [ "$regenerate_config" = "1" ]; then
  generate_config=1
fi

if [ "$generate_config" = "1" ]; then
  if [ -z "${FRP_AUTH_TOKEN:-}" ]; then
    echo "首次启动缺少 FRP_AUTH_TOKEN，无法生成安全的隧道认证配置。" >&2
    exit 78
  fi
  validate_single_line "$FRP_AUTH_TOKEN" "FRP_AUTH_TOKEN"

  temp_config="${config_path}.tmp.$$"
  trap 'rm -f "$temp_config"' EXIT HUP INT TERM

  if [ -f "$config_path" ]; then
    backup_base="${config_path}.env-backup-$(date -u +%Y%m%dT%H%M%SZ)"
    backup_path="$backup_base"
    backup_suffix=1
    while [ -e "$backup_path" ]; do
      backup_path="${backup_base}-${backup_suffix}"
      backup_suffix=$((backup_suffix + 1))
    done
    cp -p "$config_path" "$backup_path"
    echo "已备份环境变量重建前的配置: $backup_path"
  fi

  if [ "$role" = "frps" ]; then
    escaped_token=$(printf '%s' "$FRP_AUTH_TOKEN" | sed 's/\\/\\\\/g; s/"/\\"/g')
    cat > "$temp_config" <<EOF
bindAddr = "0.0.0.0"
bindPort = 7000
vhostHTTPPort = 8080
vhostHTTPSPort = 8443

auth.method = "token"
auth.token = "$escaped_token"

webServer.addr = "0.0.0.0"
webServer.port = 7500
webServer.sessionAuth.enabled = true
webServer.sessionAuth.dataDir = "/var/lib/frp"
webServer.sessionAuth.sessionTTLSeconds = 604800

log.to = "/var/log/frp/frps.log"
log.level = "$log_level"
log.maxDays = $log_max_days
EOF
  else
    server_addr="${FRP_SERVER_ADDR:-}"
    if [ -z "$server_addr" ]; then
      echo "首次启动缺少 FRP_SERVER_ADDR，无法生成客户端连接配置。" >&2
      exit 78
    fi
    validate_single_line "$server_addr" "FRP_SERVER_ADDR"
    server_port="${FRP_SERVER_PORT:-7000}"
    validate_port "$server_port" "FRP_SERVER_PORT"
    escaped_server_addr=$(printf '%s' "$server_addr" | sed 's/\\/\\\\/g; s/"/\\"/g')
    escaped_token=$(printf '%s' "$FRP_AUTH_TOKEN" | sed 's/\\/\\\\/g; s/"/\\"/g')
    cat > "$temp_config" <<EOF
serverAddr = "$escaped_server_addr"
serverPort = ${server_port}
loginFailExit = false

auth.method = "token"
auth.token = "$escaped_token"

webServer.addr = "0.0.0.0"
webServer.port = 7400
webServer.sessionAuth.enabled = true
webServer.sessionAuth.dataDir = "/var/lib/frp"
webServer.sessionAuth.sessionTTLSeconds = 604800

store.path = "/var/lib/frp/frpc_store.json"

log.to = "/var/log/frp/frpc.log"
log.level = "$log_level"
log.maxDays = $log_max_days
EOF
  fi
  chmod 0600 "$temp_config"
  mv "$temp_config" "$config_path"
  trap - EXIT HUP INT TERM
  chmod 0600 "$config_path"
  echo "已生成启动配置: $config_path"
fi

export FRP_SUPERVISOR_SOCKET="${FRP_SUPERVISOR_SOCKET:-/run/frp/supervisor.sock}"
export FRP_READY_FILE="$ready_file"
export FRP_CONFIG_PATH="$config_path"
export FRP_CONFIG_ROOT="$config_dir"
export FRP_DATA_DIR="$data_dir"

supervisor_bin="${FRP_SUPERVISOR_BIN:-/usr/bin/frp-supervisor}"

exec "$supervisor_bin" \
  --socket "$FRP_SUPERVISOR_SOCKET" \
  --ready-file "$ready_file" \
  --config "$config_path" \
  --upstream "/usr/bin/${role} -c ${config_path}"
