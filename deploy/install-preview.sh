#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DAYA_INSTALL_MODE=preview bash "$SCRIPT_DIR/install.sh"
