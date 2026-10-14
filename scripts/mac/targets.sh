# shellcheck shell=bash
# Куда нажимать на снимке (029, прогон 4): рамки целей для стрелок в инструкции. Каждая функция печатает
# «x y w h<TAB>подпись» (точки экрана, верх-лево) или ничего; shots.sh кладёт её в shots.json (target.rect),
# place_run.py переводит в .ann.json. Подключается из shots.sh; GK_OWNERS, OPEN_ANYWAY, GK_DONE,
# settings_pids, terminal_pids — оттуда же.
# Функции зовутся из shots.sh по имени
# shellcheck disable=SC2329

# «Готово» без «OK»/«Отмена»: при распознавании по тексту «ок» нашлось бы в десятке слов
GK_DONE_OCR=("Done" "Готово")
APPLE_SETTINGS=("System Settings" "Системные настройки" "System Preferences" "Системные параметры")
PRIVACY_LABELS=("Конфиденциальность и безопасность" "Privacy & Security" "Защита и безопасность" "Security & Privacy")

# win_rect <pid-ы> <владелец> <front|largest> — «x y w h» окна
win_rect() {
  local line
  line="$(find_window "$1" "$2" "$3")" || return 1
  printf '%s' "${line#* }"
}

# gk_button_rect — «Готово» в окне Gatekeeper: доступность, затем текст на снимке внутри окна
gk_button_rect() {
  local b res rect
  for b in com.apple.coreservices.uiagent com.apple.UserNotificationCenter; do
    res="$(axfind "$b" find "${GK_DONE[@]}")"
    if [ -n "$res" ]; then
      printf '%s' "$res"
      return 0
    fi
  done
  rect="$(win_rect "$(gk_pids)" "$GK_OWNERS" front)" || return 0
  ocr_in "$rect" "${GK_DONE_OCR[@]}"
  return 0
}

# apple_menu_rect — пункт «Системные настройки…» в открытом меню Apple (меню — в левом верхнем углу)
apple_menu_rect() {
  ocr_in "0 0 700 700" "${APPLE_SETTINGS[@]}"
  return 0
}

# privacy_rect — «Конфиденциальность и безопасность» в окне «Настроек»: строка боковой колонки (левее),
# иначе заголовок страницы
privacy_rect() {
  local rect
  rect="$(win_rect "$(settings_pids)" "" largest)" || return 0
  ocr_in "$rect" --leftmost "${PRIVACY_LABELS[@]}"
  return 0
}

# confirm_button_rect <pid-ы> — «Все равно открыть» внутри окна подтверждения, не в «Настройках» за ним
confirm_button_rect() {
  local rect
  rect="$(win_rect "$1" "SecurityAgent|$GK_OWNERS" front)" || return 0
  ocr_in "$rect" "${OPEN_ANYWAY[@]}"
  return 0
}

# open_button_rect — кнопка «Открыть» в окне после правой кнопки → «Открыть» (macOS 12–14)
open_button_rect() {
  local rect
  rect="$(win_rect "$(gk_pids)" "$GK_OWNERS" front)" || return 0
  ocr_in "$rect" "Open" "Открыть"
  return 0
}

# terminal_cmd_rect — строка с командой xattr в окне «Терминала»
terminal_cmd_rect() {
  local rect
  rect="$(win_rect "$(terminal_pids)" "" largest)" || return 0
  ocr_in "$rect" "xattr -dr"
  return 0
}

# shot_t <имя> <pid-ы> <владелец> <front|largest> <«x y w h<TAB>подпись» или пусто> <почему> — снимок с целью;
# цель не найдена — снимок есть, но стрелки не будет (так и пишем в отчёт)
shot_t() {
  local res="$5"
  if [ -n "$res" ]; then
    shot "$1" "$2" "$3" "$4" "${res#*"$TAB"}" "${res%%"$TAB"*}" -- "$6"
  else
    shot "$1" "$2" "$3" "$4" -- "${6:+$6; }цель (куда нажимать) не найдена — стрелки не будет"
  fi
}
