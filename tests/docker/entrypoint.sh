#!/usr/bin/env bash
set -euo pipefail

ssh-keygen -A

if [[ "${DISABLE_SFTP:-0}" == "1" ]]; then
  sed -i '/^[[:space:]]*Subsystem[[:space:]]\+sftp[[:space:]]/d' /etc/ssh/sshd_config
fi

exec /usr/sbin/sshd -D -e
