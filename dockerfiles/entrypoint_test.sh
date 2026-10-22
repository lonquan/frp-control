#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/frp-entrypoint.XXXXXX")
fake_supervisor="${test_root}/fake-supervisor"
case_number=0

cleanup() {
  rm -rf "$test_root"
}
trap cleanup EXIT HUP INT TERM

cat > "$fake_supervisor" <<'EOF'
#!/bin/sh
printf '%s\n' "$@" > "${TEST_CAPTURE}/args"
env > "${TEST_CAPTURE}/env"
EOF
chmod 0755 "$fake_supervisor"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

new_case() {
  case_number=$((case_number + 1))
  case_dir="${test_root}/case-${case_number}"
  capture_dir="${case_dir}/capture"
  mkdir -p "$capture_dir"
}

run_entrypoint() {
  role="$1"
  shift
  env -i \
    PATH="$PATH" \
    TEST_CAPTURE="$capture_dir" \
    FRP_SUPERVISOR_BIN="$fake_supervisor" \
    FRP_CONFIG_ROOT="${case_dir}/config" \
    FRP_CONFIG_PATH="${case_dir}/config/${role}.toml" \
    FRP_DATA_DIR="${case_dir}/data" \
    FRP_LOG_DIR="${case_dir}/logs" \
    FRP_READY_FILE="${case_dir}/run/ready" \
    "$@" \
    /bin/sh "${script_dir}/entrypoint.sh" "$role"
}

new_case
printf 'file-password\n' > "${case_dir}/password"
printf 'file-token\n' > "${case_dir}/token"
run_entrypoint frps \
  FRP_ADMIN_PASSWORD=environment-password \
  FRP_ADMIN_PASSWORD_FILE="${case_dir}/password" \
  FRP_AUTH_TOKEN=environment-token \
  FRP_AUTH_TOKEN_FILE="${case_dir}/token" \
  FRP_LOG_LEVEL=debug \
  FRP_LOG_MAX_DAYS=14

grep -q '^auth.token = "file-token"$' "${case_dir}/config/frps.toml" ||
  fail "secret file should take precedence over direct token"
grep -q '^log.level = "debug"$' "${case_dir}/config/frps.toml" ||
  fail "log level should be written to generated config"
grep -q '^log.maxDays = 14$' "${case_dir}/config/frps.toml" ||
  fail "log retention should be written to generated config"
grep -q '^FRP_ADMIN_PASSWORD=file-password$' "${capture_dir}/env" ||
  fail "password file value should be exported"
grep -q '^FRP_AUTH_TOKEN=file-token$' "${capture_dir}/env" ||
  fail "token file value should be exported"

mkdir -p "${case_dir}/data/admin"
printf '{}\n' > "${case_dir}/data/admin/auth.json"
cp "${case_dir}/config/frps.toml" "${case_dir}/config.before"
run_entrypoint frps \
  FRP_AUTH_TOKEN=changed-token \
  FRP_LOG_LEVEL=error
cmp -s "${case_dir}/config.before" "${case_dir}/config/frps.toml" ||
  fail "existing config should not be replaced without FRP_REGENERATE_CONFIG=1"

run_entrypoint frps \
  FRP_REGENERATE_CONFIG=1 \
  FRP_AUTH_TOKEN=replacement-token \
  FRP_LOG_LEVEL=warn
grep -q '^auth.token = "replacement-token"$' "${case_dir}/config/frps.toml" ||
  fail "regenerated config should use the new token"
grep -q '^log.level = "warn"$' "${case_dir}/config/frps.toml" ||
  fail "regenerated config should use the new log level"
find "${case_dir}/config" -name 'frps.toml.env-backup-*' -print -quit |
  grep -q . || fail "regeneration should preserve a timestamped backup"

new_case
if run_entrypoint frpc \
  FRP_ADMIN_PASSWORD=client-password \
  FRP_AUTH_TOKEN=client-token \
  >"${case_dir}/output" 2>&1; then
  fail "frpc first start should require FRP_SERVER_ADDR"
fi
grep -q 'FRP_SERVER_ADDR' "${case_dir}/output" ||
  fail "missing FRP_SERVER_ADDR should produce a clear error"

new_case
if run_entrypoint frpc \
  FRP_ADMIN_PASSWORD=client-password \
  FRP_AUTH_TOKEN=client-token \
  FRP_SERVER_ADDR=frps.example.com \
  FRP_SERVER_PORT=invalid \
  >"${case_dir}/output" 2>&1; then
  fail "invalid FRP_SERVER_PORT should fail"
fi
grep -q 'FRP_SERVER_PORT' "${case_dir}/output" ||
  fail "invalid port should identify FRP_SERVER_PORT"

new_case
if run_entrypoint frps \
  FRP_ADMIN_PASSWORD=server-password \
  FRP_AUTH_TOKEN=server-token \
  FRP_REGENERATE_CONFIG=true \
  >"${case_dir}/output" 2>&1; then
  fail "invalid FRP_REGENERATE_CONFIG should fail"
fi
grep -q 'FRP_REGENERATE_CONFIG' "${case_dir}/output" ||
  fail "invalid reset flag should identify FRP_REGENERATE_CONFIG"

new_case
if run_entrypoint frps \
  FRP_ADMIN_PASSWORD=server-password \
  FRP_AUTH_TOKEN=server-token \
  FRP_LOG_LEVEL=verbose \
  >"${case_dir}/output" 2>&1; then
  fail "invalid FRP_LOG_LEVEL should fail"
fi
grep -q 'FRP_LOG_LEVEL' "${case_dir}/output" ||
  fail "invalid log level should identify FRP_LOG_LEVEL"

new_case
printf '\n' > "${case_dir}/empty-token"
if run_entrypoint frps \
  FRP_ADMIN_PASSWORD=server-password \
  FRP_AUTH_TOKEN_FILE="${case_dir}/empty-token" \
  >"${case_dir}/output" 2>&1; then
  fail "empty secret files should fail"
fi
grep -q '文件内容为空' "${case_dir}/output" ||
  fail "empty secret file failure should be explicit"

echo "entrypoint tests passed"
