#!/usr/bin/env bash
# Страж glibc (спека 101, решение 101-7): наибольшая версия символов GLIBC_x.y в бинарнике не выше порога.
#   scripts/linux/glibc.sh <бинарник> [порог]     порог по умолчанию — GLIBC_MAX или 2.35 (Ubuntu 22.04)
# Печатает найденный максимум; код 1 — выше порога или символов нет (статический бинарник нам не подходит),
# 2 — неверные аргументы. Сборка идёт на 24.04 (glibc 2.39), а ставиться должна и на 22.04.
set -euo pipefail

[ $# -ge 1 ] && [ $# -le 2 ] || { echo "использование: glibc.sh <бинарник> [порог]" >&2; exit 2; }
bin=$1
max=${2:-${GLIBC_MAX:-2.35}}
[ -f "$bin" ] || { echo "нет файла: $bin" >&2; exit 2; }

found=$(objdump -T "$bin" | grep -o 'GLIBC_[0-9][0-9.]*' | sed 's/^GLIBC_//' | sort -uV | tail -n 1 || true)
if [ -z "$found" ]; then
  echo "::error::В $(basename "$bin") нет символов GLIBC_ — не динамический бинарник glibc" >&2
  exit 1
fi
echo "наибольшая GLIBC_$found (порог $max)"
# sort -V: если порог не больше найденного и они разные — найденный выше
if [ "$found" != "$max" ] && [ "$(printf '%s\n%s\n' "$found" "$max" | sort -V | tail -n 1)" = "$found" ]; then
  echo "::error::$(basename "$bin") требует GLIBC_$found, допустимо не выше $max (Ubuntu 22.04)" >&2
  exit 1
fi
