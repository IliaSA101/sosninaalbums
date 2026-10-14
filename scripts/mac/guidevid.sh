# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Ролик-инструкция Mac v2 (036 design §4): все переходы — щелчками по шагам guide.md, без открытия по адресу
# (адрес — только запасной путь, когда цель не нашлась). Рядом с живой записью video/raw.mp4 пишется журнал
# video/steps.jsonl (формат — design §2, строки пишет macshots.py journal): перед каждым действием hold
# (стоп-кадр без нажатия), затем живые move/keys/type, ожидание раннера — idle. Монтирует scripts/guide/overlay.py.
# Подключается из shots.sh (ключ --guide-video); при подключении ничего не запускает. Под bash 3.2 (см. lib.sh).
# Функции зовутся по имени из shots.sh: MACOS, MAJOR, LANG_UI, APP, DST, EXE, WORK, OUT, SETTINGS_ID,
# OPEN_ANYWAY, GK_OWNERS и шаговые функции — оттуда же.
# shellcheck disable=SC2329

GV_FLASH="#d81f26"
GV_JOURNAL=""
GV_CLOCK=""
GV_WIN=""
GV_IDLE0=""
GV_PEND_CHAPTER=""
GV_PEND_STEP=""
VDST=""
VXATTR=""

# ---------- чистые функции (проверяет test_lib.sh) ----------

# gv_rect_of <«x y w h<TAB>подпись»> — только рамка
gv_rect_of() {
  printf '%s' "${1%%"$TAB"*}"
}

# gv_centre <«x y w h»> — «cx cy»; не четыре числа — код 1
gv_centre() {
  # shellcheck disable=SC2086 # «x y w h» → четыре слова
  set -- $1
  [ "$#" -eq 4 ] || return 1
  awk -v x="$1" -v y="$2" -v w="$3" -v h="$4" 'BEGIN { printf "%d %d", x + w / 2, y + h / 2 }'
}

# gv_second_row — stdin: строки «x y w h<TAB>текст» от ocr; печатает вторую сверху: первая — поле ввода Spotlight
# с тем же текстом, вторая — найденный пункт. Одна строка — пусто (пункт не появился)
gv_second_row() {
  awk -F'\t' '{ n = split($1, a, " "); if (n == 4) print a[2] "\t" $0 }' | sort -n | cut -f2- |
    awk -F'\t' '
      NR == 1 { split($1, a, " "); y0 = a[2]; next }
      { split($1, a, " "); if (a[2] > y0 + 20) { print; exit } }'
}

# ---------- журнал ----------

gv_jl() {
  "$PY" "$MAC_DIR/macshots.py" journal --file "$GV_JOURNAL" --clock "$GV_CLOCK" "$@" 2>>"$OUT/logs/journal-err.txt"
}

gv_now() {
  gv_jl now
}

# gv_close_idle — открытый простой (с GV_IDLE0 до сейчас) в журнал; после этого простоя нет
gv_close_idle() {
  if [ -n "$GV_IDLE0" ]; then
    gv_jl idle "t0=$GV_IDLE0" t1=@now
  fi
  GV_IDLE0=""
}

# gv_note <текст> — строка note в журнале и в steps.txt. Открытый простой закрывается до заметки и
# продолжается после неё: иначе note внутри простоя срезает его начало (время в журнале не убывает)
gv_note() {
  local was="$GV_IDLE0"
  gv_close_idle
  gv_jl note t=@now "text=$*"
  note "ролик: $*"
  if [ -n "$was" ]; then
    GV_IDLE0="$(gv_now)"
  fi
}

# Глава и шаг пишутся при первом действии: перед ними стоит простой с конца прошлого действия
gv_chapter() { GV_PEND_CHAPTER="$1"; }
gv_step() { GV_PEND_STEP="$1${TAB}$2${TAB}$3"; }

# gv_flush — idle (с конца прошлого действия), глава, шаг: всё в порядке времени, затем hold
gv_flush() {
  local n of title rest
  gv_close_idle
  if [ -n "$GV_PEND_CHAPTER" ]; then
    gv_jl chapter t=@now "title=$GV_PEND_CHAPTER"
    GV_PEND_CHAPTER=""
  fi
  if [ -n "$GV_PEND_STEP" ]; then
    n="${GV_PEND_STEP%%"$TAB"*}"
    rest="${GV_PEND_STEP#*"$TAB"}"
    of="${rest%%"$TAB"*}"
    title="${rest#*"$TAB"}"
    gv_jl step t=@now "n:=$n" "of:=$of" "title=$title"
    GV_PEND_STEP=""
  fi
}

# gv_settle [сек] — пауза после действия, чтобы было видно, что произошло; с её конца идёт простой
gv_settle() {
  sleep "${1:-1.2}"
  GV_IDLE0="$(gv_now)"
}

# gv_hold <рамка или пусто> <подпись> [клавиши json] — стоп-кадр: цель найдена, ничего ещё не нажато
gv_hold() {
  gv_flush
  if [ -n "${3:-}" ]; then
    gv_jl hold t=@now "rect=$1" "caption=$2" "keys:=$3"
  else
    gv_jl hold t=@now "rect=$1" "caption=$2"
  fi
  [ -n "$1" ] || note "ролик: «$2» — цель не найдена, без стрелки"
}

# ---------- примитивы действий ----------

# gv_cursor_to <рамка> — курсор плавно к центру (cliclick -e 200: путь виден, но не 8 с, как при 600), в журнале move
gv_cursor_to() {
  local c
  c="$(gv_centre "$1")" || return 0
  cliclick -e 200 "m:${c% *},${c#* }" 2>>"$OUT/logs/cliclick-err.txt"
  gv_jl move t0=@end t1=@now
}

# act_click <рамка или пусто> <подпись> <c|dc|rc> [запасная команда…] — цель не найдена: стоп-кадр без стрелки
# и запасной путь (по адресу, клавишами)
act_click() {
  local rect="$1" cap="$2" mode="$3" c
  shift 3
  gv_hold "$rect" "$cap"
  if [ -n "$rect" ] && c="$(gv_centre "$rect")"; then
    gv_cursor_to "$rect"
    cliclick "$mode:${c% *},${c#* }" 2>>"$OUT/logs/cliclick-err.txt"
  elif [ "$#" -gt 0 ]; then
    # запасной путь долгий и без видимого действия — это простой, он ускоряется
    GV_IDLE0="$(gv_now)"
    gv_note "запасной путь для «${cap}»"
    "$@"
    gv_close_idle
  fi
  gv_settle 1.2
}

# act_keys <рамка или пусто> <подпись> <клавиши json> <команда…> — стоп-кадр с плашкой клавиш, затем нажатие
act_keys() {
  local rect="$1" cap="$2" glyphs="$3"
  shift 3
  gv_hold "$rect" "$cap" "$glyphs"
  "$@"
  gv_jl keys t0=@end t1=@now "keys:=$glyphs"
  gv_settle 1.2
}

# act_info <подпись> — стоп-кадр без действия (окно программы, подсказка)
act_info() {
  gv_hold "" "$1"
  GV_IDLE0="$(gv_now)"
}

gv_key_return() { cliclick kp:return 2>>"$OUT/logs/cliclick-err.txt"; }
gv_key_spotlight() {
  cliclick kd:cmd kp:space ku:cmd 2>>"$OUT/logs/cliclick-err.txt"
  sleep 1
  # горячую клавишу система иногда не берёт из CGEvent — тогда System Events
  find_window "" "Spotlight" front >/dev/null 2>&1 ||
    osascript -e 'tell application "System Events" to key code 49 using command down' >/dev/null 2>&1
}
gv_key_paste() { osascript -e 'tell application "System Events" to keystroke "v" using command down' >/dev/null 2>&1; }
gv_clear_field() {
  osascript -e 'tell application "System Events" to keystroke "a" using command down' >/dev/null 2>&1
  osascript -e 'tell application "System Events" to key code 51' >/dev/null 2>&1
}

# gv_type_slow <текст> — по знаку, ~7 знаков/с (cliclick t: пишет знак как Unicode, кириллица подходит)
gv_type_slow() {
  local ch
  while IFS= read -r ch; do
    cliclick "t:$ch" 2>>"$OUT/logs/cliclick-err.txt"
    sleep 0.14
  done < <("$PY" -c 'import sys; print("\n".join(sys.argv[1]))' "$1")
}

# gv_spotlight_hit — «рамка<TAB>текст» найденного пункта Spotlight («Терминал»/«Terminal»), иначе пусто
gv_spotlight_hit() {
  ocr_hits "Терминал" "Terminal" | gv_second_row
}

# act_type <подпись> <текст> — набор в Spotlight. Сразу английское имя («Terminal»): Spotlight находит по нему и
# русский «Терминал», а кириллица по знаку у раннера (раскладка US) не набирается. Не набралось (пункта нет) —
# второй раз через System Events. Неудачная попытка в журнал шагом не идёт: простой (ускоряется) и заметка;
# событие type — только последняя попытка
act_type() {
  local cap="$1" text="$2" try t0
  gv_hold "" "$cap"
  for try in 1 2; do
    t0="$(gv_now)"
    if [ "$try" = 1 ]; then
      gv_type_slow "$text"
    else
      osascript -e "tell application \"System Events\" to keystroke \"${text}\"" >/dev/null 2>&1
    fi
    sleep 1
    [ -n "$(gv_spotlight_hit)" ] && break
    [ "$try" = 2 ] && break
    gv_clear_field
    gv_jl idle "t0=$t0" t1=@now
    gv_note "Spotlight: «${text}» не набралось через cliclick — System Events"
  done
  gv_jl type "t0=$t0" t1=@now "text=$text"
  gv_settle 1.0
}

# gv_wheel <x> <y> <порций> — колесо мыши (windows.swift scroll)
gv_wheel() {
  local b
  if b="$(windows_bin)"; then
    "$b" scroll "$1" "$2" "$3" 2>>"$OUT/logs/windows-err.txt"
  else
    swift "$MAC_DIR/windows.swift" scroll "$1" "$2" "$3" 2>>"$OUT/logs/windows-err.txt"
  fi
}

# act_scroll_until <x> <y> <проверка…> — прокрутка порциями с паузами, пока проверка не пройдёт (до 6 порций);
# в журнале — один move
act_scroll_until() {
  local x="$1" y="$2" i t0
  shift 2
  gv_flush
  t0="$(gv_now)"
  for i in 1 2 3 4 5 6; do
    "$@" >/dev/null 2>&1 && break
    gv_wheel "$x" "$y" 4
    sleep 0.3
  done
  gv_jl move "t0=$t0" t1=@now
  GV_IDLE0="$(gv_now)"
}

# gv_win_point <pid-ы> <доля x, %> <доля y, %> — точка внутри окна «x y»
gv_win_point() {
  local line px="$2" py="$3"
  line="$(find_window "$1" "" largest)" || return 1
  # shellcheck disable=SC2086 # «id x y w h» → пять слов
  set -- $line
  [ "$#" -eq 5 ] || return 1
  printf '%s %s' "$(($2 + $4 * px / 100))" "$(($3 + $5 * py / 100))"
}

# finder_item_rect <имя> — рамка выделенного значка в окне Finder (AX selected, как в step_context)
finder_item_rect() {
  gv_rect_of "$(axfind com.apple.finder selected "$1")"
}

# ---------- подготовка ----------

gv_stock_wallpaper() {
  local d="/System/Library/Desktop Pictures/Solid Colors" n
  for n in "Stone.png" "Space Gray.png" "Silver.png" "Sky Blue.png"; do
    [ -f "$d/$n" ] && { printf '%s' "$d/$n"; return 0; }
  done
  return 1
}

gv_wall_set() {
  osascript -e "tell application \"System Events\" to tell every desktop to set picture to \"$1\"" >>"$OUT/logs/wallpaper.txt" 2>&1
}

# gv_flash — метка синхронизации: стол на 0,8 с красный. Журнал пишет t после возврата osascript — обои
# рисуются в пределах долей секунды, смещение монтаж берёт по первому красному кадру записи
gv_flash() {
  local png="$WORK/sync.png" stock
  stock="$(gv_stock_wallpaper)"
  "$PY" "$MAC_DIR/macshots.py" solid --color "$GV_FLASH" --windows "$GV_WIN" --out "$png" || return 1
  gv_wall_set "$png" || return 1
  gv_jl sync t=@now kind=flash "color=$GV_FLASH" dur:=0.8
  sleep 0.8
  [ -n "$stock" ] && gv_wall_set "$stock"
  sleep 0.5
}

# gv_install_copy — свежая «скачанная» копия с меткой карантина
gv_install_copy() {
  rm -rf "$VDST" 2>/dev/null || sudo -n rm -rf "$VDST"
  if ! ditto "$APP" "$VDST" 2>>"$OUT/logs/guidevid-err.txt"; then
    sudo -n ditto "$APP" "$VDST" 2>>"$OUT/logs/guidevid-err.txt" && sudo -n chown -R "$(id -un)" "$VDST"
  fi
  quarantine "$VDST"
}

# gv_finder_show — окно Finder с выделенной программой (путь «Программы» из guide.md), значки крупно
gv_finder_show() {
  osascript -e 'tell application "Finder" to close every window' >/dev/null 2>&1
  open -R "$VDST" 2>>"$OUT/logs/open.txt"
  sleep 3
  osascript -e 'tell application "Finder" to set current view of Finder window 1 to icon view' >/dev/null 2>&1
  sleep 1
}

# gv_guard — окно запроса «hosted-compute-agent» поверх экрана (решение 66) закрываем и пишем note
gv_guard() {
  if find_window "" "UserNotificationCenter" front >/dev/null 2>&1; then
    gv_note "поверх экрана окно запроса системы — закрыто Escape; стоп-кадр с ним не годен"
    key_code 53
  fi
}

# gv_app_open — программа открылась (процесс, окно, WebView дорисовался)
gv_app_open() {
  local pid
  pid="$(wait_app "$VDST" 25)" || return 1
  wait_window "$pid" "" 30
  sleep 3
}

# ---------- запасные пути (цель не найдена) ----------

gv_fb_open_app() { open_bg "$VDST" >/dev/null; }
gv_fb_dismiss() { dismiss_gk >/dev/null; }
gv_fb_settings() {
  key_code 53
  open -b "$SETTINGS_ID" 2>>"$OUT/logs/open.txt"
}
gv_fb_privacy() { open "x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension" 2>>"$OUT/logs/open.txt"; }
gv_fb_open_anyway() { axfind "$SETTINGS_ID" click "${OPEN_ANYWAY[@]}" >/dev/null; }
gv_fb_confirm() { open_anyway_ocr click >/dev/null; }
gv_fb_menu_open() {
  key_code 125
  key_code 36
}
gv_fb_context() { cliclick "rc:$1,$2" 2>>"$OUT/logs/cliclick-err.txt"; }

gv_confirm_pids() { printf '%s %s' "$(pids_of SecurityAgent)" "$(gk_pids)"; }
gv_have_privacy() { [ -n "$(privacy_rect)" ]; }
gv_have_open_anyway() { [ -n "$(open_anyway_ocr)" ]; }

# ---------- сценарии ----------

# macOS 15 и 26: Finder → «Готово» → меню Apple → «Настройки» → «Конфиденциальность» → «Все равно открыть»
gv_scene_gatekeeper() {
  local name rect pids p res
  name="$(basename "$VDST" .app)"
  gv_step 1 6 "Откройте программу"
  act_click "$(finder_item_rect "$name")" "Дважды щёлкните по ${name}" dc gv_fb_open_app
  wait_win_fn gk_pids 20
  sleep 1

  gv_step 2 6 "Закройте предупреждение"
  act_click "$(gv_rect_of "$(gk_button_rect)")" "Нажмите «Готово» — не «Переместить в Корзину»" c gv_fb_dismiss
  kill_gk_agents

  gv_step 3 6 "Откройте настройки"
  act_click "12 1 24 22" "Откройте меню Apple" c
  rect="$(gv_rect_of "$(apple_menu_rect)")"
  act_click "$rect" "Выберите «Системные настройки…»" c gv_fb_settings
  wait_win_fn settings_pids 20
  sleep 3

  gv_step 4 6 "Конфиденциальность и безопасность"
  if p="$(gv_win_point "$(settings_pids)" 12 60)"; then
    act_scroll_until "${p% *}" "${p#* }" gv_have_privacy
  fi
  act_click "$(gv_rect_of "$(privacy_rect)")" "Выберите «Конфиденциальность и безопасность»" c gv_fb_privacy
  sleep 2

  gv_step 5 6 "Разрешите запуск"
  if p="$(gv_win_point "$(settings_pids)" 70 55)"; then
    act_scroll_until "${p% *}" "${p#* }" gv_have_open_anyway
  fi
  res="$(open_anyway_ocr)"
  if [ -z "$res" ]; then
    # невидимый путь: AX, колесо, адрес ?Security — как в снимках
    res="$(find_open_anyway)"
    [ -n "$res" ] && gv_note "«Все равно открыть» найдена запасным путём: $(cat "$WORK/open-anyway-via" 2>/dev/null)"
  fi
  act_click "$(gv_rect_of "$res")" "Нажмите «Все равно открыть»" c gv_fb_open_anyway
  sleep 2
  wait_win_fn gv_confirm_pids 15
  pids="$(gv_confirm_pids)"
  act_info "Если спросит — введите пароль от Mac"
  act_click "$(gv_rect_of "$(confirm_button_rect "$pids")")" "Нажмите «Все равно открыть» в этом окне" c gv_fb_confirm

  gv_step 6 6 "Готово"
  gv_app_open || gv_note "программа не открылась за 25 с"
  act_info "Программа открыта. Дальше она открывается сразу"
  sleep 0.5
}

# macOS 12–14: правая кнопка → «Открыть» → «Открыть»
gv_scene_context() {
  local name rect c region
  name="$(basename "$VDST" .app)"
  gv_step 1 3 "Откройте программу"
  rect="$(finder_item_rect "$name")"
  c="$(gv_centre "$rect")"
  act_click "$rect" "Щёлкните правой кнопкой по ${name}" rc
  gv_step 2 3 "Выберите «Открыть»"
  if [ -n "$c" ]; then
    # меню раскрывается вправо и вниз от точки щелчка
    region="${c% *} ${c#* } 360 480"
    act_click "$(gv_rect_of "$(ocr_in "$region" "Open" "Открыть")")" "Выберите «Открыть»" c gv_fb_menu_open
  else
    act_click "" "Выберите «Открыть»" c gv_fb_menu_open
  fi
  wait_win_fn gk_pids 15
  sleep 1
  gv_step 3 3 "Подтвердите запуск"
  act_click "$(gv_rect_of "$(open_button_rect)")" "Нажмите «Открыть»" c gv_key_return
  gv_app_open || gv_note "программа не открылась за 25 с"
  act_info "Программа открыта. Дальше она открывается сразу"
  sleep 0.5
}

# Глава 2: «Терминал» через Spotlight, команда из буфера, программа снова
gv_scene_terminal() {
  local name rect
  name="$(basename "$VDST" .app)"
  gv_chapter "Если пишет «повреждено»"
  # свежая копия с меткой и чистый стол — в простое, он ускоряется
  stop_app "$VDST"
  reset_all
  gv_install_copy
  gv_finder_show

  gv_step 1 3 "Откройте «Терминал»"
  act_keys "" "Нажмите ⌘ + Пробел" '["⌘","Пробел"]' gv_key_spotlight
  act_type "Введите: Терминал (Terminal)" "Terminal"
  rect="$(gv_rect_of "$(gv_spotlight_hit)")"
  act_keys "$rect" "Нажмите Return" '["Return"]' gv_key_return
  if ! wait_win_fn terminal_pids 15; then
    gv_note "«Терминал» не открылся из Spotlight — open -a"
    open -a Terminal
    wait_win_fn terminal_pids 15
  fi
  sleep 2
  gv_guard

  gv_step 2 3 "Вставьте команду"
  printf '%s' "$VXATTR" | pbcopy
  act_keys "" "Вставьте команду: ⌘ + V" '["⌘","V"]' gv_key_paste
  rect="$(gv_rect_of "$(terminal_cmd_rect)")"
  act_keys "$rect" "Нажмите Return" '["Return"]' gv_key_return
  sleep 2
  gv_guard
  if xattr -p com.apple.quarantine "$VDST" >/dev/null 2>&1; then
    gv_note "метка карантина осталась после команды — снята здесь, не из «Терминала»"
    xattr -dr com.apple.quarantine "$VDST"
  fi

  # «Терминал» закрывается, Finder возвращается — в простое
  killall Terminal 2>/dev/null
  gv_finder_show
  gv_step 3 3 "Откройте программу снова"
  act_click "$(finder_item_rect "$name")" "Дважды щёлкните по ${name}" dc gv_fb_open_app
  gv_app_open || gv_note "программа не открылась за 25 с"
  act_info "Программа открылась — готово"
  sleep 0.5
}

# Сценарий целиком — под with_timeout (подоболочка): запись запущена снаружи. Время 0 журнала — meta
gv_scenario() {
  sleep 2
  gv_flash || gv_note "метка синхронизации не поставилась — монтаж откажет"
  gv_finder_show
  GV_IDLE0="$(gv_now)"
  gv_chapter "Первый запуск на macOS $MAJOR"
  if [ "${MAJOR:-0}" -ge 15 ]; then gv_scene_gatekeeper; else gv_scene_context; fi
  stop_app "$VDST"
  gv_scene_terminal
  gv_jl end t=@now
  stop_app "$VDST"
}

# guide_video_run — после снимков (shots.sh --guide-video): копия в «Программах», запись, сценарий, итог в summary.md
guide_video_run() {
  local f="$OUT/video/raw.mp4" info dur mb stats syncs
  section "Ролик-инструкция (guidevid.sh)"
  if ! ensure_ffmpeg; then
    row "ролик" "нет" "ffmpeg не поставился (logs/brew.txt)"
    return 0
  fi
  if ! command -v cliclick >/dev/null 2>&1; then
    row "ролик" "нет" "нет cliclick — щелчки и курсор невозможны"
    return 0
  fi
  GV_JOURNAL="$OUT/video/steps.jsonl"
  GV_CLOCK="$WORK/jclock"
  GV_WIN="$WORK/gv-windows.json"
  VDST="/Applications/$(basename "$APP")"
  if [ ! -w /Applications ] && ! sudo -n true 2>/dev/null; then
    VDST="$DST"
    row "ролик: копия" "предупреждение" "в «Программы» не записать — копия в ${DST%/*}"
  fi
  # команда из guide.md; путь — тот, где лежит копия
  VXATTR="xattr -dr com.apple.quarantine $VDST"
  reset_all
  set_wallpaper
  gv_install_copy
  if [ ! -d "$VDST" ]; then
    row "ролик: копия" "нет" "не создалась (logs/guidevid-err.txt)"
    return 0
  fi
  windows_json "$GV_WIN"
  "$PY" "$MAC_DIR/macshots.py" journal --file "$GV_JOURNAL" --clock "$GV_CLOCK" --windows "$GV_WIN" meta \
    os=macos "ver=$MACOS" "lang=$LANG_UI" recorder=avfoundation "script=guidevid 1" 2>>"$OUT/logs/journal-err.txt"
  if ! avf_start "$f" hq; then
    row "ролик" "нет" "запись не началась: $REC_WHY"
    rm -rf "$VDST"
    return 0
  fi
  run_step 600 gv_scenario
  stop_rec "$REC_PID"
  reset_all
  [ "$VDST" = "$DST" ] || rm -rf "$VDST" 2>/dev/null
  info="$(ffprobe -v error -select_streams v:0 -show_entries format=duration -of default=nw=1:nk=1 "$f" 2>>"$OUT/logs/ffprobe.txt")"
  dur="${info%%.*}"
  mb=$(($(stat -f %z "$f" 2>/dev/null || echo 0) / 1048576))
  syncs="$(grep -c '"ev":"sync"' "$GV_JOURNAL" 2>/dev/null)"
  if [ "${dur:-0}" -ge 10 ] && [ "${syncs:-0}" -ge 1 ]; then
    row "ролик: запись" "да" "raw.mp4 ${dur} с, ${mb} МБ, метка синхронизации в журнале есть"
  else
    row "ролик: запись" "нет" "raw.mp4 ${dur:-0} с, ${mb} МБ, меток синхронизации в журнале: ${syncs:-0}"
  fi
  if stats="$("$PY" "$MAC_DIR/macshots.py" jstat --file "$GV_JOURNAL" 2>/dev/null)"; then
    row "ролик" "да" "$stats"
  else
    row "ролик" "нет" "журнал не читается (video/steps.jsonl)"
  fi
  return 0
}
