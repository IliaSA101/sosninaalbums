#!/usr/bin/env bash
# Стенд QA — своя копия wails dev рядом с основной (34115) — двойник scripts/stand.ps1 для Linux и macOS:
#   scripts/stand.sh <имя> [порт]        (по умолчанию порт 34116; принимаются и -Name b -Port 34116, как у .ps1)
# ПРАВЯТСЯ ВМЕСТЕ со stand.ps1: те же проверки имени и порта, SA_STAND, лог .gotmp/wails-dev-<имя>.log.
# Запускать из корня рабочей копии (ветка до слияния); одна рабочая копия — один стенд. SA_STAND даёт программе свой
# замок второго экземпляра и свои папки настроек, кэша миниатюр и копий (internal/osx DataName). Vite берёт свободный
# порт сам, wails dev находит его по выводу. Работает до остановки wails dev.
# Отличия Linux: тег webkit2_41 (WebKitGTK 4.1); без экрана (нет DISPLAY и WAYLAND_DISPLAY) окно — в xvfb-run.
# QA — в Chromium по http://localhost:<порт>: окно WebKitGTK не то, что видит пользователь (WebView2).
set -eu

name='' port=34116
while [ $# -gt 0 ]; do
  case "$1" in
    -Name | --name) name=$2; shift 2 ;;
    -Port | --port) port=$2; shift 2 ;;
    *) if [ -z "$name" ]; then name=$1; else port=$1; fi; shift ;;
  esac
done

root=$PWD
# Имя входит в имена папок данных: то же правило, что osx.ParseStand, — отказ раньше сборки.
printf '%s' "$name" | grep -Eq '^[A-Za-z0-9]{1,16}$' || { echo "Имя стенда «$name»: только латиница и цифры, до 16 знаков" >&2; exit 1; }
name=$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')
printf '%s' "$port" | grep -Eq '^[0-9]+$' && [ "$port" -ge 1024 ] && [ "$port" -le 65535 ] || { echo "Порт $port вне 1024–65535" >&2; exit 1; }
[ "$port" != 34115 ] || { echo 'Порт 34115 — у основного стенда, возьмите 34116 и выше' >&2; exit 1; }
[ -f "$root/wails.json" ] || { echo "Нет wails.json в $root — запускать из корня рабочей копии" >&2; exit 1; }
[ -e "$root/frontend/node_modules" ] || { echo 'Нет frontend/node_modules: в рабочей копии агента — ссылка на основное дерево (CLAUDE.md, Worktree)' >&2; exit 1; }

. "$root/scripts/env.sh"
command -v wails >/dev/null || { echo 'wails не найден (папка инструментов проекта, scripts/env.sh)' >&2; exit 1; }

# Занятый порт: wails dev упал бы позже и с невнятной ошибкой.
node -e "const s=require('net').createServer();s.once('error',()=>process.exit(1));s.listen($port,'127.0.0.1',()=>s.close())" ||
  { echo "Порт $port занят — другой стенд или программа" >&2; exit 1; }

export SA_STAND=$name
log="$GOTMPDIR/wails-dev-$name.log"
echo "стенд $name, http://localhost:$port, $root" > "$log"
echo "Стенд $name: http://localhost:$port, лог $log"

run=()
if [ "$(uname -s)" = Linux ] && [ -z "${DISPLAY:-}" ] && [ -z "${WAYLAND_DISPLAY:-}" ]; then
  command -v xvfb-run >/dev/null || { echo 'Нет экрана и нет xvfb-run (sudo apt install xvfb)' >&2; exit 1; }
  run=(xvfb-run -a)
fi
exec "${run[@]}" wails dev -devserver "localhost:$port" ${wails_tags:+-tags "$wails_tags"} >> "$log" 2>&1
