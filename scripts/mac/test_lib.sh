#!/bin/bash
# shellcheck source-path=SCRIPTDIR
# Тесты чистых функций lib.sh: bash scripts/mac/test_lib.sh (в mac-shots.yml — /bin/bash 3.2 macOS)
set -u
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"
# shellcheck source=display.sh
. "$MAC_DIR/display.sh"
# shellcheck source=video.sh
. "$MAC_DIR/video.sh"

fails=0
eq() {
  if [ "$2" = "$3" ]; then return 0; fi
  printf 'ПРОВАЛ %s: ждали [%s], получили [%s]\n' "$1" "$3" "$2"
  fails=$((fails + 1))
}
ok() { if "${@:2}"; then return 0; fi; echo "ПРОВАЛ $1: ждали успех"; fails=$((fails + 1)); }
no() { if ! "${@:2}"; then return 0; fi; echo "ПРОВАЛ $1: ждали отказ"; fails=$((fails + 1)); }

ok "11.0 ≤ 12.0" ver_le 11.0 12.0
ok "10.13 ≤ 12.0" ver_le 10.13 12.0
ok "12 ≤ 12.0" ver_le 12 12.0
ok "12.0.0 ≤ 12.0" ver_le 12.0.0 12.0
no "12.1 > 12.0" ver_le 12.1 12.0
no "13.0 > 12.0" ver_le 13.0 12.0
no "10.13 > 10.9" ver_le 10.13 10.9

ok "universal x86_64 arm64" is_universal "x86_64 arm64"
ok "universal arm64 x86_64" is_universal "arm64 x86_64"
ok "universal с переводом строки" is_universal "$(printf 'arm64\nx86_64\n')"
no "только arm64" is_universal "arm64"
no "только x86_64" is_universal "x86_64"
no "arm64e не arm64" is_universal "x86_64 arm64e"
no "пусто" is_universal ""

build='Load command 10
      cmd LC_BUILD_VERSION
  cmdsize 32
 platform MACOS
    minos 11.0
      sdk 15.2'
eq "minos из LC_BUILD_VERSION" "$(printf '%s\n' "$build" | parse_minos)" "11.0"
old='Load command 9
      cmd LC_VERSION_MIN_MACOSX
  cmdsize 16
  version 10.13
      sdk 10.15
Load command 10
      cmd LC_SOURCE_VERSION
  version 0.0'
eq "minos из LC_VERSION_MIN_MACOSX" "$(printf '%s\n' "$old" | parse_minos)" "10.13"
eq "version чужой команды не берём" "$(printf 'cmd LC_SOURCE_VERSION\nversion 0.0\n' | parse_minos)" ""

eq "версия из имени zip" "$(version_from_zip /tmp/x/SosninaAlbums-0.9.1-macos-universal.zip)" "0.9.1"
eq "чужое имя zip" "$(version_from_zip /tmp/macos.zip)" ""

eq "ячейка без | и переводов" "$(md_cell "$(printf 'a|b\nc')")" "a/b c"
eq "метка карантина" "$(quarantine_value 255 ABC)" "0083;ff;Safari;ABC"

eq "TCC 13 столбцов" "$(tcc_values 13 kTCCServiceScreenCapture /usr/sbin/screencapture 1 100)" \
  "'kTCCServiceScreenCapture','/usr/sbin/screencapture',1,2,4,1,NULL,NULL,0,'UNUSED',NULL,0,100"
eq "TCC 17 столбцов" "$(tcc_values 17 S C 1 100)" "'S','C',1,2,4,1,NULL,NULL,0,'UNUSED',NULL,0,100,NULL,NULL,'UNUSED',100"
no "TCC чужая схема" tcc_values 15 S C 1 100

devs='[AVFoundation indev @ 0x1] AVFoundation video devices:
[AVFoundation indev @ 0x1] [0] FaceTime HD Camera
[AVFoundation indev @ 0x1] [3] Capture screen 0
[AVFoundation indev @ 0x1] AVFoundation audio devices:'
eq "индекс экрана avfoundation" "$(printf '%s\n' "$devs" | avf_screen_index)" "3"
eq "нет экрана avfoundation" "$(printf 'x\n' | avf_screen_index)" ""

eq "частота" "$(fps_of 150 40)" "3.75"
eq "частота за 0 с" "$(fps_of 5 0)" "0"

v="$(video_verdict 42 15 0.4 12)"
eq "видео годно" "${v%%"$TAB"*}" "да"
v="$(video_verdict 42 3.2 0.4 25)"
eq "видео: мало кадров" "${v%%"$TAB"*}" "нет"
case "$v" in *"5 кадр"*"20 МБ"*) ;; *) eq "видео: причины" "$v" "…5 кадр…20 МБ…" ;; esac
v="$(video_verdict 42 15 0.05 5)"
eq "видео застыло" "${v%%"$TAB"*}" "нет"
v="$(video_verdict 42 15 "" 5)"
eq "движение не измерено — не провал" "${v%%"$TAB"*}" "да"
v="$(video_verdict 242 14.7 0.099 20 24)"
eq "прогон 3: доля 0,099, но 24 смены кадра — не застыло" "${v%%"$TAB"*}" "да"
v="$(video_verdict 42 15 0.05 5 3)"
eq "три смены кадра — застыло" "${v%%"$TAB"*}" "нет"
v="$(video_verdict 8 15 0.5 1 7 5)"
eq "проба 8 с при минимуме 5 с — годна" "${v%%"$TAB"*}" "да"
v="$(video_verdict 3 15 0.5 1 7 5)"
eq "проба 3 с при минимуме 5 с — коротка" "${v%%"$TAB"*}" "нет"
case "$v" in *"короче 5 с"*) ;; *) eq "причина: короче 5 с" "$v" "…короче 5 с…" ;; esac
v="$(video_verdict 42 15 0.05 5 0)"
eq "ноль смен кадра — застыло" "${v%%"$TAB"*}" "нет"

eq "экран: auto" "$(display_candidates auto)" "1440x900:hidpi 1920x1080 1680x1050 1440x900"
eq "экран: пусто = auto" "$(display_candidates "")" "1440x900:hidpi 1920x1080 1680x1050 1440x900"
eq "экран: свой список" "$(display_candidates "1920x1080, 1280x720:hidpi")" "1920x1080 1280x720:hidpi"
eq "экран: ошибка в списке = auto" "$(display_candidates "1920x1080 большой")" "1440x900:hidpi 1920x1080 1680x1050 1440x900"
eq "экран: ш в режим hidpi" "$(display_wh 1440x900:hidpi)" "1440 900 hidpi"
eq "экран: ш в режим обычный" "$(display_wh 1920x1080)" "1920 1080 any"
ok "экран: режим совпал" display_matches "1920 1080 1920 1080 60 -" 1920 1080 any
ok "экран: hidpi совпал" display_matches "1440 900 2880 1800 60 hidpi" 1440 900 hidpi
no "экран: просили hidpi, стал обычный" display_matches "1440 900 1440 900 60 -" 1440 900 hidpi
no "экран: другой размер" display_matches "1024 768 1024 768 60 -" 1920 1080 any
no "экран: пустой вывод" display_matches "" 1920 1080 any

# guidevid.sh (036): чистые помощники ролика
# shellcheck source=guidevid.sh
. "$MAC_DIR/guidevid.sh"
eq "рамка из «рамка<TAB>подпись»" "$(gv_rect_of "10 20 30 40${TAB}Готово")" "10 20 30 40"
eq "рамка без подписи" "$(gv_rect_of "10 20 30 40")" "10 20 30 40"
eq "рамка пустой вход" "$(gv_rect_of "")" ""
eq "центр рамки" "$(gv_centre "612 388 96 110")" "660 443"
eq "центр рамки с дробными" "$(gv_centre "10.5 20 31 40")" "26 40"
no "центр: не четыре числа" gv_centre "1 2 3"
no "центр: пусто" gv_centre ""
# Spotlight: сверху поле ввода с тем же текстом, ниже — пункт; третья строка (предпросмотр) не нужна
rows="$(printf '420 80 140 30\tТерминал\n420 170 90 24\tТерминал\n900 175 120 20\tТерминал — Утилиты\n')"
eq "Spotlight: вторая сверху" "$(printf '%s\n' "$rows" | gv_second_row)" "$(printf '420 170 90 24\tТерминал')"
eq "Spotlight: только поле ввода — пусто" "$(printf '420 80 140 30\tТерминал\n' | gv_second_row)" ""
eq "Spotlight: строка вплотную к полю — не пункт" "$(printf '420 80 140 30\tТерминал\n420 90 90 24\tТерминал\n' | gv_second_row)" ""
eq "Spotlight: порядок на входе не важен" "$(printf '900 175 120 20\tТерминал — Утилиты\n420 80 140 30\tТерминал\n' | gv_second_row)" "$(printf '900 175 120 20\tТерминал — Утилиты')"
eq "Spotlight: пустой вход" "$(printf '' | gv_second_row)" ""

# Рамка цели для стрелки (036): точка — квадрат вокруг неё; щелчок — центр рамки или сама точка
eq "цель: рамка как есть" "$(gv_target_rect "612 388 96 110")" "612 388 96 110"
eq "цель: точка — квадрат 40×40 вокруг" "$(gv_target_rect "700 400")" "680 380 40 40"
no "цель: три числа" gv_target_rect "1 2 3"
no "цель: пусто" gv_target_rect ""
eq "щелчок: центр рамки" "$(gv_click_point "612 388 96 110")" "660 443"
eq "щелчок: сама точка" "$(gv_click_point "700 400")" "700 400"
no "щелчок: пусто" gv_click_point ""
# поле Spotlight: «id x y w h» окна; узкое окно — значок в строке меню, не поле
eq "поле Spotlight: верх окна не выше 64" "$(gv_field_rect "81 480 200 960 540")" "480 200 960 64"
eq "поле Spotlight: низкое окно как есть" "$(gv_field_rect "81 480 200 960 50")" "480 200 960 50"
no "поле Spotlight: значок меню 31 pt" gv_field_rect "22 1717 0 31 24"
no "поле Spotlight: окна нет" gv_field_rect ""
# повторы искалки цели: пусто, пусто, найдено — берём третий ответ; всегда пусто — код 1
GV_RETRY_PAUSE=0
n_try="$(mktemp)"
gv_flaky() { echo x >>"$n_try"; [ "$(wc -l <"$n_try")" -ge 3 ] && printf '1 2 3 4'; return 0; }
eq "повторы: найдено на третьей" "$(gv_find_retry 3 gv_flaky; echo "/$?")" "1 2 3 4/0"
rm -f "$n_try"
gv_never() { return 0; }
no "повторы: так и не нашлось" gv_find_retry 3 gv_never
eq "повторы: пусто без вывода" "$(gv_find_retry 2 gv_never)" ""

# find_app (039): .app в корне (выпуск 0.10) и в папке выпуска (0.11); ноль и несколько — отказ
# shellcheck source=gvzip.sh
. "$MAC_DIR/gvzip.sh"
FA="$(mktemp -d)"
mkdir -p "$FA/root/x.app/Contents" "$FA/nested/Sosnina Albums 0.11.0/sosninaalbums.app/Contents" \
  "$FA/empty" "$FA/two/a.app" "$FA/two/b.app" "$FA/mixed/a.app" "$FA/mixed/dir/b.app" "$FA/mac/__MACOSX/c.app" \
  "$FA/mac/Sosnina Albums 0.11.0/sosninaalbums.app"
eq "find_app: .app в корне" "$(find_app "$FA/root")" "$FA/root/x.app"
eq "find_app: .app в папке выпуска" "$(find_app "$FA/nested")" "$FA/nested/Sosnina Albums 0.11.0/sosninaalbums.app"
eq "find_app: __MACOSX не считается" "$(find_app "$FA/mac")" "$FA/mac/Sosnina Albums 0.11.0/sosninaalbums.app"
no "find_app: пустая папка" find_app "$FA/empty"
no "find_app: нет такой папки" find_app "$FA/нет"
no "find_app: два в корне" find_app "$FA/two"
no "find_app: корень и вложенная" find_app "$FA/mixed"
eq "find_app: пусто — ничего не печатает" "$(find_app "$FA/empty")" ""
# gz_extracted: папка первого уровня «Загрузок» с программой; сама .app в «Загрузках» (копия шагов снимков) не считается
mkdir -p "$FA/dl-bare/sosninaalbums.app" "$FA/dl/Sosnina Albums 0.11.0/sosninaalbums.app" "$FA/dl/sosninaalbums.app" "$FA/dl/пусто"
eq "gz_extracted: папка выпуска" "$(gz_extracted "$FA/dl")" "$FA/dl/Sosnina Albums 0.11.0"
no "gz_extracted: архив без папки" gz_extracted "$FA/dl-bare"
no "gz_extracted: нет Загрузок" gz_extracted "$FA/нет"
rm -rf "$FA"
# какие главы снимать: у .app архива нет — classic; мусор — all
eq "scenes: all по умолчанию" "$(gv_scenes_norm all /x/a.zip)" "all"
eq "scenes: unzip" "$(gv_scenes_norm unzip /x/a.zip)" "unzip"
eq "scenes: classic" "$(gv_scenes_norm classic /x/a.zip)" "classic"
eq "scenes: мусор — all" "$(gv_scenes_norm "" /x/a.zip)" "all"
eq "scenes: незнакомое — all" "$(gv_scenes_norm разное /x/a.zip)" "all"
eq "scenes: .app — всегда classic" "$(gv_scenes_norm all /x/sosninaalbums.app)" "classic"
eq "scenes: пустой источник — classic" "$(gv_scenes_norm unzip "")" "classic"
# строка «Программы» — только в боковой панели (левая треть окна Finder 100 200 900 600)
ok "боковая панель: строка у левого края" gz_in_sidebar "110 300 120 24" "100 200 900 600"
ok "боковая панель: граница трети" gz_in_sidebar "250 300 100 24" "100 200 900 600"
no "боковая панель: папка в списке справа" gz_in_sidebar "600 300 120 24" "100 200 900 600"
no "боковая панель: цель левее окна" gz_in_sidebar "10 300 40 24" "100 200 900 600"
no "боковая панель: не четыре числа" gz_in_sidebar "110 300" "100 200 900 600"
no "боковая панель: нет окна" gz_in_sidebar "110 300 120 24" ""
# путь перетаскивания: пять точек от источника к цели, последняя — цель
eq "перетаскивание: пять шагов" "$(gz_steps 100 200 300 100 5 | tr '\n' '|')" "140 180|180 160|220 140|260 120|300 100|"
eq "перетаскивание: источник и цель в одной точке" "$(gz_steps 50 50 50 50 2 | tr '\n' '|')" "50 50|50 50|"
# gv_prepare_copy: classic делает копию (ditto подменён), остальные сцены только очищают «Программы»
PC="$(mktemp -d)"
OUT="$PC/out"
mkdir -p "$OUT/logs" "$PC/apps/old.app" "$PC/src.app"
VDST="$PC/apps/old.app"
APP="$PC/src.app"
quarantine() { :; }
ditto() { cp -R "$1" "$2"; }
GV_SCENES=unzip
ok "prepare_copy: unzip — код 0" gv_prepare_copy
eq "prepare_copy: unzip — «Программы» очищены" "$(test -d "$VDST" && echo есть || echo нет)" "нет"
mkdir -p "$VDST"
GV_SCENES=all
gv_prepare_copy
eq "prepare_copy: all — копия не создаётся" "$(test -d "$VDST" && echo есть || echo нет)" "нет"
GV_SCENES=classic
gv_prepare_copy
eq "prepare_copy: classic — копия сделана" "$(test -d "$VDST" && echo есть || echo нет)" "есть"
rm -rf "$VDST"
ditto() { return 1; }
sudo() { return 1; }
no "prepare_copy: classic — копия не создалась, код 1" gv_prepare_copy
unset -f ditto sudo quarantine
rm -rf "$PC"
GV_SCENES=all

# Журнал ролика: note внутри простоя и запасной путь act_click (036). Нужны python и macshots.py; время настоящее
OUT="$(mktemp -d)"
mkdir -p "$OUT/logs"
PY="$(command -v python3 >/dev/null 2>&1 && python3 -c 1 >/dev/null 2>&1 && echo python3 || echo python)"
GV_JOURNAL="$OUT/steps.jsonl"
GV_CLOCK="$OUT/clock"
gv_jl meta os=macos ver=15.0 lang=ru recorder=avfoundation
# «событие t0 t1» по строкам журнала без meta
jsum() {
  "$PY" -I -c 'import json,sys
for l in open(sys.argv[1], encoding="utf-8").read().splitlines()[1:]:
    o = json.loads(l); print(o["ev"], o.get("t0", o.get("t")), o.get("t1", ""))' "$GV_JOURNAL"
}
GV_IDLE0="$(gv_now)"
sleep 0.6
gv_note "заметка внутри простоя" >/dev/null 2>&1
sleep 0.6
gv_flush
rows="$(jsum)"
eq "note в простое: idle, note, idle" "$(printf '%s\n' "$rows" | cut -d' ' -f1 | tr '\n' ' ')" "idle note idle "
eq "note в простое: первый idle не срезан" "$(printf '%s\n' "$rows" | awk 'NR == 1 { e = $3; d = $3 - $2 } NR == 2 { n = $2 } END { print (e <= n && d >= 0.4) ? "ok" : "срезан" }')" "ok"
eq "note в простое: второй idle с момента note" "$(printf '%s\n' "$rows" | awk 'NR == 2 { n = $2 } NR == 3 { print ($2 >= n) ? "ok" : "сдвинут" }')" "ok"
# запасной путь без цели — простой с начала запасного пути
gv_fb_slow() { sleep 1.5; }
act_click "" "Нажмите" c gv_fb_slow >/dev/null 2>&1
gv_flush
rows="$(jsum | tail -n +4)"
eq "act_click: запасной путь — есть hold и note" "$(printf '%s\n' "$rows" | cut -d' ' -f1 | grep -c -E '^(hold|note)$')" "2"
eq "act_click: простоя хватает на запасной путь" "$(printf '%s\n' "$rows" | awk '$1 == "idle" { s += $3 - $2 } END { print (s >= 1.4) ? "ok" : "короткий" }')" "ok"
# hold перед щелчком несёт рамку цели, а щелчок идёт в её центр / в точку (036: без рамки нет стрелки в ролике)
cliclick() { printf '%s\n' "$*" >>"$OUT/clicks"; }
jholds() {
  "$PY" -I -c 'import json,sys
for l in open(sys.argv[1], encoding="utf-8").read().splitlines()[1:]:
    o = json.loads(l)
    if o["ev"] == "hold": print(" ".join(map(str, o["rect"])) if o["rect"] else "-")' "$GV_JOURNAL" | tr -d '\r'
}
GV_IDLE0=""
gv_jl meta os=macos ver=15.0 lang=ru recorder=avfoundation
: >"$OUT/clicks"
act_click "612 388 96 110" "Дважды щёлкните по значку" dc >/dev/null 2>&1
act_click "700 400" "Нажмите точку" c >/dev/null 2>&1
act_click "" "Нет цели" c >/dev/null 2>&1
gv_flush
eq "act_click: рамка в hold — как найдена, точка — квадратом, нет цели — без рамки" "$(jholds | tr '\n' '|')" "612 388 96 110|680 380 40 40|-|"
eq "act_click: щелчки — центр рамки и сама точка, без цели щелчка нет" "$(grep -E '^(dc|c):' "$OUT/clicks" | tr '\n' ' ')" "dc:660,443 c:700,400 "
unset -f cliclick
rm -rf "$OUT"

# bash 3.2 под set -u склеивает имя переменной с соседним не-ASCII символом («cands…: unbound
# variable», прогон 38524239739, строка с «ни один из»): рядом с не-ASCII пишем ${имя}. Байты ≥ 0x80 ловит локаль C
bad="$(cd "$MAC_DIR" && LC_ALL=C grep -nE '\$[A-Za-z_][A-Za-z0-9_]*[^ -~[:space:]]' ./*.sh)"
eq "нет \$имя вплотную к не-ASCII в scripts/mac/*.sh" "$bad" ""

# Сбой display.swift (не собрался) не валит шаг: код 0, строка «нет» в summary.md, снимки на текущем экране
OUT="$(mktemp -d)"
mkdir -p "$OUT/logs"
display_run() { echo "error: cannot convert value"; return 1; }
set_display auto >/dev/null 2>&1
eq "экран: сбой display.swift — код 0" "$?" "0"
case "$(cat "$OUT/summary.md")" in *"экран: режим | нет |"*"текущем экране"*) ;; *) eq "экран: сбой попал в summary.md" "$(cat "$OUT/summary.md")" "…нет…текущем экране…" ;; esac
rm -rf "$OUT"

# --no-install (выпуск): brew и pip не зовутся, нужного инструмента нет — отказ, а не установка
ok "установки: по умолчанию разрешены" install_allowed
MAC_NO_INSTALL=1
no "установки: MAC_NO_INSTALL=1 запрещает" install_allowed
OUT="$(mktemp -d)"
mkdir -p "$OUT/logs"
brew() { echo "brew $*" >>"$OUT/brewed"; }
with_timeout() { shift; "$@"; }
PATH_SAVE="$PATH"
PATH="$OUT/nobin"
no "установки: ffmpeg не ставится" ensure_ffmpeg
no "установки: displayplacer не ставится" displayplacer_try "1920x1080"
PATH="$PATH_SAVE"
eq "установки: brew не вызывался" "$(cat "$OUT/brewed" 2>/dev/null)" ""
unset -f brew with_timeout
MAC_NO_INSTALL=0
ok "установки: MAC_NO_INSTALL=0 разрешает" install_allowed
rm -rf "$OUT"

if [ "$fails" -gt 0 ]; then
  echo "test_lib.sh: провалов $fails"
  exit 1
fi
echo "test_lib.sh: OK (bash $BASH_VERSION)"
