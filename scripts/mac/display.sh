# shellcheck shell=bash
# Разрешение экрана раннера (029, прогон 4): снимки 1024×768 мелкие. Подключается из shots.sh.
# Порядок проб: родной display.swift (CoreGraphics: все режимы, выставить), затем displayplacer из brew.
# Что выбрано и что есть у раннера — logs/display-modes.txt, logs/display.txt; итог — строка «экран» в summary.md.
# Функции зовутся из shots.sh по имени
# shellcheck disable=SC2329

# Порядок «лучше → хуже»: Retina 1440×900 даёт снимок 2880×1800, дальше — обычные режимы по убыванию
DISPLAY_AUTO="1440x900:hidpi 1920x1080 1680x1050 1440x900"

# display_candidates <auto|«WxH[:hidpi] …»> — список проб; ошибочный ввод — как auto
display_candidates() {
  local spec="${1:-auto}" t out=""
  case "$spec" in
    auto | "")
      printf '%s' "$DISPLAY_AUTO"
      return 0
      ;;
  esac
  # shellcheck disable=SC2086 # список — отдельными словами (разделитель — пробел или запятая)
  for t in $(printf '%s' "$spec" | tr ',' ' '); do
    if printf '%s' "$t" | grep -Eq '^[0-9]+x[0-9]+(:hidpi)?$'; then
      out="${out:+$out }$t"
    else
      printf '%s' "$DISPLAY_AUTO"
      return 0
    fi
  done
  printf '%s' "${out:-$DISPLAY_AUTO}"
}

# display_wh <кандидат> — «ширина высота режим» (hidpi|any)
display_wh() {
  local c="$1" w h m=any
  w="${c%%x*}"
  h="${c#*x}"
  h="${h%%:*}"
  case "$c" in *:hidpi) m=hidpi ;; esac
  printf '%s %s %s' "$w" "$h" "$m"
}

# display_run <аргументы display.swift> — собранный один раз display, иначе swift-скриптом
display_run() {
  local b="$WORK/display"
  if [ ! -x "$b" ]; then
    swiftc -O "$MAC_DIR/display.swift" -o "$b" >"$OUT/logs/swiftc-display.txt" 2>&1 || b=""
  fi
  if [ -n "$b" ]; then "$b" "$@" 2>&1; else swift "$MAC_DIR/display.swift" "$@" 2>&1; fi
}

# display_matches <«ш в пикс-ш пикс-в Гц hidpi»> <ш> <в> <режим> — текущий режим тот, что просили
display_matches() {
  # shellcheck disable=SC2086 # шесть слов вывода display cur
  set -- $1 "$2" "$3" "$4"
  [ "$#" -ge 9 ] || return 1
  [ "$1" = "$7" ] && [ "$2" = "$8" ] || return 1
  [ "$9" != hidpi ] || [ "$6" = hidpi ]
}

# displayplacer_try <кандидаты> — запасной путь: displayplacer из brew; печатает «ок <кандидат>» или ничего
displayplacer_try() {
  local id c w h m sc
  if ! command -v displayplacer >/dev/null 2>&1; then
    with_timeout 240 brew install jakehilborn/jakehilborn/displayplacer >>"$OUT/logs/brew.txt" 2>&1
  fi
  command -v displayplacer >/dev/null 2>&1 || return 1
  displayplacer list >"$OUT/logs/displayplacer-list.txt" 2>&1
  id="$(awk '/Persistent screen id:/ { print $4; exit }' "$OUT/logs/displayplacer-list.txt")"
  [ -n "$id" ] || return 1
  for c in $1; do
    # shellcheck disable=SC2046 # «ш в режим» → три слова
    set -- $(display_wh "$c")
    w="$1"
    h="$2"
    m="$3"
    sc=off
    [ "$m" = hidpi ] && sc=on
    if displayplacer "id:$id res:${w}x${h} hz:60 color_depth:8 scaling:$sc" >>"$OUT/logs/display.txt" 2>&1; then
      sleep 3
      if display_matches "$(display_run cur)" "$w" "$h" "$m"; then
        printf 'ок %s (displayplacer)' "$c"
        return 0
      fi
    fi
  done
  return 1
}

# set_display <auto|список> — выставить лучший из доступных режимов; всегда код 0 (неудача — строка в отчёте)
set_display() {
  local spec="${1:-auto}" cands c w h m before got="" via=""
  cands="$(display_candidates "$spec")"
  display_run list >"$OUT/logs/display-modes.txt"
  before="$(display_run cur)"
  note "экран до: $before; кандидаты: $cands; режимов у раннера: $(wc -l <"$OUT/logs/display-modes.txt" | tr -d ' ')"
  for c in $cands; do
    # shellcheck disable=SC2046 # «ш в режим» → три слова
    set -- $(display_wh "$c")
    w="$1"
    h="$2"
    m="$3"
    {
      echo "== $c"
      display_run set "$w" "$h" "$m"
    } >>"$OUT/logs/display.txt"
    sleep 3
    if display_matches "$(display_run cur)" "$w" "$h" "$m"; then
      got="$c"
      via="CoreGraphics"
      break
    fi
  done
  if [ -z "$got" ]; then
    got="$(displayplacer_try "$cands")" && via="displayplacer"
  fi
  if [ -n "$got" ]; then
    row "экран: режим" "да" "$got через $via; было: $before; стало: $(display_run cur)"
  else
    row "экран: режим" "нет" "ни один из «$cands» не выставился (logs/display-modes.txt — что есть у раннера, logs/display.txt); остался: $before"
  fi
  sleep 2
  return 0
}

# report_screen — фактическое разрешение по снимку 01-desktop и списку окон (для отчёта прогона)
report_screen() {
  local line
  line="$("$PY" "$MAC_DIR/macshots.py" screen --windows "$OUT/logs/windows-01-desktop.json" \
    --png "$OUT/png/01-desktop.png" 2>/dev/null)"
  row "экран: фактический" "да" "${line:-не определён}"
}
