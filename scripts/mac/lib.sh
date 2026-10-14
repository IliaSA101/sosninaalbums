# shellcheck shell=bash
# Общие функции scripts/mac (029 design §3): таблица summary.md, журнал, тайм-аут, снимок,
# ожидание окна. Подключается через «.», сам ничего не запускает.
# Пишется под bash 3.2 (системный на macOS): без ассоциативных массивов, mapfile, ${x,,},
# пустые массивы под set -u не раскрываются; утилиты BSD (stat -f, без grep -P и timeout).

MAC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PY="${PY:-python3}"
TAB="$(printf '\t')"

# install_allowed — можно ли ставить чужой код (brew, pip). В выпуске (release.yml) нельзя:
# shots.sh --no-install или MAC_NO_INSTALL=1; нужного инструмента нет — шаг пропускается с пометкой
install_allowed() { [ "${MAC_NO_INSTALL:-0}" != 1 ]; }

# ---------- чистые функции (проверяет test_lib.sh) ----------

# ver_le A B — версия A ≤ B; части через точку, недостающие = 0. Пустую строку отсекает вызывающий.
ver_le() {
  awk -v a="$1" -v b="$2" 'BEGIN {
    na = split(a, x, "."); nb = split(b, y, "."); n = (na > nb) ? na : nb
    for (i = 1; i <= n; i++) { p = x[i] + 0; q = y[i] + 0; if (p < q) exit 0; if (p > q) exit 1 }
    exit 0 }'
}

# is_universal <вывод lipo -archs> — есть и x86_64, и arm64, в любом порядке (прогон 1: сравнение
# шаблоном «*" x86_64 "*" arm64 "*» не находило общий пробел между словами)
is_universal() {
  local w x=0 a=0
  # shellcheck disable=SC2086 # список архитектур — отдельными словами
  for w in $1; do
    case "$w" in
      x86_64) x=1 ;;
      arm64) a=1 ;;
    esac
  done
  [ "$x" = 1 ] && [ "$a" = 1 ]
}

# parse_minos — minos из вывода `vtool -show-build` / `otool -l` (stdin):
# LC_BUILD_VERSION → «minos X», старые бинари — LC_VERSION_MIN_MACOSX → «version X»
parse_minos() {
  awk '
    $1 == "minos" { print $2; exit }
    $1 == "cmd" { vm = ($2 == "LC_VERSION_MIN_MACOSX"); next }
    vm && $1 == "version" { print $2; exit }
  '
}

# version_from_zip <путь> — X.Y.Z из SosninaAlbums-X.Y.Z-macos-universal.zip, иначе пусто
version_from_zip() {
  basename "$1" | sed -n 's/^SosninaAlbums-\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)-macos.*\.zip$/\1/p'
}

# md_cell <текст> — одна ячейка таблицы Markdown: без «|» и переводов строк
md_cell() {
  printf '%s' "$1" | tr '\n|' ' /'
}

# find_app <папка> — путь к единственному .app: в самой папке (выпуск 0.10) или на уровень глубже
# (выпуск 0.11: «Sosnina Albums X.Y.Z/sosninaalbums.app»). Ноль или несколько — код 1.
# __MACOSX (остатки AppleDouble) не считается
find_app() {
  local a n=0 found=""
  for a in "$1"/*.app "$1"/*/*.app; do
    [ -d "$a" ] || continue
    case "$a" in */__MACOSX/*) continue ;; esac
    n=$((n + 1))
    found="$a"
  done
  [ "$n" -eq 1 ] || return 1
  printf '%s' "$found"
}

# quarantine_value <unix-время> <uuid> — метка «скачано Safari», как у браузера
quarantine_value() {
  printf '0083;%x;Safari;%s' "$1" "$2"
}

# tcc_values <столбцов> <служба> <клиент> <тип клиента: 0 bundle id, 1 путь> <unix-время>
# Строка VALUES для таблицы access в TCC.db: 13 столбцов до Sonoma, 17 — Sonoma и новее
# (runner-images configure-tccdb-macos.sh, issue #9529). Иная схема — код 1.
tcc_values() {
  local base="'$2','$3',$4,2,4,1,NULL,NULL,0,'UNUSED',NULL,0,$5"
  case "$1" in
    13) printf '%s' "$base" ;;
    17) printf '%s' "$base,NULL,NULL,'UNUSED',$5" ;;
    *) return 1 ;;
  esac
}

# avf_screen_index — номер устройства «Capture screen 0» из `ffmpeg -f avfoundation -list_devices true` (stdin)
avf_screen_index() {
  awk 'match($0, /\[[0-9]+\] Capture screen 0/) {
    s = substr($0, RSTART + 1); sub(/\].*/, "", s); print s; exit }'
}

# fps_of <кадров> <секунд> — частота с двумя знаками; 0 секунд — 0
fps_of() {
  awk -v n="$1" -v s="$2" 'BEGIN { if (s + 0 <= 0) print "0"; else printf "%.2f\n", n / s }'
}

# video_verdict <секунд> <кадр/с> <доля движения или пусто> <МБ> [<число изменившихся пар кадров> [<мин. секунд, 20>]] — «да|нет<TAB>почему»
# по критерию спеки: ≥ 20 с, ≥ 5 кадр/с, экран не застыл (> 10 % соседних кадров разные); > 20 МБ — только
# пометка. Прогон 3: запись на 242 с шла по живому экрану (кадр из середины — «Настройки» с курсором), но шаги
# в основном ждут: доля 0,099 при десятках смен кадра. Поэтому от «застыл» спасают и ≥ 8 изменившихся пар
video_verdict() {
  awk -v d="$1" -v f="$2" -v m="$3" -v mb="$4" -v c="${5:-}" -v mn="${6:-20}" 'BEGIN {
    ok = 1; why = ""
    if (d + 0 < mn + 0) { ok = 0; why = why "короче " mn " с; " }
    if (f + 0 < 5) { ok = 0; why = why "меньше 5 кадр/с; " }
    if (m == "") why = why "движение не измерено; "
    else if (m + 0 <= 0.10 && (c == "" || c + 0 < 8)) { ok = 0; why = why "экран застыл; " }
    if (mb + 0 > 20) why = why "больше 20 МБ — обрезать; "
    sub(/; $/, "", why)
    printf "%s\t%s", (ok ? "да" : "нет"), why }'
}

# ---------- вывод: summary.md, журнал ----------

# out_init <папка> — раскладка артефакта (design §3.2)
out_init() {
  OUT="$1"
  mkdir -p "$OUT/logs" "$OUT/png" "$OUT/video"
}

# note <текст> — строка в logs/steps.txt и в журнал шага
note() {
  printf '%s %s\n' "$(date +%H:%M:%S)" "$*" | tee -a "$OUT/logs/steps.txt" >&2
}

# section <заголовок> — новая таблица «вариант | итог | почему» в summary.md
section() {
  printf '\n## %s\n\n| вариант | итог | почему |\n| --- | --- | --- |\n' "$1" >>"$OUT/summary.md"
}

# row <вариант> <да|нет|пропуск|предупреждение> <почему>
row() {
  printf '| %s | %s | %s |\n' "$(md_cell "$1")" "$(md_cell "$2")" "$(md_cell "$3")" >>"$OUT/summary.md"
  note "$1: $2${3:+ — $3}"
}

# summary_head — версия macOS, архитектура, язык, экран: один раз на артефакт
summary_head() {
  [ -s "$OUT/summary.md" ] && return 0
  {
    printf '# Mac: %s %s (%s), раннер %s\n\n' "$(sw_vers -productName 2>/dev/null)" \
      "$(sw_vers -productVersion 2>/dev/null)" "$(uname -m)" "${RUNNER_NAME:-локально}"
    printf -- '- язык: %s\n' "$(defaults read -g AppleLanguages 2>/dev/null | tr -d ' \n')"
    printf -- '- экран: %s\n' "$(system_profiler SPDisplaysDataType 2>/dev/null | awk -F': ' '/Resolution/ { print $2; exit }')"
  } >"$OUT/summary.md"
}

# ---------- процессы ----------

# with_timeout <сек> <команда…> — своя замена timeout (в macOS её нет); по истечении TERM, через 2 с KILL
with_timeout() {
  local secs="$1" pid w rc
  shift
  "$@" &
  pid=$!
  (
    sleep "$secs"
    # дети шага (osascript, open) иначе живут дальше и мешают следующим шагам
    pkill -TERM -P "$pid" 2>/dev/null
    kill -TERM "$pid" 2>/dev/null && note "тайм-аут ${secs} с: $1"
    sleep 2
    kill -KILL "$pid" 2>/dev/null
  ) </dev/null >/dev/null 2>&1 &
  w=$!
  wait "$pid"
  rc=$?
  pkill -P "$w" 2>/dev/null
  kill "$w" 2>/dev/null
  wait "$w" 2>/dev/null
  return "$rc"
}

# alive <pid> — процесс есть и не зомби
alive() {
  local s
  s="$(ps -o stat= -p "$1" 2>/dev/null)" || return 1
  case "$s" in Z*) return 1 ;; esac
  return 0
}

# app_pids <app> — pid процессов этого .app (через open их родитель не мы)
app_pids() {
  pgrep -f "$1/Contents/MacOS/" 2>/dev/null | tr '\n' ' '
}

# wait_app <app> <сек> — дождаться процесса .app; печатает pid
wait_app() {
  local i=0 p
  while [ "$i" -lt "$2" ]; do
    p="$(app_pids "$1")"
    if [ -n "${p// /}" ]; then
      printf '%s' "${p%% *}"
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

# plist_get <app> <ключ>
plist_get() {
  /usr/libexec/PlistBuddy -c "Print :$2" "$1/Contents/Info.plist" 2>/dev/null
}

# stop_app <app> — вежливо (quit), затем pkill; ждёт исчезновения: SingleInstance иначе
# передаст следующий запуск старому процессу
stop_app() {
  local id i
  id="$(plist_get "$1" CFBundleIdentifier)"
  [ -n "$(app_pids "$1")" ] || return 0
  [ -n "$id" ] && osascript -e "quit app id \"$id\"" >/dev/null 2>&1
  for i in 1 2 3 4 5; do
    [ -z "$(app_pids "$1")" ] && return 0
    sleep 1
  done
  pkill -f "$1/Contents/MacOS/" 2>/dev/null
  sleep 1
  pkill -KILL -f "$1/Contents/MacOS/" 2>/dev/null
  return 0
}

# quarantine <путь> — метка карантина на каждый файл, как после распаковки скачанного zip
quarantine() {
  local q
  q="$(quarantine_value "$(date +%s)" "$(uuidgen)")"
  find "$1" -print0 | xargs -0 xattr -w com.apple.quarantine "$q" 2>/dev/null
  xattr -p com.apple.quarantine "$1" >/dev/null 2>&1
}

# Окна Gatekeeper рисуют агенты с именем длиннее 16 знаков: killall и pgrep -x по усечённому имени
# их не видят (прогон 1: CoreServicesUIAgent с pid 317 пережил все killall) — ищем по пути
GK_AGENTS_RE='/(CoreServicesUIAgent|UserNotificationCenter)[.]app/Contents/MacOS/'

# gk_pids — pid агентов окон Gatekeeper
gk_pids() {
  pgrep -f "$GK_AGENTS_RE" 2>/dev/null | tr '\n' ' '
}

# kill_gk_agents — закрыть окна Gatekeeper; launchd поднимет агентов заново при следующем запросе
kill_gk_agents() {
  pkill -f "$GK_AGENTS_RE" 2>/dev/null
  sudo -n pkill -f "$GK_AGENTS_RE" 2>/dev/null
  return 0
}

# open_bg <app> — open с меткой карантина ждёт ответа окна Gatekeeper (прогон 1: шаг висел до
# тайм-аута) — в фоне; печатает pid процесса open
open_bg() {
  open "$1" </dev/null >/dev/null 2>>"$OUT/logs/open.txt" &
  printf '%s' "$!"
}

# ---------- окна и снимки ----------

# windows_bin — собрать windows.swift один раз (swift-скрипт компилируется при каждом запуске)
windows_bin() {
  local b="$WORK/windows"
  [ -x "$b" ] && { printf '%s' "$b"; return 0; }
  if swiftc -O "$MAC_DIR/windows.swift" -o "$b" >"$OUT/logs/swiftc.txt" 2>&1; then
    printf '%s' "$b"
    return 0
  fi
  return 1
}

# windows_json <файл> — список окон в JSON
windows_json() {
  local b
  if b="$(windows_bin)"; then
    "$b" >"$1" 2>>"$OUT/logs/windows-err.txt" && return 0
  fi
  swift "$MAC_DIR/windows.swift" >"$1" 2>>"$OUT/logs/windows-err.txt"
}

# ocr_run <аргументы ocr.swift> — собранный один раз ocr, иначе swift-скриптом
ocr_run() {
  local b="$WORK/ocr"
  if [ ! -x "$b" ]; then
    swiftc -O "$MAC_DIR/ocr.swift" -o "$b" >"$OUT/logs/swiftc-ocr.txt" 2>&1 || b=""
  fi
  if [ -n "$b" ]; then "$b" "$@" 2>>"$OUT/logs/ocr-err.txt"; else swift "$MAC_DIR/ocr.swift" "$@" 2>>"$OUT/logs/ocr-err.txt"; fi
}

# ocr_hits <надпись>… — текст на снимке экрана; все строки «x y w h<TAB>строка» (точки), короткие первыми
ocr_hits() {
  local png="$WORK/ocr.png"
  screencapture -x "$png" 2>>"$OUT/logs/screencapture.txt" || return 1
  ocr_run find "$png" "$@" | tee -a "$OUT/logs/ocr-hits.txt"
}

# ocr_find <надпись>… — самая короткая строка из ocr_hits или пусто
ocr_find() {
  ocr_hits "$@" | head -n 1
}

# ocr_in <«x y w h» окна> [--leftmost] <надпись>… — то же, но только строки внутри окна: подпись кнопки
# встречается и в окне, и под ним (прогон 3: «Все равно открыть» в «Настройках» за окном подтверждения)
ocr_in() {
  local rect="$1" lm=""
  shift
  if [ "${1:-}" = "--leftmost" ]; then
    lm="--leftmost"
    shift
  fi
  ocr_hits "$@" | "$PY" "$MAC_DIR/macshots.py" inrect --rect "$rect" ${lm:+"$lm"}
}

# ocr_click «x y w h» — щелчок мыши (CGEvent) в центр рамки
ocr_click() {
  # shellcheck disable=SC2086 # «x y w h» → четыре числа
  set -- $1
  [ "$#" -eq 4 ] || return 1
  ocr_run click "$(($1 + $3 / 2))" "$(($2 + $4 / 2))"
}

# find_window <pid-ы через пробел> <рег. выражение владельца> <front|largest> — «id x y w h» в точках
find_window() {
  local f="$WORK/win-probe.json"
  windows_json "$f" || return 1
  "$PY" "$MAC_DIR/macshots.py" win --windows "$f" --pid "$1" --owner "$2" --pick "$3"
}

# wait_window <pid-ы> <владелец> <сек> — окно появилось
wait_window() {
  local i=0
  while [ "$i" -lt "$3" ]; do
    find_window "$1" "$2" largest >/dev/null 2>&1 && return 0
    sleep 1
    i=$((i + 1))
  done
  return 1
}

# shot <имя> <pid-ы> <владелец> <front|largest> [<надпись> <x y w h>] [-- <почему>]
# Полный экран → png/<имя>.png, окно → png/<имя>-win.png, запись в shots.json, строка в summary.md.
# Код 0 — снимок годен (не одни обои, нужное окно в списке).
shot() {
  local name="$1" pids="$2" owner="$3" pick="$4" label="" rect="" why="" wid line res
  shift 4
  if [ "$#" -ge 2 ] && [ "$1" != "--" ]; then
    label="$1"
    rect="$2"
    shift 2
  fi
  [ "${1:-}" = "--" ] && why="${2:-}"
  local full="$OUT/png/$name.png" win="$OUT/png/$name-win.png" wj="$OUT/logs/windows-$name.json"
  screencapture -x "$full" 2>>"$OUT/logs/screencapture.txt"
  windows_json "$wj"
  if [ -n "$pids$owner" ]; then
    line="$("$PY" "$MAC_DIR/macshots.py" win --windows "$wj" --pid "$pids" --owner "$owner" --pick "$pick" 2>/dev/null)"
    wid="${line%% *}"
    [ -n "$wid" ] && screencapture -x -o -l"$wid" "$win" 2>>"$OUT/logs/screencapture.txt"
  fi
  # Надпись без рамки — кнопку не нашли: вариант не удался, даже если снимок годен
  local need=""
  [ -n "$label" ] && [ -z "$rect" ] && need="--need-rect"
  res="$("$PY" "$MAC_DIR/macshots.py" record --out "$OUT" --name "$name" --windows "$wj" \
    --pid "$pids" --owner "$owner" --pick "$pick" --label "$label" --rect "$rect" \
    --macos "$MACOS" --lang "$LANG_UI" --why "$why" ${need:+"$need"} 2>>"$OUT/logs/macshots-err.txt")"
  [ -n "$res" ] || res="нет${TAB}macshots.py не ответил (logs/macshots-err.txt)"
  row "$name" "${res%%"$TAB"*}" "${res#*"$TAB"}"
  [ "${res%%"$TAB"*}" = "да" ]
}

# key_code <код> — нажатие через System Events (53 Escape, 119 End, 121 Page Down)
key_code() {
  osascript -e "tell application \"System Events\" to key code $1" >/dev/null 2>&1
}

# axfind <bundle id> <find|click|selected> <надпись>… — «x y w h<TAB>надпись» (точки) или пусто
axfind() {
  osascript "$MAC_DIR/axfind.applescript" "$@" 2>>"$OUT/logs/axfind-err.txt"
}
