#!/bin/bash
# shellcheck source-path=SCRIPTDIR
# Тесты чистых функций lib.sh: bash scripts/mac/test_lib.sh (в mac-shots.yml — /bin/bash 3.2 macOS)
set -u
# shellcheck source=lib.sh
. "$(cd "$(dirname "$0")" && pwd)/lib.sh"

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

if [ "$fails" -gt 0 ]; then
  echo "test_lib.sh: провалов $fails"
  exit 1
fi
echo "test_lib.sh: OK (bash $BASH_VERSION)"
