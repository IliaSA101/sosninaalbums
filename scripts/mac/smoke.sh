#!/bin/bash
# shellcheck source-path=SCRIPTDIR
# Блокирующая проверка Mac-сборки (029 design §3.1): smoke.sh <zip> <папка вывода>
# Код ≠ 0 при первом провале блокирующего пункта; строки — в <папка>/summary.md, подробности — logs/smoke-*.txt.
# Окружение: MAC_MIN_OS (12.0), MAC_ALIVE_SEC (10), MAC_SMOKE_SELFTEST=crash — исполняемый файл
# заменяется на «exit 1», чтобы убедиться, что п. 4 действительно валит задание.
set -uo pipefail

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

[ "$#" -eq 2 ] || { echo "использование: smoke.sh <zip> <папка вывода>" >&2; exit 2; }
ZIP="$1"
out_init "$2"
MIN_OS="${MAC_MIN_OS:-12.0}"
ALIVE="${MAC_ALIVE_SEC:-10}"
WORK="${RUNNER_TEMP:-$(mktemp -d)}/smoke"
LOGFILE="$HOME/Library/Application Support/SosninaAlbums/log.txt"
VERSION="$(version_from_zip "$ZIP")"
LAST_ERR=""

summary_head
section "Проверка запуска (smoke.sh)"

# diag <файл stderr> — хвост stderr, системный журнал и отчёты о падении: в журнал, сводку и ::group::
diag() {
  local d="$OUT/logs/smoke-diag.txt" f
  {
    echo "== stderr (50 строк): $1"
    [ -f "$1" ] && tail -50 "$1"
    echo "== log show"
    log show --last 2m --style compact --predicate 'process CONTAINS "sosninaalbums"' 2>&1 | tail -200
    echo "== DiagnosticReports"
    for f in "$HOME"/Library/Logs/DiagnosticReports/*osnina* /Library/Logs/DiagnosticReports/*osnina*; do
      [ -f "$f" ] || continue
      echo "-- $f"
      head -200 "$f"
    done
  } >"$d" 2>&1
  if [ -f "$1" ]; then
    {
      printf '\n<details><summary>stderr, последние 50 строк</summary>\n\n```text\n'
      tail -50 "$1"
      printf '```\n\n</details>\n'
    } >>"$OUT/summary.md"
  fi
  echo "::group::Диагностика запуска"
  cat "$d"
  echo "::endgroup::"
}

# fail <пункт> <почему> [<stderr>] — строка «нет», диагностика, выход
fail() {
  row "$1" "нет" "$2"
  echo "::error::Проверка Mac, $1: $2"
  [ -n "${3:-}" ] && diag "$3"
  exit 1
}

pass() { row "$1" "да" "${2:-}"; }

# run_alive <метка> <команда…> — процесс прожил ALIVE секунд; потом TERM, через 5 с KILL
run_alive() {
  local tag="$1" pid i=0 rc
  shift
  LAST_ERR="$OUT/logs/smoke-$tag.err"
  "$@" >"$OUT/logs/smoke-$tag.out" 2>"$LAST_ERR" &
  pid=$!
  while [ "$i" -lt "$ALIVE" ]; do
    sleep 1
    i=$((i + 1))
    if ! alive "$pid"; then
      wait "$pid"
      rc=$?
      note "$tag: вышел через $i с, код $rc"
      return 1
    fi
  done
  kill -TERM "$pid" 2>/dev/null
  for i in 1 2 3 4 5; do
    alive "$pid" || break
    sleep 1
  done
  if alive "$pid"; then
    kill -KILL "$pid" 2>/dev/null
    echo "::warning::$tag: не завершился по TERM за 5 с — KILL"
  fi
  wait "$pid" 2>/dev/null
  return 0
}

# alive_app <метка> <app> — через LaunchServices: процесс появился за ALIVE с и прожил ещё ALIVE с
alive_app() {
  local pid i=0
  LAST_ERR="$OUT/logs/smoke-$1.err"
  pid="$(wait_app "$2" "$ALIVE")" || { note "$1: процесс не появился"; return 1; }
  while [ "$i" -lt "$ALIVE" ]; do
    sleep 1
    i=$((i + 1))
    alive "$pid" || { note "$1: процесс $pid вышел через $i с"; return 1; }
  done
  return 0
}

log_lines() { if [ -f "$LOGFILE" ]; then wc -l <"$LOGFILE" | tr -d ' '; else echo 0; fi; }

# ---------- 1. Распаковка и Info.plist ----------
rm -rf "$WORK"
mkdir -p "$WORK/app"
ditto -x -k "$ZIP" "$WORK/app" 2>"$OUT/logs/smoke-unzip.txt" || fail "1 распаковка" "ditto не распаковал $(basename "$ZIP")"
# .app в корне (0.10) или в папке «Sosnina Albums X.Y.Z» (0.11)
APP="$(find_app "$WORK/app")" || fail "1 распаковка" "в zip должно быть ровно одно .app (в корне или в папке выпуска)"
PLIST="$APP/Contents/Info.plist"
plutil -lint "$PLIST" >"$OUT/logs/smoke-plist.txt" 2>&1 || fail "1 распаковка" "Info.plist не читается"
EXE="$(plist_get "$APP" CFBundleExecutable)"
BIN="$APP/Contents/MacOS/$EXE"
[ -n "$EXE" ] && [ -f "$BIN" ] || fail "1 распаковка" "нет исполняемого файла '$EXE'"
MODE="$(stat -f %Lp "$BIN")"
[ "$MODE" = "755" ] || fail "1 распаковка" "права $EXE — $MODE, нужно 755"
echo "$APP" >"$OUT/logs/app-path.txt"
pass "1 распаковка" "$(basename "$APP"), $EXE, id $(plist_get "$APP" CFBundleIdentifier), версия ${VERSION:-не из имени zip}"

# ---------- 2. Архитектуры и minos ----------
ARCHS="$(lipo -archs "$BIN" 2>&1)"
is_universal "$ARCHS" || fail "2 архитектуры" "не universal: $ARCHS"
mins=""
for a in x86_64 arm64; do
  m="$( (vtool -arch "$a" -show-build "$BIN" 2>/dev/null || otool -arch "$a" -l "$BIN") | parse_minos)"
  [ -n "$m" ] || fail "2 архитектуры" "$a: minos не найден (vtool/otool)"
  ver_le "$m" "$MIN_OS" || fail "2 архитектуры" "$a: minos $m > $MIN_OS"
  mins="$mins $a $m,"
done
LSMIN="$(plist_get "$APP" LSMinimumSystemVersion)"
if [ -n "$LSMIN" ]; then
  ver_le "$LSMIN" "$MIN_OS" || fail "2 архитектуры" "LSMinimumSystemVersion $LSMIN > $MIN_OS"
else
  echo "::warning::В Info.plist нет LSMinimumSystemVersion"
fi
pass "2 архитектуры" "$ARCHS; minos${mins} plist ${LSMIN:-нет} (≤ $MIN_OS)"

# ---------- 3. Подпись (ad-hoc), оценка Gatekeeper — в журнал ----------
{
  codesign -dv --verbose=2 "$APP" 2>&1
  echo "== spctl -a -vv"
  spctl -a -vv "$APP" 2>&1
  echo "== spctl --status"
  spctl --status 2>&1
} >"$OUT/logs/smoke-codesign.txt"
codesign --verify --deep --strict "$APP" >>"$OUT/logs/smoke-codesign.txt" 2>&1 ||
  fail "3 подпись" "codesign --verify --deep --strict не прошёл (logs/smoke-codesign.txt)"
SPCTL="$(spctl -a -vv "$APP" 2>&1 | head -1)"
pass "3 подпись" "codesign --verify да; spctl: ${SPCTL#"$APP": }"

# Самопроверка: испорченный бинарь должен уронить п. 4 (подпись уже проверена — иначе упал бы п. 3)
if [ "${MAC_SMOKE_SELFTEST:-}" = "crash" ]; then
  printf '#!/bin/sh\necho "selftest: crash" >&2\nexit 1\n' >"$BIN"
  chmod 755 "$BIN"
  row "самопроверка" "предупреждение" "MAC_SMOKE_SELFTEST=crash: $EXE заменён на exit 1"
fi

# ---------- 4–5. Прямой запуск родной архитектуры и строка в log.txt ----------
HOST="$(uname -m)"
before="$(log_lines)"
run_alive "direct-$HOST" "$BIN" || fail "4 запуск $HOST" "процесс не прожил $ALIVE с" "$LAST_ERR"
stop_app "$APP"
pass "4 запуск $HOST" "жил $ALIVE с"

want="запуск, версия ${VERSION}"
if [ -f "$LOGFILE" ] && tail -n "+$((before + 1))" "$LOGFILE" | grep -aqF "$want"; then
  pass "5 log.txt" "есть «${want}»"
else
  cp "$LOGFILE" "$OUT/logs/smoke-app-log.txt" 2>/dev/null
  fail "5 log.txt" "нет новой строки «${want}» в $LOGFILE" "$LAST_ERR"
fi

# ---------- 6. Через LaunchServices ----------
open -n "$APP" 2>"$OUT/logs/smoke-open.err"
alive_app "open" "$APP" || fail "6 open" "через open процесс не появился или не прожил $ALIVE с" "$OUT/logs/smoke-open.err"
stop_app "$APP"
pass "6 open" "жил $ALIVE с"

# ---------- 7. Метка карантина: Gatekeeper (журнал) и команда из инструкции (блокирует) ----------
DST="$HOME/Downloads/$(basename "$APP")"
rm -rf "$DST"
ditto "$APP" "$DST"
quarantine "$DST" || echo "::warning::Метка карантина не записалась"
xattr -l "$DST" >"$OUT/logs/smoke-xattr.txt" 2>&1
# open ждёт ответа окна Gatekeeper — без фона проверка висела бы до тайм-аута задания
open "$DST" </dev/null >/dev/null 2>"$OUT/logs/smoke-gk.err" &
gk_open=$!
sleep 5
# По имени процесса, не по пути: с меткой карантина macOS запускает копию из AppTranslocation
if pgrep -x "$EXE" >/dev/null; then
  row "7 Gatekeeper" "предупреждение" "с меткой карантина запустилось — Gatekeeper на раннере выключен ($(spctl --status 2>&1))"
else
  row "7 Gatekeeper" "да" "заблокировал запуск с меткой карантина ($(spctl --status 2>&1))"
fi
pkill -x "$EXE" 2>/dev/null
sleep 1
pkill -KILL -x "$EXE" 2>/dev/null
kill_gk_agents
kill "$gk_open" 2>/dev/null
wait "$gk_open" 2>/dev/null
xattr -dr com.apple.quarantine "$DST"
open "$DST" 2>"$OUT/logs/smoke-xattr.err"
alive_app "xattr" "$DST" || fail "7 после xattr -dr" "не открылось после снятия метки" "$OUT/logs/smoke-xattr.err"
stop_app "$DST"
pass "7 после xattr -dr" "xattr -dr com.apple.quarantine → open: жил $ALIVE с"
rm -rf "$DST"

# ---------- 8. Вторая архитектура ----------
if [ "$HOST" = "arm64" ]; then
  if arch -x86_64 /usr/bin/true 2>/dev/null; then
    run_alive "rosetta" arch -x86_64 "$BIN" || fail "8 x86_64 (Rosetta)" "процесс не прожил $ALIVE с" "$LAST_ERR"
    stop_app "$APP"
    pass "8 x86_64 (Rosetta)" "жил $ALIVE с"
  else
    echo "::warning::Rosetta нет — x86_64 не проверен (запасной путь: runners=macos-26-intel)"
    row "8 x86_64 (Rosetta)" "пропуск" "Rosetta нет"
  fi
else
  row "8 arm64" "пропуск" "раннер $HOST: arm64 здесь не запустить"
fi

echo "Проверка Mac пройдена: $(basename "$APP") ${VERSION}"
exit 0
