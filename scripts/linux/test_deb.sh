#!/usr/bin/env bash
# Тест deb.sh, deb-demo.sh и glibc.sh (спека 101, 101-12; 142): .deb из пустышки — поля control, права, .desktop, воспроизводимость.
#   scripts/linux/test_deb.sh        код ≠ 0 при первом провале
# Нужны dpkg-deb и objdump; desktop-file-validate — если есть (иначе пропуск с предупреждением).
set -uo pipefail

HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
fails=0
check() { # check <описание> <команда…>
  local what=$1
  shift
  if "$@" >"$WORK/last.txt" 2>&1; then echo "ok   $what"; else echo "FAIL $what"; sed 's/^/     /' "$WORK/last.txt"; fails=$((fails + 1)); fi
}

printf '#!/bin/sh\nexit 0\n' >"$WORK/bin"
chmod 755 "$WORK/bin"
date=2026-10-22T12:00:00+03:00
"$HERE/deb.sh" --bin "$WORK/bin" --version 0.23.0 --out "$WORK/a" --date "$date" >/dev/null 2>&1
deb="$WORK/a/SosninaAlbums-0.23.0-linux-amd64.deb"

check 'deb.sh: файл создан с ожидаемым именем' test -s "$deb"
check 'control: Package, Version, Architecture' bash -c '
  [ "$(dpkg-deb -f "$1" Package)" = sosninaalbums ] && [ "$(dpkg-deb -f "$1" Version)" = 0.23.0 ] && [ "$(dpkg-deb -f "$1" Architecture)" = amd64 ]' _ "$deb"
check 'control: Depends — GTK 3 и WebKitGTK 4.1' bash -c '
  d=$(dpkg-deb -f "$1" Depends); [[ "$d" == *libgtk-3-0* ]] && [[ "$d" == *libwebkit2gtk-4.1-0* ]]' _ "$deb"
check 'control: Maintainer без почты' bash -c '! dpkg-deb -f "$1" Maintainer | grep -q "@"' _ "$deb"
check 'права файлов: бинарник 0755 root, остальное 0644' bash -c '
  l=$(dpkg-deb -c "$1")
  grep -qE "^-rwxr-xr-x root/root .* \./usr/bin/sosninaalbums$" <<<"$l" &&
  [ "$(grep -E "^-" <<<"$l" | grep -vc "^-rw-r--r-- root/root")" = 1 ]' _ "$deb"
check 'права каталогов: все 0755 root' bash -c '! dpkg-deb -c "$1" | grep "^d" | grep -v "^drwxr-xr-x root/root"' _ "$deb"
check 'состав: .desktop, mime-тип, значок, copyright, THIRD_PARTY_NOTICES' bash -c '
  l=$(dpkg-deb -c "$1"); for f in usr/share/applications/sosninaalbums.desktop usr/share/mime/packages/sosninaalbums.xml usr/share/icons/hicolor/512x512/apps/sosninaalbums.png usr/share/doc/sosninaalbums/copyright usr/share/doc/sosninaalbums/THIRD_PARTY_NOTICES.txt; do grep -q " \./$f$" <<<"$l" || { echo "нет $f"; exit 1; }; done' _ "$deb"
check 'copyright: условие NOTICE.md и ссылка на GPL-3' bash -c '
  c=$(dpkg-deb --fsys-tarfile "$1" | tar -xO ./usr/share/doc/sosninaalbums/copyright); [[ "$c" == *7* ]] && [[ "$c" == */usr/share/common-licenses/GPL-3* ]]' _ "$deb"

mkdir -p "$WORK/x"
dpkg-deb -x "$deb" "$WORK/x"
D="$WORK/x/usr/share/applications/sosninaalbums.desktop"
M="$WORK/x/usr/share/mime/packages/sosninaalbums.xml"
# связь .salbum (спека 112-14): .desktop открывает файл и объявляет тип, xml описывает *.salbum
check '.desktop: Exec с %f' grep -qE '^Exec=sosninaalbums %f$' "$D"
check '.desktop: MimeType — наш тип' grep -qx 'MimeType=application/x-sosninaalbums;' "$D"
check 'mime-xml: тип и glob *.salbum' bash -c 'grep -q "type=\"application/x-sosninaalbums\"" "$1" && grep -q "pattern=\"\*.salbum\"" "$1"' _ "$M"
check 'mime-xml: права 0644 root' bash -c 'dpkg-deb -c "$1" | grep -qE "^-rw-r--r-- root/root .* \./usr/share/mime/packages/sosninaalbums\.xml$"' _ "$deb"
check 'mime-каталоги: 0755 root' bash -c 'l=$(dpkg-deb -c "$1"); grep -qE "^drwxr-xr-x root/root .* \./usr/share/mime/$" <<<"$l" && grep -qE "^drwxr-xr-x root/root .* \./usr/share/mime/packages/$" <<<"$l"' _ "$deb"
if command -v xmllint >/dev/null; then
  check 'mime-xml: well-formed (xmllint)' xmllint --noout "$M"
else
  echo 'skip xmllint не установлен'
fi
# Тип спрашиваем у инструмента, читающего базу shared-mime-info: xdg-mime без рабочего стола (CI) уходит в
# generic-ветку и без mimetype смотрит в содержимое файла, а не в glob — поэтому gio (libglib2.0-bin) или mimetype.
have_mime=1
if command -v gio >/dev/null; then
  mime_of() { gio info -a standard::content-type "$1" | sed -n 's/^ *standard::content-type: //p'; }
elif command -v mimetype >/dev/null; then
  mime_of() { mimetype -b "$1"; }
else
  have_mime=0
fi
if command -v update-mime-database >/dev/null && [ "$have_mime" = 1 ]; then
  export -f mime_of
  mkdir -p "$WORK/mh/mime/packages"
  cp "$M" "$WORK/mh/mime/packages/"
  update-mime-database "$WORK/mh/mime" >/dev/null 2>&1
  printf "{}" >"$WORK/Свадьба.salbum"
  printf "{}" >"$WORK/UPPER.SALBUM"
  export XDG_DATA_HOME="$WORK/mh" XDG_DATA_DIRS="$WORK/mh:/usr/share"
  check 'mime-база: .salbum — наш тип' bash -c '[ "$(mime_of "$1")" = application/x-sosninaalbums ]' _ "$WORK/Свадьба.salbum"
  check 'mime-база: регистр расширения не важен' bash -c '[ "$(mime_of "$1")" = application/x-sosninaalbums ]' _ "$WORK/UPPER.SALBUM"
else
  echo 'skip update-mime-database или gio/mimetype не установлены'
fi

if command -v desktop-file-validate >/dev/null; then
  check '.desktop проходит desktop-file-validate' desktop-file-validate "$WORK/x/usr/share/applications/sosninaalbums.desktop"
else
  echo 'skip desktop-file-validate не установлен'
fi

# воспроизводимость: два прогона с паузой, одинаковый sha256
sleep 2
"$HERE/deb.sh" --bin "$WORK/bin" --version 0.23.0 --out "$WORK/b" --date "$date" >/dev/null 2>&1
check 'два прогона дают одинаковый sha256' bash -c '[ "$(sha256sum <"$1")" = "$(sha256sum <"$2")" ]' _ "$deb" "$WORK/b/SosninaAlbums-0.23.0-linux-amd64.deb"
"$HERE/deb.sh" --bin "$WORK/bin" --version 0.23.0 --out "$WORK/c" --date 2026-10-23T12:00:00+03:00 >/dev/null 2>&1
check 'другая дата — другие байты' bash -c '[ "$(sha256sum <"$1")" != "$(sha256sum <"$2")" ]' _ "$deb" "$WORK/c/SosninaAlbums-0.23.0-linux-amd64.deb"

# края: плохая версия и нет бинарника — код 2 и 1
check 'неверная версия — код 2' bash -c '"$1" --bin "$2" --version 1.2 --out "$3" >/dev/null 2>&1; [ $? = 2 ]' _ "$HERE/deb.sh" "$WORK/bin" "$WORK/d"
check 'нет бинарника — код 1' bash -c '"$1" --bin "$2/нет" --version 1.2.3 --out "$3" >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/deb.sh" "$WORK" "$WORK/d"

# deb-demo.sh (спека 142, 0.30-142-47): пакет данных из пустышки-zip — поля control, состав, права, воспроизводимость
printf 'PK-пустышка' >"$WORK/wedding-navy-watercolor-demo-7.zip"
ddate=2026-10-28T12:00:00+03:00
"$HERE/deb-demo.sh" --zip "$WORK/wedding-navy-watercolor-demo-7.zip" --out "$WORK/e" --date "$ddate" >/dev/null 2>&1
demo="$WORK/e/SosninaAlbums-demo-7-linux-all.deb"
check 'deb-demo.sh: файл создан, версия — число из имени zip' bash -c '
  [ -s "$1" ] && [ "$(dpkg-deb -f "$1" Package)" = sosninaalbums-demo ] && [ "$(dpkg-deb -f "$1" Version)" = 7 ]' _ "$demo"
check 'deb-demo control: all, Multi-Arch foreign, Enhances, без Depends' bash -c '
  [ "$(dpkg-deb -f "$1" Architecture)" = all ] && [ "$(dpkg-deb -f "$1" Multi-Arch)" = foreign ] &&
  [ "$(dpkg-deb -f "$1" Enhances)" = sosninaalbums ] && [ -z "$(dpkg-deb -f "$1" Depends)" ]' _ "$demo"
check 'deb-demo: состав — zip и copyright, права 0644/0755 root' bash -c '
  l=$(dpkg-deb -c "$1")
  grep -qE "^-rw-r--r-- root/root .* \./usr/share/sosninaalbums/wedding-navy-watercolor-demo-7\.zip$" <<<"$l" &&
  grep -qE "^-rw-r--r-- root/root .* \./usr/share/doc/sosninaalbums-demo/copyright$" <<<"$l" &&
  ! grep "^d" <<<"$l" | grep -qv "^drwxr-xr-x root/root"' _ "$demo"
check 'deb-demo copyright: CC0' bash -c '
  dpkg-deb --fsys-tarfile "$1" | tar -xO ./usr/share/doc/sosninaalbums-demo/copyright | grep -q "^License: CC0-1.0"' _ "$demo"
check 'deb-demo: сжатие gzip' bash -c 'ar t "$1" | grep -qx "data.tar.gz"' _ "$demo"
sleep 2
"$HERE/deb-demo.sh" --zip "$WORK/wedding-navy-watercolor-demo-7.zip" --out "$WORK/f" --date "$ddate" >/dev/null 2>&1
check 'deb-demo: два прогона дают одинаковый sha256' bash -c '[ "$(sha256sum <"$1")" = "$(sha256sum <"$2")" ]' _ "$demo" "$WORK/f/SosninaAlbums-demo-7-linux-all.deb"
check 'deb-demo: неверное имя zip — код 2' bash -c '"$1" --zip "$2" --out "$3" --date "$4" >/dev/null 2>&1; [ $? = 2 ]' _ "$HERE/deb-demo.sh" "$WORK/bin" "$WORK/g" "$ddate"
check 'deb-demo: нет zip — код 1' bash -c '"$1" --zip "$2/нет-demo-1.zip" --out "$3" --date "$4" >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/deb-demo.sh" "$WORK" "$WORK/g" "$ddate"
# zip репозитория: дата берётся из истории (нужна полная копия), номер пакета — по имени zip
if [ "$(git -C "$HERE" rev-parse --is-shallow-repository 2>/dev/null)" = false ]; then
  check 'deb-demo: zip репозитория и дата коммита zip' bash -c '"$1" --out "$2" >/dev/null' _ "$HERE/deb-demo.sh" "$WORK/h"
else
  echo 'skip deb-demo с датой из истории: неполная копия'
fi

# glibc.sh: на самом себе (скрипт без символов) — код 1; на системном ls порог 99 проходит, порог 2.0 — нет
check 'glibc.sh: бинарник без символов GLIBC_ — код 1' bash -c '"$1" "$2" >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/glibc.sh" "$WORK/bin"
check 'glibc.sh: порог 99 проходит' "$HERE/glibc.sh" /bin/ls 99
check 'glibc.sh: порог 2.0 не проходит' bash -c '"$1" /bin/ls 2.0 >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/glibc.sh"

[ "$fails" = 0 ] && { echo 'test_deb.sh: всё прошло'; exit 0; }
echo "test_deb.sh: провалов $fails"
exit 1
