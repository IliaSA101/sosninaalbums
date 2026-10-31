#!/usr/bin/env bash
# Сборка .deb (спека 101, design §3.3): только dpkg-deb, без nfpm и сторонних действий.
#   запуск: deb.sh --bin <бинарник> --version X.Y.Z --out <папка> [--date <ISO-время>]  # lang-ok
# Результат: <D>/SosninaAlbums-<X.Y.Z>-linux-amd64.deb; последняя строка stdout — путь к нему.
# Воспроизводимость: время всех файлов — --date (в выпуске — время коммита метки; иначе SOURCE_DATE_EPOCH, иначе
# текущее), владелец root, права заданы явно (каталоги не наследуют umask): одинаковые входы дают одинаковые байты.
# Остальное содержимое — из репозитория: build/linux (.desktop, mime-тип .salbum, значок), NOTICE.md, THIRD_PARTY_NOTICES.txt.
set -euo pipefail

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
bin='' ver='' out='' date=''
while [ $# -gt 0 ]; do
  case "$1" in
    --bin) bin=${2:?нет значения у --bin}; shift 2 ;;
    --version) ver=${2:?нет значения у --version}; shift 2 ;;
    --out) out=${2:?нет значения у --out}; shift 2 ;;
    --date) date=${2:?нет значения у --date}; shift 2 ;;
    *) echo "неизвестный аргумент: $1 (есть --bin --version --out --date)" >&2; exit 2 ;;
  esac
done
[ -n "$bin" ] && [ -n "$ver" ] && [ -n "$out" ] || { echo "использование: deb.sh --bin B --version X.Y.Z --out D [--date ISO]" >&2; exit 2; }
[[ "$ver" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "неверная версия: '$ver'" >&2; exit 2; }
[ -s "$bin" ] || { echo "нет бинарника: $bin" >&2; exit 1; }
for f in build/linux/sosninaalbums.desktop build/linux/sosninaalbums.xml build/linux/sosninaalbums.png NOTICE.md THIRD_PARTY_NOTICES.txt; do
  [ -s "$root/$f" ] || { echo "нет входного файла: $f" >&2; exit 1; }
done

if [ -n "$date" ]; then
  epoch=$(date -u -d "$date" +%s) || { echo "не ISO-время: '$date'" >&2; exit 2; }
else
  epoch=${SOURCE_DATE_EPOCH:-$(date +%s)}
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
pkg="$work/pkg"
# Каталоги — по одному и с явным правом: промежуточные install -d создал бы с правами по umask (0775)
for d in DEBIAN usr usr/bin usr/share usr/share/applications usr/share/mime usr/share/mime/packages usr/share/doc usr/share/doc/sosninaalbums \
  usr/share/icons usr/share/icons/hicolor usr/share/icons/hicolor/512x512 usr/share/icons/hicolor/512x512/apps; do
  install -d -m 0755 "$pkg/$d"
done
install -m 0755 "$bin" "$pkg/usr/bin/sosninaalbums"
install -m 0644 "$root/build/linux/sosninaalbums.desktop" "$pkg/usr/share/applications/sosninaalbums.desktop"
# mime-тип .salbum: базы mime и desktop обновляют триггеры dpkg (shared-mime-info, desktop-file-utils), postinst не нужен
install -m 0644 "$root/build/linux/sosninaalbums.xml" "$pkg/usr/share/mime/packages/sosninaalbums.xml"
install -m 0644 "$root/build/linux/sosninaalbums.png" "$pkg/usr/share/icons/hicolor/512x512/apps/sosninaalbums.png"
install -m 0644 "$root/THIRD_PARTY_NOTICES.txt" "$pkg/usr/share/doc/sosninaalbums/THIRD_PARTY_NOTICES.txt"
# copyright: NOTICE.md как есть, текст GPL v3 — в системном каталоге лицензий (его ставит base-files)
{
  cat "$root/NOTICE.md"
  printf '\n\nПолный текст GNU GPL версии 3 — в /usr/share/common-licenses/GPL-3.\n'
  printf 'The full text of the GNU GPL version 3 is in /usr/share/common-licenses/GPL-3.\n'
} >"$pkg/usr/share/doc/sosninaalbums/copyright"
chmod 0644 "$pkg/usr/share/doc/sosninaalbums/copyright"

size=$(du -sk --apparent-size "$pkg/usr" | cut -f1)
# Maintainer без почты: в репозиторий личные адреса не пишем (dpkg-deb такое принимает)
cat >"$pkg/DEBIAN/control" <<CTL
Package: sosninaalbums
Version: $ver
Architecture: amd64
Section: graphics
Priority: optional
Installed-Size: $size
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Maintainer: Ilya Sosnin (github.com/IliaSA101)
Homepage: https://github.com/IliaSA101/sosninaalbums
Description: вёрстка свадебных фотоальбомов
 Sosnina Albums: выбираете папки с фото, раскладываете их по разворотам
 вручную или автосборкой и выгружаете JPEG 300 dpi.
CTL
chmod 0644 "$pkg/DEBIAN/control"

find "$pkg" -exec touch -h -d "@$epoch" {} +
mkdir -p "$out"
deb="$out/SosninaAlbums-$ver-linux-amd64.deb"
rm -f "$deb"
# SOURCE_DATE_EPOCH — и время записей ar-контейнера (без него в .deb попадает текущее время)
SOURCE_DATE_EPOCH=$epoch dpkg-deb --root-owner-group -Zxz --build "$pkg" "$deb" >&2
echo "$deb"
