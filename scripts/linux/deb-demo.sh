#!/usr/bin/env bash
# Пакет данных «пример набора» (спека 142, 0.30-142-47): zip примера кладётся в /usr/share/sosninaalbums/, где его
# находит программа. Не зависит от основного .deb: ставится рядом (Enhances), удаляется вместе с ним.
#   запуск: deb-demo.sh --out <папка> [--zip <zip примера>] [--date <ISO-время>]  # lang-ok
# Результат: <D>/SosninaAlbums-demo-<N>-linux-all.deb; последняя строка stdout — путь к нему. N — число в имени zip
# (wedding-navy-watercolor-demo-<N>.zip): версия пакета меняется только вместе с примером, apt его не переставляет.
# Воспроизводимость: --date постоянная — дата коммита zip (git log -1 --format=%cI -- internal/demopack/*.zip), а не
# метки выпуска: иначе байты пакета разные на каждой странице выпуска. Без --date берётся она же и нужна полная история.
set -euo pipefail

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
zip='' out='' date=''
while [ $# -gt 0 ]; do
  case "$1" in
    --zip) zip=${2:?нет значения у --zip}; shift 2 ;;
    --out) out=${2:?нет значения у --out}; shift 2 ;;
    --date) date=${2:?нет значения у --date}; shift 2 ;;
    *) echo "неизвестный аргумент: $1 (есть --zip --out --date)" >&2; exit 2 ;;
  esac
done
[ -n "$out" ] || { echo "использование: deb-demo.sh --out D [--zip Z] [--date ISO]" >&2; exit 2; }
if [ -z "$zip" ]; then
  # Единственный zip примера в репозитории; два сразу — неоднозначность, а не «берём любой»
  shopt -s nullglob
  zips=("$root"/internal/demopack/*.zip)
  [ "${#zips[@]}" -eq 1 ] || { echo "в internal/demopack ждали один zip, найдено: ${#zips[@]}" >&2; exit 1; }
  zip=${zips[0]}
fi
[ -s "$zip" ] || { echo "нет zip примера: $zip" >&2; exit 1; }
name=$(basename "$zip")
[[ "$name" =~ ^[a-z0-9-]+-demo-([1-9][0-9]*)\.zip$ ]] || { echo "имя zip не вида <набор>-demo-<N>.zip: $name" >&2; exit 2; }
ver=${BASH_REMATCH[1]}

if [ -z "$date" ]; then
  # В неполной копии git log вернул бы дату единственного коммита — молча менять байты пакета нельзя
  [ "$(git -C "$root" rev-parse --is-shallow-repository 2>/dev/null)" = false ] ||
    { echo "нужна полная история git (или --date): дата коммита zip в неполной копии неверна" >&2; exit 1; }
  date=$(git -C "$root" log -1 --format=%cI -- "internal/demopack/$name")
  [ -n "$date" ] || { echo "zip не в git: передайте --date" >&2; exit 1; }
fi
epoch=$(date -u -d "$date" +%s) || { echo "не ISO-время: '$date'" >&2; exit 2; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
pkg="$work/pkg"
for d in DEBIAN usr usr/share usr/share/sosninaalbums usr/share/doc usr/share/doc/sosninaalbums-demo; do
  install -d -m 0755 "$pkg/$d"
done
install -m 0644 "$zip" "$pkg/usr/share/sosninaalbums/$name"
# Пример набора — CC0 (спека 142 §1); текст лицензии — в системном каталоге (base-files)
cat >"$pkg/usr/share/doc/sosninaalbums-demo/copyright" <<CPR
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: Sosnina Albums
Source: https://github.com/IliaSA101/sosninaalbums

Files: usr/share/sosninaalbums/$name
Copyright: автор примера набора
License: CC0-1.0
 Пример набора передан в общественное достояние (CC0 1.0).
 The example pack is dedicated to the public domain (CC0 1.0).
 Полный текст / The full text: /usr/share/common-licenses/CC0-1.0
CPR
chmod 0644 "$pkg/usr/share/doc/sosninaalbums-demo/copyright"

size=$(du -sk --apparent-size "$pkg/usr" | cut -f1)
cat >"$pkg/DEBIAN/control" <<CTL
Package: sosninaalbums-demo
Version: $ver
Architecture: all
Multi-Arch: foreign
Section: graphics
Priority: optional
Installed-Size: $size
Enhances: sosninaalbums
Maintainer: Ilya Sosnin (github.com/IliaSA101)
Homepage: https://github.com/IliaSA101/sosninaalbums
Description: пример набора для Sosnina Albums
 Свадебный макет-пример для программы Sosnina Albums: работает без сети.
 Ставится рядом с пакетом sosninaalbums.
CTL
chmod 0644 "$pkg/DEBIAN/control"

find "$pkg" -exec touch -h -d "@$epoch" {} +
mkdir -p "$out"
deb="$out/SosninaAlbums-demo-$ver-linux-all.deb"
rm -f "$deb"
# -Zgzip: xz на уже сжатых PNG только тратит время; SOURCE_DATE_EPOCH — и время записей ar-контейнера
SOURCE_DATE_EPOCH=$epoch dpkg-deb --root-owner-group -Zgzip --build "$pkg" "$deb" >&2
echo "$deb"
