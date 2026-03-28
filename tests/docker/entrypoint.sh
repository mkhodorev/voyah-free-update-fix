#!/usr/bin/env bash
set -euo pipefail

ssh-keygen -A
exec /usr/sbin/sshd -D -e
