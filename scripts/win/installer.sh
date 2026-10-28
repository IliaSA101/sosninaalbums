#!/usr/bin/env bash
# Сборка установщика Windows (спека 111, design §6): makensis (3.08+) на Linux или в Git Bash, с -WX.
#   запуск: installer.sh --exe <exe> --version X.Y.Z --out <папка> [--icon <icon.ico>] [--demo <zip примера> --standard]
# Результат: <папка>/SosninaAlbums-<X.Y.Z>-windows-x64-setup.exe (компактная); с --demo и --standard (только вместе) —
#   …-windows-x64-standard-setup.exe с примером в data\ (0.30-142-44; под маску release-sign «*-windows-x64-setup.exe» не попадает); последняя строка stdout — путь к нему.
# Воспроизводимость: makensis не пишет времени сборки, одни входы дают одни байты (проверяет test_installer.py).
# Пустой exe допустим: ci.yml так проверяет сам скрипт .nsi без сборки программы.
# Папка NSIS: переменная NSISDIR (для makensis, распакованного без установки); по умолчанию — системная.
set -euo pipefail

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
exe='' ver='' out='' demo='' std=0 icon="$root/build/windows/icon.ico"
while [ $# -gt 0 ]; do
  case "$1" in
    --exe) exe=${2:?нет значения у --exe}; shift 2 ;;
    --version) ver=${2:?нет значения у --version}; shift 2 ;;
    --out) out=${2:?нет значения у --out}; shift 2 ;;
    --icon) icon=${2:?нет значения у --icon}; shift 2 ;;
    --demo) demo=${2:?нет значения у --demo}; shift 2 ;;
    --standard) std=1; shift ;;
    *) echo "неизвестный аргумент: $1 (есть --exe --version --out --icon --demo --standard)" >&2; exit 2 ;;
  esac
done
[ -n "$exe" ] && [ -n "$ver" ] && [ -n "$out" ] || { echo 'нужны --exe, --version и --out' >&2; exit 2; }
# редакция задаётся парой: --demo без --standard дал бы «компактный» с примером, --standard без --demo — пустой «стандартный»
{ [ -n "$demo" ] && [ "$std" = 1 ]; } || { [ -z "$demo" ] && [ "$std" = 0 ]; } || { echo '--demo и --standard — только вместе' >&2; exit 2; }
# VIProductVersion требует четыре числа: версия строго X.Y.Z
printf '%s' "$ver" | grep -Eq '^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}$' || { echo "версия «$ver» — ждали X.Y.Z" >&2; exit 2; }
[ -f "$exe" ] || { echo "нет файла exe: $exe" >&2; exit 1; }
[ -f "$root/scripts/win/wizard.bmp" ] || { echo "нет картинки мастера scripts/win/wizard.bmp (make_wizard_bmp.py)" >&2; exit 1; }
for dpi in 100 125 150 175; do
  [ -f "$root/scripts/win/wizard-$dpi.bmp" ] || { echo "нет картинки мастера scripts/win/wizard-$dpi.bmp (make_wizard_bmp.py)" >&2; exit 1; }
done
[ -f "$root/scripts/win/header.bmp" ] || { echo "нет картинки шапки scripts/win/header.bmp (make_wizard_bmp.py)" >&2; exit 1; }
[ -z "$demo" ] || [ -f "$demo" ] || { echo "нет zip примера: $demo" >&2; exit 1; }
[ -f "$icon" ] || { echo "нет значка: $icon" >&2; exit 1; }

command -v makensis >/dev/null || { echo 'makensis не найден (Ubuntu: sudo apt install nsis)' >&2; exit 1; }
# makensis -VERSION печатает «v3.09» или «v3.09-4»; нужно не старше 3.08 (синтаксис ${GetOptions}, Unicode)
have=$(makensis -VERSION | sed -n 's/^v\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')
[ -n "$have" ] || { echo 'не разобрана версия makensis' >&2; exit 1; }
if [ "$(printf '%s\n3.08\n' "$have" | sort -V | head -n 1)" != 3.08 ]; then
  echo "makensis $have старше 3.08" >&2
  exit 1
fi

mkdir -p "$out"
out_abs=$(CDPATH='' cd -- "$out" && pwd)
suffix=setup
[ -z "$demo" ] || suffix=standard-setup
setup="$out_abs/SosninaAlbums-$ver-windows-x64-$suffix.exe"
abs() { printf '%s/%s' "$(CDPATH='' cd -- "$(dirname -- "$1")" && pwd)" "$(basename -- "$1")"; }

demo_def=()
[ -z "$demo" ] || demo_def=("-DDEMO_SRC=$(abs "$demo")")
# -V2: только предупреждения и ошибки; -WX: предупреждение = провал
makensis -WX -V2 "-DVERSION=$ver" "-DEXE_SRC=$(abs "$exe")" "-DICON=$(abs "$icon")" "-DWIZARD=$root/scripts/win/wizard.bmp" "-DWIZARD_DPI=$root/scripts/win" "-DHEADER=$root/scripts/win/header.bmp" "-DOUT=$setup" ${demo_def[@]+"${demo_def[@]}"} "$root/scripts/win/installer.nsi" >&2
[ -s "$setup" ] || { echo "установщик не создан: $setup" >&2; exit 1; }
echo "$setup"
