#!/usr/bin/env bash
set -Eeuo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "این نصب‌کننده باید با root اجرا شود." >&2
  exit 1
fi
for command_name in curl tar; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    apt-get update
    apt-get install -y curl tar
  fi
done

ref="${DAYA_INSTALL_REF:-feat/preview03-isolated}"
if [[ ! "$ref" =~ ^[A-Za-z0-9._/-]+$ || "$ref" == *".."* ]]; then
  echo "مقدار DAYA_INSTALL_REF معتبر نیست." >&2
  exit 1
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
echo "دریافت نسخه Preview 03 (${ref}) از GitHub..."
curl -fsSL "https://api.github.com/repos/hatinati2/daya-assistant/tarball/${ref}" -o "$tmp/source.tar.gz"
mkdir "$tmp/source"
tar -xzf "$tmp/source.tar.gz" --strip-components=1 -C "$tmp/source"
echo "شروع نصب Daya Assistant Preview 03 روی پورت 8103..."
DAYA_INSTALL_MODE=preview03 bash "$tmp/source/deploy/install.sh"
