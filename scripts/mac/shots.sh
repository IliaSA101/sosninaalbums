#!/bin/bash
# shellcheck source-path=SCRIPTDIR
# Снимки и видео для инструкции (029 design §3.2–3.6):
#   shots.sh <app или zip> <папка вывода> [--lang ru|en] [--video]
# Всегда код 0: каждый вариант пишет строку в summary.md и запись в shots.json, неудача шага
# не мешает следующим. zip распаковывается заново — selftest в smoke.sh портит только свою копию.
# Шаги зовутся по имени через run_step (SC2329); журналы — файлы пользователя, sudo нужен только команде (SC2024)
# shellcheck disable=SC2329,SC2024
set -u

# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"
# shellcheck source=video.sh
. "$MAC_DIR/video.sh"

SRC="${1:-}"
DIR="${2:-}"
[ -n "$SRC" ] && [ -n "$DIR" ] || { echo "использование: shots.sh <app|zip> <папка> [--lang ru|en] [--video]" >&2; exit 0; }
shift 2
LANG_UI=ru
VIDEO=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --lang) LANG_UI="${2:-ru}"; shift ;;
    --video) VIDEO=1 ;;
    *) echo "::warning::shots.sh: неизвестный ключ $1" ;;
  esac
  shift
done
case "$LANG_UI" in ru | en) ;; *) LANG_UI=ru ;; esac

out_init "$DIR"
WORK="${RUNNER_TEMP:-$(mktemp -d)}/shots"
rm -rf "$WORK"
mkdir -p "$WORK"
MACOS="$(sw_vers -productVersion 2>/dev/null)"
MAJOR="${MACOS%%.*}"
SETTINGS_ID="com.apple.systempreferences"
# Надписи кнопки на 15+: какая из русских — покажет снимок (открытый вопрос спеки)
OPEN_ANYWAY=("Open Anyway" "Открыть всё равно" "Открыть все равно" "Всё равно открыть" "Все равно открыть")
# Владельцы окна Gatekeeper в списке окон (на 14 бывает и UserNotificationCenter)
GK_OWNERS='CoreServicesUIAgent|UserNotificationCenter'
# Кнопки окна отказа, которые ничего не удаляют («В Корзину» не нажимаем)
GK_DONE=("Done" "Готово" "OK" "ОК" "Cancel" "Отменить" "Отмена")
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1

summary_head
section "Снимки (shots.sh, язык $LANG_UI)"

# ---------- подготовка ----------

pids_of() { pgrep -x "$1" 2>/dev/null | tr '\n' ' '; }
# «System Preferences» длиннее 16 знаков — pgrep -x его не видит, ищем по пути
settings_pids() { pgrep -f '/System (Settings|Preferences)[.]app/Contents/MacOS/' 2>/dev/null | tr '\n' ' '; }
terminal_pids() { pids_of Terminal; }

# wait_win_fn <функция, печатающая pid-ы> <сек> — процесс мог ещё не стартовать, pid-ы берём заново
wait_win_fn() {
  local i=0 p
  while [ "$i" -lt "$2" ]; do
    p="$($1)"
    if [ -n "${p// /}" ] && find_window "$p" "" largest >/dev/null 2>&1; then return 0; fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

prepare_app() {
  local a n=0
  case "$SRC" in
    *.zip)
      mkdir -p "$WORK/app"
      ditto -x -k "$SRC" "$WORK/app" 2>>"$OUT/logs/shots-unzip.txt" || return 1
      for a in "$WORK"/app/*.app; do [ -d "$a" ] && n=$((n + 1)) && APP="$a"; done
      [ "$n" -eq 1 ] || return 1
      ;;
    *.app) APP="$SRC" ;;
    *) return 1 ;;
  esac
  EXE="$(plist_get "$APP" CFBundleExecutable)"
  [ -n "$EXE" ]
}

setup_tools() {
  {
    sw_vers
    echo
    uname -a
  } >"$OUT/logs/sw_vers.txt" 2>&1
  system_profiler SPDisplaysDataType >"$OUT/logs/system_profiler.txt" 2>&1
  # Pillow — в своём venv: системный python3 из brew не даёт ставить пакеты (PEP 668)
  local v="$WORK/venv" ver
  if python3 -m venv "$v" >>"$OUT/logs/pip.txt" 2>&1; then
    for ver in 12.0.0 11.3.0; do
      if with_timeout 180 "$v/bin/python" -m pip install -q --disable-pip-version-check "pillow==$ver" >>"$OUT/logs/pip.txt" 2>&1; then
        PY="$v/bin/python"
        row "Pillow" "да" "pillow==$ver"
        break
      fi
    done
  fi
  [ "$PY" = python3 ] && row "Pillow" "нет" "не поставился (logs/pip.txt): changed = null, ok — по списку окон"
  if ! command -v cliclick >/dev/null 2>&1; then
    with_timeout 180 brew install cliclick >>"$OUT/logs/brew.txt" 2>&1
  fi
  if windows_bin >/dev/null; then
    row "windows.swift" "да" "собран swiftc"
  else
    row "windows.swift" "предупреждение" "swiftc не собрал (logs/swiftc.txt) — запуск через swift"
  fi
}

# §3.6 — язык до первого запуска «Настроек» и окна Gatekeeper; перезайти в систему нельзя
set_lang() {
  local loc langs p
  if [ "$LANG_UI" = ru ]; then langs="ru-RU en"; loc=ru_RU; else langs="en-US"; loc=en_US; fi
  # shellcheck disable=SC2086 # список языков — отдельными словами
  defaults write -g AppleLanguages -array $langs
  defaults write -g AppleLocale "$loc"
  # shellcheck disable=SC2086
  sudo -n defaults write /Library/Preferences/.GlobalPreferences AppleLanguages -array $langs >>"$OUT/logs/lang.txt" 2>&1
  sudo -n defaults write /Library/Preferences/.GlobalPreferences AppleLocale "$loc" >>"$OUT/logs/lang.txt" 2>&1
  for p in cfprefsd SystemUIServer Dock Finder ControlCenter; do killall "$p" 2>/dev/null; done
  restart_gk_agents
  sleep 4
  row "язык" "да" "AppleLanguages: $(defaults read -g AppleLanguages 2>/dev/null | tr -d ' \n'); агенты окон Gatekeeper перезапущены (logs/lang.txt); итог — глазами по снимкам"
}

# agents_ps — агенты окон Gatekeeper и пароля: pid, владелец, время старта
agents_ps() {
  local p
  p="$(pgrep -f 'CoreServicesUIAgent|UserNotificationCenter|SecurityAgent' 2>/dev/null | tr '\n' ',')"
  [ -n "$p" ] && ps -o pid=,user=,lstart=,command= -p "${p%,}"
}

# restart_gk_agents — прогон 1: CoreServicesUIAgent запущен при загрузке на en-US и пережил killall,
# окно Gatekeeper вышло английским. Агент читает язык при старте — перезапуск до первой попытки открыть
restart_gk_agents() {
  local f="$OUT/logs/lang.txt"
  {
    echo "== агенты до перезапуска"
    agents_ps
    launchctl kickstart -k "gui/$(id -u)/com.apple.coreservices.uiagent" 2>&1
  } >>"$f"
  kill_gk_agents
  sudo -n pkill -f '/SecurityAgent[.]bundle/Contents/MacOS/' 2>/dev/null
  sleep 2
  {
    echo "== агенты после (новые pid — перезапуск прошёл)"
    agents_ps
  } >>"$f"
}

set_wallpaper() {
  local d="/System/Library/Desktop Pictures/Solid Colors" f="" n
  for n in "Stone.png" "Space Gray.png" "Silver.png" "Sky Blue.png"; do
    [ -f "$d/$n" ] && f="$d/$n" && break
  done
  if [ -z "$f" ]; then
    for n in "$d"/*; do [ -f "$n" ] && f="$n" && break; done
  fi
  if [ -n "$f" ] && osascript -e "tell application \"System Events\" to tell every desktop to set picture to \"$f\"" >>"$OUT/logs/wallpaper.txt" 2>&1; then
    row "обои" "да" "$(basename "$f")"
  else
    row "обои" "предупреждение" "однотонные не поставились — остались как есть"
  fi
  sleep 2
}

# Сброс между шагами: закрыть программу, «Настройки», окно Gatekeeper и «Терминал»
reset_all() {
  pkill -x "$EXE" 2>/dev/null
  osascript -e "quit app id \"$SETTINGS_ID\"" >/dev/null 2>&1
  kill_gk_agents
  killall Terminal 2>/dev/null
  # open из прошлого шага ещё ждёт ответа окна Gatekeeper
  pkill -f "^open $DST" 2>/dev/null
  sleep 1
  pkill -KILL -x "$EXE" 2>/dev/null
  pkill -x "System Settings" 2>/dev/null
  pkill -x "System Preferences" 2>/dev/null
  key_code 53
  sleep 1
}

# Свежая «скачанная» копия с меткой карантина. DST и XATTR_CMD заданы до шагов: шаги идут
# в подоболочке with_timeout, их переменные наружу не выходят
fresh_download() {
  rm -rf "$DST"
  ditto "$APP" "$DST" && quarantine "$DST"
}

# tcc_fix — §3.4: снимки — одни обои; права записи экрана реальным клиентам в обе TCC.db
tcc_fix() {
  local ts db n c v ok=1 ff
  ts="$(date +%s)"
  ff="$(command -v ffmpeg 2>/dev/null)"
  [ -n "$ff" ] && ff="$("$PY" -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$ff")"
  for db in "/Library/Application Support/com.apple.TCC/TCC.db" "$HOME/Library/Application Support/com.apple.TCC/TCC.db"; do
    n="$(sudo -n sqlite3 "$db" "SELECT COUNT(*) FROM pragma_table_info('access');" 2>>"$OUT/logs/tcc.txt")"
    for c in /usr/sbin/screencapture /usr/bin/osascript /bin/bash "$ff"; do
      [ -n "$c" ] || continue
      if ! v="$(tcc_values "$n" kTCCServiceScreenCapture "$c" 1 "$ts")"; then
        echo "схема $db: столбцов '$n'" >>"$OUT/logs/tcc.txt"
        break
      fi
      sudo -n sqlite3 "$db" "INSERT OR REPLACE INTO access VALUES($v);" >>"$OUT/logs/tcc.txt" 2>&1 && ok=0
    done
  done
  return "$ok"
}

enable_gatekeeper() {
  sudo -n spctl --global-enable >>"$OUT/logs/gatekeeper.txt" 2>&1 ||
    sudo -n spctl --master-enable >>"$OUT/logs/gatekeeper.txt" 2>&1
}

# ---------- шаги (design §3.3); каждый — под with_timeout, неудача пишет «нет» ----------

step_desktop() { shot 01-desktop "" "" largest; }

step_app() {
  local pid why=""
  stop_app "$APP"
  open "$APP" 2>>"$OUT/logs/open.txt"
  if ! pid="$(wait_app "$APP" 30)"; then
    "$APP/Contents/MacOS/$EXE" >/dev/null 2>"$OUT/logs/app-direct.err" &
    why="open не дал процесса — запуск напрямую"
    pid="$(wait_app "$APP" 15)" || { row 02-app-window нет "процесс не запустился"; return 1; }
  fi
  wait_window "$pid" "" 30 || why="${why:+$why; }окна нет за 30 с"
  sleep 3 # WebView дорисовывает интерфейс
  shot 02-app-window "$pid" "" largest -- "$why"
  local rc=$?
  stop_app "$APP"
  return "$rc"
}

step_gatekeeper() {
  local st st0 why=""
  reset_all
  fresh_download || { row 03-gatekeeper нет "копия с меткой карантина не создалась"; return 1; }
  st0="$(spctl --status 2>&1)"
  st="$st0"
  case "$st0" in
    *disabled*)
      enable_gatekeeper
      st="$(spctl --status 2>&1)"
      why="Gatekeeper был выключен; после spctl: $st"
      ;;
  esac
  # open ждёт ответа окна — в фоне; окно остаётся до step_privacy («Готово» — часть инструкции)
  open_bg "$DST" >/dev/null
  wait_win_fn gk_pids 20 || why="${why:+$why; }окна агентов Gatekeeper нет за 20 с"
  sleep 1
  # Владелец — запасной путь: на 14 окно бывает у UserNotificationCenter, pid мог смениться
  shot 03-gatekeeper "$(gk_pids)" "$GK_OWNERS" front -- "spctl: $st${why:+; $why}"
}

# dismiss_gk — «Готово» (15+) или «OK»/«Отменить» в окне Gatekeeper, курсор к кнопке — для видео
dismiss_gk() {
  local b res
  for b in com.apple.coreservices.uiagent com.apple.UserNotificationCenter; do
    res="$(axfind "$b" find "${GK_DONE[@]}")"
    [ -n "$res" ] || continue
    cursor_to "${res%%"$TAB"*}"
    sleep 1
    axfind "$b" click "${GK_DONE[@]}" >/dev/null
    printf '%s' "${res#*"$TAB"}"
    return 0
  done
  return 1
}

open_privacy() {
  local why=""
  open "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension" 2>>"$OUT/logs/open.txt"
  if ! wait_win_fn settings_pids 20; then
    open "x-apple.systempreferences:com.apple.preference.security?General" 2>>"$OUT/logs/open.txt"
    why="запасной адрес preference.security"
    wait_win_fn settings_pids 20 || why="$why; окна «Настроек» нет"
  fi
  sleep 3
  printf '%s' "$why"
}

step_privacy() {
  local why b
  # «Готово» в окне отказа: на 15+ после него в «Настройках» появляется «Открыть всё равно»
  if b="$(dismiss_gk)"; then
    why="в окне Gatekeeper нажата «$b»"
  else
    why="кнопки в окне Gatekeeper не нашлось"
  fi
  sleep 1
  kill_gk_agents
  b="$(open_privacy)"
  shot 04-privacy "$(settings_pids)" "" largest -- "$why${b:+; $b}"
}

# wheel_settings — колесо мыши над правой частью окна «Настроек» (windows.swift scroll)
wheel_settings() {
  local line b
  line="$(find_window "$(settings_pids)" "" largest)" || return 1
  # shellcheck disable=SC2086 # «id x y w h» → пять чисел
  set -- $line
  [ "$#" -eq 5 ] || return 1
  if b="$(windows_bin)"; then
    "$b" scroll "$(($2 + $4 * 7 / 10))" "$(($3 + $5 / 2))" 60 2>>"$OUT/logs/windows-err.txt"
  else
    swift "$MAC_DIR/windows.swift" scroll "$(($2 + $4 * 7 / 10))" "$(($3 + $5 / 2))" 60 2>>"$OUT/logs/windows-err.txt"
  fi
}

# find_open_anyway [click] — рамка и надпись кнопки. Кнопка внизу панели, в разделе «Безопасность»;
# SwiftUI строит его только при прокрутке (прогон 1: End/Page Down не помогли — фокус в боковом меню).
# Порядок: как есть → полоса прокрутки через доступность → колесо мыши → адрес раздела ?Security
find_open_anyway() {
  local act="${1:-find}" res how
  for how in none ax wheel url; do
    case "$how" in
      ax) note "Open Anyway: прокрутка AX — $(axfind "$SETTINGS_ID" scroll)" ;;
      wheel) wheel_settings && note "Open Anyway: колесо мыши" ;;
      url)
        open "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension?Security" 2>>"$OUT/logs/open.txt"
        sleep 2
        axfind "$SETTINGS_ID" scroll >/dev/null
        ;;
    esac
    [ "$how" = none ] || sleep 1
    res="$(axfind "$SETTINGS_ID" "$act" "${OPEN_ANYWAY[@]}")"
    if [ -n "$res" ]; then
      note "Open Anyway: найдена ($how)"
      printf '%s' "$res"
      return 0
    fi
  done
  return 1
}

step_open_anyway() {
  local res why=""
  [ -n "$(settings_pids | tr -d ' ')" ] || why="$(open_privacy)"
  if ! res="$(find_open_anyway find)"; then
    shot 05-open-anyway "$(settings_pids)" "" largest "Open Anyway" "" -- \
      "${why:+$why; }нет ни после прокрутки (AX, колесо), ни по адресу ?Security — была ли попытка открыть (03)?"
    return 1
  fi
  cursor_to "${res%%"$TAB"*}"
  shot 05-open-anyway "$(settings_pids)" "" largest "${res#*"$TAB"}" "${res%%"$TAB"*}" -- "$why"
}

step_confirm() {
  local res pids
  # Панель уже прокручена в 05 — одна попытка; цепочка прокруток здесь съела бы тайм-аут шага
  res="$(axfind "$SETTINGS_ID" click "${OPEN_ANYWAY[@]}")"
  [ -n "$res" ] || { row 06-confirm нет "кнопки «Открыть всё равно» нет — нажимать нечего"; return 1; }
  sleep 3
  pids="$(pids_of SecurityAgent) $(gk_pids)"
  [ -n "${pids// /}" ] || pids="$(settings_pids)"
  shot 06-confirm "$pids" "SecurityAgent|$GK_OWNERS" front -- "нажата «${res#*"$TAB"}»; пароль раннера неизвестен — дальше не идём"
  local rc=$?
  key_code 53
  sleep 1
  kill_gk_agents
  return "$rc"
}

# terminal_xattr — команда из инструкции в окне «Терминала»; печатает примечание
terminal_xattr() {
  local f="$WORK/xattr.command"
  # do script — AppleEvents к Terminal; если права нет (osascript ждёт согласия) — файл .command
  if ! with_timeout 20 osascript -e 'on run argv' -e 'tell application "Terminal"' -e 'activate' \
    -e 'do script (item 1 of argv)' -e 'try' -e 'set font size of window 1 to 18' -e 'end try' \
    -e 'end tell' -e 'end run' "$XATTR_CMD" >>"$OUT/logs/terminal.txt" 2>&1; then
    printf '#!/bin/bash\nclear\necho "$ %s"\n%s\n' "$XATTR_CMD" "$XATTR_CMD" >"$f"
    chmod 755 "$f"
    open -a Terminal "$f"
    printf '%s' "AppleEvents к Terminal не прошли — файл .command"
  fi
  wait_win_fn terminal_pids 15
}

step_terminal() {
  local why
  reset_all
  fresh_download || { row 07-terminal-xattr нет "копия с меткой карантина не создалась"; return 1; }
  why="$(terminal_xattr)"
  sleep 3
  if xattr -p com.apple.quarantine "$DST" >/dev/null 2>&1; then
    why="${why:+$why; }метка осталась — команда не выполнилась"
  fi
  shot 07-terminal-xattr "$(terminal_pids)" "" largest -- "команда: $XATTR_CMD${why:+; $why}"
}

step_after_xattr() {
  local pid why=""
  killall Terminal 2>/dev/null
  [ -d "$DST" ] || fresh_download
  if xattr -p com.apple.quarantine "$DST" >/dev/null 2>&1; then
    xattr -dr com.apple.quarantine "$DST"
    why="метку сняли здесь, не из «Терминала»"
  fi
  open "$DST" 2>>"$OUT/logs/open.txt"
  pid="$(wait_app "$DST" 20)" || { row 08-app-after-xattr нет "процесс не появился после open"; return 1; }
  wait_window "$pid" "" 30
  sleep 3
  shot 08-app-after-xattr "$pid" "" largest -- "$why"
  local rc=$?
  stop_app "$DST"
  return "$rc"
}

# 10–11: правая кнопка → «Открыть»; обходит проверку только до macOS 14
step_context() {
  local res rect cx cy name mpid="" why=""
  if [ "${MAJOR:-0}" -ge 15 ]; then
    row 10-context-menu пропуск "macOS $MACOS: с 15 правая кнопка → «Открыть» проверку не обходит"
    row 11-open-dialog пропуск "то же"
    return 0
  fi
  reset_all
  fresh_download || { row 10-context-menu нет "копия с меткой карантина не создалась"; return 1; }
  name="$(basename "$DST" .app)"
  open -R "$DST"
  sleep 3
  osascript -e 'tell application "Finder" to set current view of Finder window 1 to icon view' >/dev/null 2>&1
  sleep 1
  res="$(axfind com.apple.finder selected "$name")"
  [ -n "$res" ] || { row 10-context-menu нет "выделенный значок $name не найден"; return 1; }
  rect="${res%%"$TAB"*}"
  # shellcheck disable=SC2086 # «x y w h» → четыре числа
  set -- $rect
  cx=$(($1 + $3 / 2))
  cy=$(($2 + $4 / 2))
  if command -v cliclick >/dev/null 2>&1; then
    cliclick "rc:$cx,$cy"
  else
    osascript "$MAC_DIR/axfind.applescript" com.apple.finder menu "$name" >/dev/null 2>&1 &
    mpid=$!
    why="меню через AXShowMenu (cliclick нет)"
  fi
  sleep 2
  shot 10-context-menu "$(pids_of Finder)" "" front "$name" "$rect" -- "$why"
  # первый пункт меню Finder — «Открыть»
  key_code 125
  key_code 36
  wait_win_fn gk_pids 15
  sleep 1
  shot 11-open-dialog "$(gk_pids)" "$GK_OWNERS" front
  key_code 53
  kill_gk_agents
  [ -n "$mpid" ] && kill "$mpid" 2>/dev/null
  return 0
}

# run_step <сек> <функция> — тайм-аут и запись о падении; следующий шаг идёт в любом случае
run_step() {
  local rc
  note "шаг $2"
  with_timeout "$1" "$2"
  rc=$?
  [ "$rc" -ge 124 ] && row "$2" "нет" "прерван (код $rc, тайм-аут $1 с?)"
  return "$rc"
}

# ---------- прогон ----------

if ! prepare_app; then
  row "программа" "нет" "не удалось взять .app из $SRC"
  exit 0
fi
DST="$HOME/Downloads/$(basename "$APP")"
XATTR_CMD="xattr -dr com.apple.quarantine ~/Downloads/$(basename "$APP")"
note "программа: $APP ($EXE), macOS $MACOS"
setup_tools
set_lang
reset_all
set_wallpaper
# Видео пишется параллельно со снимками: в прогоне 1 отдельный сценарий висел на open, ролик
# вышел застывшим (движение 0,02). Запись — до 01, остановка — после 08/11
if [ "$VIDEO" = 1 ]; then
  video_start
else
  row "видео" "пропуск" "без --video"
fi
run_step 60 step_desktop

run_step 90 step_app
# §3.4: окно в списке есть, а снимок не отличается от стола — нет права записи экрана
if [ "$("$PY" "$MAC_DIR/macshots.py" get --out "$OUT" --name 02-app-window --field ok 2>/dev/null)" = "false" ] &&
  "$PY" "$MAC_DIR/macshots.py" get --out "$OUT" --name 02-app-window --field why 2>/dev/null | grep -q "обои"; then
  if tcc_fix; then
    row "TCC.db" "да" "права записи экрана вписаны (logs/tcc.txt) — повтор 01 и 02"
    # стол заново: без права строки меню на снимке не было, и она одна дала бы «изменилось»
    run_step 60 step_desktop
    run_step 90 step_app
  else
    row "TCC.db" "нет" "sqlite3 не записал (logs/tcc.txt) — снимки останутся обоями"
  fi
fi

run_step 60 step_gatekeeper
run_step 90 step_privacy
run_step 240 step_open_anyway
run_step 60 step_confirm
run_step 90 step_terminal
run_step 60 step_after_xattr
run_step 120 step_context
reset_all
[ "$VIDEO" = 1 ] && video_stop

reset_all
note "готово: $OUT"
exit 0
