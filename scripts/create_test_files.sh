#!/usr/bin/env bash

set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  create_test_files.sh '{"path/to/file.bin": 1024, "other/file.txt": 0}'
  echo '{"path/to/file.bin": 1024}' | create_test_files.sh

The input must be a JSON object where keys are file paths and values are file sizes in bytes.
EOF
}

if [[ $# -gt 1 ]]; then
	echo "Ошибка: ожидается один JSON-аргумент или ввод через stdin" >&2
	usage >&2
	exit 1
fi

if [[ $# -eq 1 ]]; then
	spec_input="$1"
else
	spec_input="$(cat)"
fi

if [[ -z "${spec_input//[[:space:]]/}" ]]; then
	echo "Ошибка: пустой ввод" >&2
	usage >&2
	exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
	echo "Ошибка: требуется python3 для разбора JSON" >&2
	exit 1
fi

while IFS=$'\t' read -r file_path file_size; do
	[[ -n "$file_path" ]] || continue

	if [[ "$file_path" == /* ]]; then
		target_path="$file_path"
	else
		target_path="$PWD/$file_path"
	fi

	if [[ -e "$target_path" && ! -f "$target_path" ]]; then
		echo "Ошибка: $target_path уже существует и не является файлом" >&2
		exit 1
	fi

	parent_dir="$(dirname "$target_path")"
	mkdir -p "$parent_dir"

	if [[ ! "$file_size" =~ ^-?[0-9]+$ ]]; then
		echo "Ошибка: некорректный размер для $file_path: $file_size" >&2
		exit 1
	fi

	if (( file_size < 0 )); then
		echo "Ошибка: размер файла не может быть отрицательным для $file_path" >&2
		exit 1
	fi

	if (( file_size == 0 )); then
		: > "$target_path"
	else
		head -c "$file_size" /dev/urandom > "$target_path"
	fi

	echo "Создан $target_path ($file_size bytes)"
done < <(
	python3 -c '
import json
import sys

payload = sys.stdin.read()
try:
	data = json.loads(payload)
except json.JSONDecodeError as exc:
	print(f"Ошибка: некорректный JSON: {exc}", file=sys.stderr)
	sys.exit(1)

if not isinstance(data, dict):
	print("Ошибка: вход должен быть JSON-объектом вида {\"path\": size, ...}", file=sys.stderr)
	sys.exit(1)

for path in sorted(data.keys()):
	size = data[path]
	if isinstance(size, bool):
		print(f"Ошибка: размер для {path} должен быть числом, а не bool", file=sys.stderr)
		sys.exit(1)
	try:
		size = int(size)
	except (TypeError, ValueError):
		print(f"Ошибка: размер для {path} должен быть целым числом", file=sys.stderr)
		sys.exit(1)
	print(f"{path}\t{size}")
' <<< "$spec_input"
)