# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Глава «Распакуйте архив» ролика-инструкции и сценарий целиком (039 design §5.2). Архив лежит в «Загрузках» с меткой
# карантина, как после Safari; двойной щелчок → Архиватор → папка «Sosnina Albums X.Y.Z» → двойной щелчок →
# перетаскивание sosninaalbums в «Программы». Дальше — главы 0.10 из guidevid.sh. Журнал и примитивы действий — там же.
# Подключается из shots.sh после guidevid.sh; при подключении ничего не запускает. Под bash 3.2 (см. lib.sh).
# Функции зовутся по имени из shots.sh: SRC, SCENES, WORK, OUT, DST, VDST, MAJOR, APP — оттуда же.
# shellcheck disable=SC2329

GV_SCENES="all"
GZ_DL="$HOME/Downloads"
GZ_ZIP=""
GZ_DIR=""
GZ_APP=""

# ---------- чистые функции (проверяет test_lib.sh) ----------

# gv_scenes_norm <all|unzip|classic> <источник> — какие главы снимать. Глава распаковки нужна zip: у .app
# архива нет — classic; незнакомое значение — all (как по умолчанию)
gv_scenes_norm() {
  case "$2" in
    *.zip) ;;
    *)
      printf 'classic'
      return 0
      ;;
  esac
  case "$1" in
    all | unzip | classic) printf '%s' "$1" ;;
    *) printf 'all' ;;
  esac
}

# gz_extracted <папка «Загрузок»> — папка первого уровня, в которой лежит распакованная программа (find_app).
# Сами .app не считаются: в «Загрузках» бывает копия шагов снимков. Нет такой — код 1
gz_extracted() {
  local d
  for d in "$1"/*/; do
    d="${d%/}"
    case "$d" in *.app) continue ;; esac
    [ -d "$d" ] || continue
    if find_app "$d" >/dev/null; then
      printf '%s' "$d"
      return 0
    fi
  done
  return 1
}

# gz_in_sidebar <«x y w h» цели> <«x y w h» окна> — центр цели в левой трети окна Finder: боковая панель, а не
# одноимённая папка в списке справа
gz_in_sidebar() {
  # shellcheck disable=SC2086 # две рамки → восемь слов
  set -- $1 $2
  [ "$#" -eq 8 ] || return 1
  awk -v x="$1" -v w="$3" -v wx="$5" -v ww="$7" 'BEGIN { c = x + w / 2; exit !(c >= wx && c <= wx + ww / 3) }'
}

# gz_steps <x0> <y0> <x1> <y1> <шагов> — промежуточные точки «x y» по прямой, без начала, включая конец
gz_steps() {
  awk -v x0="$1" -v y0="$2" -v x1="$3" -v y1="$4" -v n="$5" 'BEGIN {
    for (i = 1; i <= n; i++) printf "%d %d\n", x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n }'
}

# ---------- примитивы ----------

gz_icons() {
  osascript -e 'tell application "Finder" to set current view of Finder window 1 to icon view' >/dev/null 2>&1
}

# gz_finder_zip — окно «Загрузок» с выделенным архивом, значки крупно
gz_finder_zip() {
  osascript -e 'tell application "Finder" to close every window' >/dev/null 2>&1
  open -R "$GZ_ZIP" 2>>"$OUT/logs/open.txt"
  sleep 3
  gz_icons
  sleep 1
}

# gz_reveal <путь> — выделить в окне Finder (окно открывается или переиспользуется), значки крупно
gz_reveal() {
  open -R "$1" 2>>"$OUT/logs/open.txt"
  sleep 2
  gz_icons
  sleep 1
}

# gz_gk_window — «id x y w h» окна Gatekeeper, если оно сейчас на экране; иначе пусто
gz_gk_window() {
  local pids
  pids="$(gk_pids)"
  [ -n "$pids" ] || return 0
  find_window "$pids" "${GK_OWNERS:-CoreServicesUIAgent|UserNotificationCenter}" front 2>/dev/null
  return 0
}

# gz_clear_gk <где> [j] — в кадре главы не должно быть окна «Открыть файл «sosninaalbums»?». Прогон 1 (0.11, macOS
# 15 и 26): окно висело с первого кадра записи — осталось от снимков и мешало Finder, перетаскивание не удавалось.
# Окно есть: до трёх кругов «Готово» → убить агентов → Esc; находка идёт в summary.md предупреждением (с 2-м
# аргументом j — ещё и заметкой в журнал: запись уже в главе). Код 0 — экран чист
gz_clear_gk() {
  local where="$1" i msg
  [ -n "$(gz_gk_window)" ] || return 0
  for i in 1 2 3; do
    dismiss_gk >/dev/null 2>&1
    sleep 1
    kill_gk_agents
    key_code 53
    sleep 1
    if [ -z "$(gz_gk_window)" ]; then
      msg="окно Gatekeeper было на экране ${where} — закрыто (круг ${i})"
      [ "${2:-}" = j ] && gv_note "$msg"
      row "ролик: окно Gatekeeper" "предупреждение" "$msg"
      return 0
    fi
  done
  msg="окно Gatekeeper на экране ${where} не закрылось за три круга — кадр с ним"
  [ "${2:-}" = j ] && gv_note "$msg"
  row "ролик: окно Gatekeeper" "предупреждение" "$msg"
  return 1
}

# gz_copy_app <откуда> <куда> — свежая копия .app; без прав на «Программы» — через sudo
gz_copy_app() {
  rm -rf "$2" 2>/dev/null || sudo -n rm -rf "$2"
  if ! ditto "$1" "$2" 2>>"$OUT/logs/guidevid-err.txt"; then
    sudo -n ditto "$1" "$2" 2>>"$OUT/logs/guidevid-err.txt" && sudo -n chown -R "$(id -un)" "$2"
  fi
}

# gz_wait <сек> <команда…> — ждать, пока команда не вернёт 0
gz_wait() {
  local n="$1" i=0
  shift
  while [ "$i" -lt "$n" ]; do
    "$@" >/dev/null 2>&1 && return 0
    sleep 1
    i=$((i + 1))
  done
  return 1
}

# gv_prepare_copy — перед записью: в classic свежая копия с меткой (как в 0.10); в остальных сценах «Программы»
# пусты, программу туда кладёт перетаскивание. Код 0 — можно записывать
gv_prepare_copy() {
  if [ "$GV_SCENES" = classic ]; then
    gv_install_copy
    [ -d "$VDST" ]
    return
  fi
  rm -rf "$VDST" 2>/dev/null || sudo -n rm -rf "$VDST"
  return 0
}

# gz_sidebar_rect — «x y w h<TAB>надпись» строки «Программы» в боковой панели Finder: доступность, затем текст
# на снимке левее всего в окне. Строка вне боковой панели (одноимённая папка в списке) не годится
gz_sidebar_rect() {
  local win res
  win="$(find_window "$(pids_of Finder)" "" largest)" || return 0
  win="${win#* }"
  res="$(axfind com.apple.finder find "Программы" "Applications")"
  if [ -n "$res" ] && gz_in_sidebar "$(gv_rect_of "$res")" "$win"; then
    printf '%s' "$res"
    return 0
  fi
  ocr_in "$win" --leftmost "Программы" "Applications"
  return 0
}

# act_drag <рамка источника> <рамка цели> <подпись 1> <подпись 2> <запасная команда…> — два стоп-кадра со стрелкой,
# затем живое перетаскивание: курсор на значок, пауза дольше интервала двойного щелчка, dd, толчок в сторону
# (без смещения Finder считает нажатие щелчком), пять шагов dm по 0,7 с (путь виден в ролике), пауза над целью, du.
# Нет любой из рамок — запасной путь (копия без перетаскивания), стрелки у такого кадра нет
act_drag() {
  local src="$1" dst="$2" cap1="$3" cap2="$4" a b
  shift 4
  gv_hold "$(gv_target_rect "$src")" "$cap1"
  gv_hold "$(gv_target_rect "$dst")" "$cap2"
  if a="$(gv_click_point "$src")" && b="$(gv_click_point "$dst")"; then
    gv_cursor_to "$src"
    sleep 0.8
    cliclick "dd:${a% *},${a#* }" 2>>"$OUT/logs/cliclick-err.txt"
    sleep 0.4
    cliclick "dm:$((${a% *} + 6)),$((${a#* } + 6))" 2>>"$OUT/logs/cliclick-err.txt"
    while IFS=' ' read -r px py; do
      sleep 0.7
      cliclick "dm:${px},${py}" 2>>"$OUT/logs/cliclick-err.txt"
    done < <(gz_steps "${a% *}" "${a#* }" "${b% *}" "${b#* }" 5)
    # над боковой панелью Finder подсвечивает цель не сразу
    sleep 0.7
    cliclick "dm:$((${b% *} + 1)),${b#* }" 2>>"$OUT/logs/cliclick-err.txt"
    sleep 0.5
    cliclick "du:${b% *},${b#* }" 2>>"$OUT/logs/cliclick-err.txt"
    gv_jl move t0=@end t1=@now
  else
    GV_IDLE0="$(gv_now)"
    gv_note "запасной путь для «${cap1}»"
    "$@"
    gv_close_idle
  fi
  gv_settle 1.5
}

gz_fb_open_zip() { open "$GZ_ZIP" 2>>"$OUT/logs/open.txt"; }
gz_fb_open_dir() { open "$GZ_DIR" 2>>"$OUT/logs/open.txt"; }
gz_fb_install() { gz_copy_app "$GZ_APP" "$VDST"; }

# ---------- глава ----------

# gz_prepare — в простое: архив в «Загрузках» с меткой, как после Safari; окно Finder
gz_prepare() {
  local zname
  zname="$(basename "$SRC")"
  GZ_ZIP="$GZ_DL/$zname"
  GZ_DIR=""
  GZ_APP=""
  mkdir -p "$GZ_DL"
  rm -rf "$GZ_ZIP" "$GZ_DL/Sosnina Albums"* "$DST"
  cp "$SRC" "$GZ_ZIP"
  quarantine "$GZ_ZIP" || gv_note "метка карантина на архив не записалась"
  gz_finder_zip
  gz_clear_gk "перед главой"
}

# gz_extract_check — после Архивации: папка и .app найдены; метка карантина у распакованного .app — в summary.md
gz_extract_check() {
  local q="нет"
  if [ -n "$GZ_APP" ] && xattr -p com.apple.quarantine "$GZ_APP" >/dev/null 2>&1; then q="да"; fi
  if [ -n "$GZ_APP" ]; then
    row "карантин после Архиватора" "$q" "xattr com.apple.quarantine у распакованного $(basename "$GZ_APP"); архив помечен скриптом, как после Safari"
  fi
}

gv_scene_unzip() {
  local zname stem fname sb name dst
  GV_IDLE0="$(gv_now)"
  gz_prepare
  zname="$(basename "$GZ_ZIP")"
  stem="${zname%.zip}"
  gv_chapter "Распакуйте архив"

  gv_step 1 3 "Распакуйте архив"
  sb="$(finder_item_rect "$stem")"
  shot_t mac-01-zip-downloads "" Finder front "${sb:+$sb$TAB$zname}" "архив в «Загрузках» с меткой карантина"
  act_click "$sb" "Дважды щёлкните по ${zname}" dc gz_fb_open_zip
  if gz_wait 20 gz_extracted "$GZ_DL"; then
    GZ_DIR="$(gz_extracted "$GZ_DL")"
    GZ_APP="$(find_app "$GZ_DIR")"
    row "ролик: распаковка" "да" "папка $(basename "$GZ_DIR"), программа $(basename "$GZ_APP")"
  else
    # архив не по раскладке 0.11 (без папки) или Архиватор не отработал: распаковка в простое, без стрелок
    gv_note "папка с программой не появилась за 20 с — распаковка ditto"
    mkdir -p "$WORK/gz"
    ditto -x -k "$GZ_ZIP" "$WORK/gz" 2>>"$OUT/logs/guidevid-err.txt"
    GZ_APP="$(find_app "$WORK/gz")"
    row "ролик: распаковка" "нет" "Архиватор не дал папку за 20 с; программа взята ditto: ${GZ_APP:-не найдена}"
  fi
  gz_extract_check

  gv_step 2 3 "Откройте папку"
  if [ -n "$GZ_DIR" ]; then
    fname="$(basename "$GZ_DIR")"
    gz_reveal "$GZ_DIR"
    act_click "$(finder_item_rect "$fname")" "Дважды щёлкните по папке «${fname}»" dc gz_fb_open_dir
    sleep 2
    gz_icons
    [ -n "$GZ_APP" ] && gz_reveal "$GZ_APP"
  else
    act_info "Программа распакована"
  fi
  name="$(basename "${GZ_APP:-sosninaalbums.app}" .app)"
  gz_clear_gk "перед снимком папки" j
  sb="$(finder_item_rect "$name")"
  shot_t mac-01-zip-folder "" Finder front "${sb:+$sb$TAB$name}" "внутри папки выпуска: программа, инструкция, «Что нового»"

  gv_step 3 3 "Перенесите в «Программы»"
  if [ -z "$GZ_APP" ]; then
    gv_note "программы после распаковки нет — глава без переноса"
    return 0
  fi
  case "$VDST" in
    /Applications/*)
      dst="$(gz_sidebar_rect)"
      shot_t mac-01-drag-applications "" Finder front "$dst" "цель перетаскивания — «Программы» в боковой панели"
      act_drag "$sb" "$(gv_rect_of "$dst")" "Перетащите ${name} ..." "... в «Программы» слева" gz_fb_install
      gz_wait 10 test -d "$VDST"
      gz_clear_gk "после переноса" j
      ;;
    *)
      gv_note "в «Программы» не записать — копия без перетаскивания"
      gz_fb_install
      act_info "Программа скопирована"
      ;;
  esac
  if [ -d "$VDST" ]; then
    row "ролик: перенос в «Программы»" "да" "$VDST"
  else
    gv_note "перетаскивание не положило программу — копия"
    gz_fb_install
    row "ролик: перенос в «Программы»" "предупреждение" "перетаскивание не удалось, копия ditto: $VDST"
  fi
  # без метки следующие главы не покажут Gatekeeper: ставим, как поставил бы Safari
  if ! xattr -p com.apple.quarantine "$VDST" >/dev/null 2>&1; then
    gv_note "у программы в «Программах» нет метки карантина — поставлена скриптом"
    quarantine "$VDST"
  fi
  act_info "Программа в «Программах»"
}

# Сценарий целиком — под with_timeout (подоболочка): запись запущена снаружи. Время 0 журнала — meta.
# all: «Распакуйте архив» → «Первый запуск» → «Если пишет «повреждено»»; unzip: только первая; classic: как 0.10
gv_scenario() {
  sleep 2
  gv_flash || gv_note "метка синхронизации не поставилась — монтаж откажет"
  if [ "$GV_SCENES" != classic ]; then
    gv_scene_unzip
    [ -d "$VDST" ] || gv_install_copy
  fi
  if [ "$GV_SCENES" != unzip ]; then
    gv_finder_show
    GV_IDLE0="$(gv_now)"
    gv_chapter "Первый запуск на macOS $MAJOR"
    if [ "${MAJOR:-0}" -ge 15 ]; then gv_scene_gatekeeper; else gv_scene_context; fi
    stop_app "$VDST"
    gv_scene_terminal
  fi
  gv_jl end t=@now
  stop_app "$VDST"
}
