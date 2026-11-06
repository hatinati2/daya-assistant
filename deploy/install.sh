#!/usr/bin/env bash
set -Eeuo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo 'این نصب‌کننده باید با root اجرا شود.' >&2
  exit 1
fi
command -v systemctl >/dev/null 2>&1 || {
  echo 'systemd روی این سرور در دسترس نیست.' >&2
  exit 1
}

MODE="${DAYA_INSTALL_MODE:-production}"
case "$MODE" in
  production)
    APP=/opt/daya-assistant
    DATA=/var/lib/daya-assistant
    CONF=/etc/daya-assistant.env
    KEY=/etc/daya-assistant.key
    SERVICE=daya-assistant
    SERVICE_USER=daya-assistant
    ADDR=0.0.0.0:8100
    PREVIEW=false
    CREDENTIALS_FILE=/root/daya-assistant-credentials.txt
    UNIT_SOURCE=daya-assistant.service
    ;;
  preview)
    APP=/opt/daya-assistant-preview
    DATA=/var/lib/daya-assistant-preview
    CONF=/etc/daya-assistant-preview.env
    KEY=/etc/daya-assistant-preview.key
    SERVICE=daya-assistant-preview
    SERVICE_USER=daya-assistant-preview
    ADDR=0.0.0.0:8101
    PREVIEW=true
    CREDENTIALS_FILE=/root/daya-assistant-preview-credentials.txt
    UNIT_SOURCE=daya-assistant-preview.service
    ;;
  *)
    echo "DAYA_INSTALL_MODE باید production یا preview باشد؛ مقدار فعلی: $MODE" >&2
    exit 1
    ;;
esac

if [[ ! -e /usr/share/zoneinfo/Asia/Tehran ]]; then
  apt-get update
  apt-get install -y tzdata
fi
if command -v timedatectl >/dev/null 2>&1; then
  timedatectl set-timezone Asia/Tehran
fi
echo "منطقه زمانی سیستم: $(date '+%Y-%m-%d %H:%M:%S %Z')"

if ! command -v go >/dev/null 2>&1; then
  apt-get update
  apt-get install -y golang-go
fi
GO_VERSION="$(go env GOVERSION | sed 's/^go//')"
MIN_VERSION=1.23
if [[ "$(printf '%s\n%s\n' "$MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$MIN_VERSION" ]]; then
  echo "Go 1.23 یا جدیدتر لازم است؛ نسخه فعلی: $GO_VERSION" >&2
  exit 1
fi

id "$SERVICE_USER" >/dev/null 2>&1 || useradd --system --home-dir "$APP" --shell /usr/sbin/nologin "$SERVICE_USER"
install -d -o root -g root -m 0755 "$APP"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0700 "$DATA"

# Generate an external AES-256-GCM key once; never replace an existing key.
if [[ ! -e "$KEY" ]]; then
  umask 077
  head -c 32 /dev/urandom > "$KEY"
elif [[ ! -f "$KEY" ]] || [[ "$(wc -c < "$KEY")" -ne 32 ]]; then
  echo "کلید رمزنگاری $KEY باید فایل معمولی دقیقاً ۳۲ بایتی باشد؛ کلید قبلی بازنویسی نمی‌شود." >&2
  exit 1
fi
chown root:"$SERVICE_USER" "$KEY"
chmod 0640 "$KEY"

if [[ ! -f "$CONF" ]]; then
  install -m 0600 /dev/null "$CONF"
fi
chmod 0600 "$CONF"

set_default() {
  local key="$1"
  local value="$2"
  if ! grep -qE "^${key}=.+" "$CONF"; then
    sed -i "/^${key}=/d" "$CONF"
    printf '%s=%s\n' "$key" "$value" >> "$CONF"
  fi
}

set_default DAYA_ADMIN_USER admin
if ! grep -qE '^DAYA_ADMIN_PASS=.+' "$CONF"; then
  PASSWORD="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
  set_default DAYA_ADMIN_PASS "$PASSWORD"
  ADMIN_USER="$(sed -n 's/^DAYA_ADMIN_USER=//p' "$CONF" | tail -n1)"
  printf 'Daya Assistant credentials\nUsername: %s\nPassword: %s\n' "$ADMIN_USER" "$PASSWORD" > "$CREDENTIALS_FILE"
  chmod 0600 "$CREDENTIALS_FILE"
  echo "اطلاعات ورود در $CREDENTIALS_FILE ذخیره شد."
fi
set_default DAYA_ADDR "$ADDR"
set_default DAYA_DATA_DIR "$DATA"
set_default DAYA_CREDENTIALS_KEY_FILE "$KEY"

# Preview and production use independent processes, ports, keys and data.
sed -i '/^DAYA_PREVIEW_MODE=/d' "$CONF"
printf 'DAYA_PREVIEW_MODE=%s\n' "$PREVIEW" >> "$CONF"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

gofmt -w cmd/server/*.go internal/jalaali/*.go
echo 'اجرای تست‌های خودکار...'
go test ./...
echo 'کامپایل برنامه...'
go build -trimpath -ldflags='-s -w' -o "$APP/daya-assistant" ./cmd/server
chmod 0755 "$APP/daya-assistant"
chown root:root "$APP/daya-assistant"

install -m 0644 "$SCRIPT_DIR/$UNIT_SOURCE" "/etc/systemd/system/$SERVICE.service"
systemctl daemon-reload
systemctl enable "$SERVICE.service"
systemctl restart "$SERVICE.service"
if ! systemctl is-active --quiet "$SERVICE.service"; then
  systemctl --no-pager --full status "$SERVICE.service" || true
  echo "راه‌اندازی $SERVICE ناموفق بود." >&2
  exit 1
fi

echo
echo "نصب/به‌روزرسانی $SERVICE کامل شد."
systemctl --no-pager --full status "$SERVICE.service" || true
echo
echo "اطلاعات ورود: cat $CREDENTIALS_FILE"
echo "تنظیمات: $CONF"
echo "داده‌ها: $DATA"
echo "کلید مستقل: $KEY"
echo "منطقه زمانی: Asia/Tehran"
echo "پنل: http://SERVER_IP:$([[ "$MODE" == "preview" ]] && echo 8101 || echo 8100)"
