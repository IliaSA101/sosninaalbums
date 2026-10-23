#!/usr/bin/env bash
# Тест deb.sh и glibc.sh (спека 101, 101-12): .deb из пустышки — поля control, права, .desktop, воспроизводимость.
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
check 'состав: .desktop, значок, copyright, THIRD_PARTY_NOTICES' bash -c '
  l=$(dpkg-deb -c "$1"); for f in usr/share/applications/sosninaalbums.desktop usr/share/icons/hicolor/512x512/apps/sosninaalbums.png usr/share/doc/sosninaalbums/copyright usr/share/doc/sosninaalbums/THIRD_PARTY_NOTICES.txt; do grep -q " \./$f$" <<<"$l" || { echo "нет $f"; exit 1; }; done' _ "$deb"
check 'copyright: условие NOTICE.md и ссылка на GPL-3' bash -c '
  c=$(dpkg-deb --fsys-tarfile "$1" | tar -xO ./usr/share/doc/sosninaalbums/copyright); [[ "$c" == *7* ]] && [[ "$c" == */usr/share/common-licenses/GPL-3* ]]' _ "$deb"

if command -v desktop-file-validate >/dev/null; then
  mkdir -p "$WORK/x"
  dpkg-deb -x "$deb" "$WORK/x"
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

# glibc.sh: на самом себе (скрипт без символов) — код 1; на системном ls порог 99 проходит, порог 2.0 — нет
check 'glibc.sh: бинарник без символов GLIBC_ — код 1' bash -c '"$1" "$2" >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/glibc.sh" "$WORK/bin"
check 'glibc.sh: порог 99 проходит' "$HERE/glibc.sh" /bin/ls 99
check 'glibc.sh: порог 2.0 не проходит' bash -c '"$1" /bin/ls 2.0 >/dev/null 2>&1; [ $? = 1 ]' _ "$HERE/glibc.sh"

[ "$fails" = 0 ] && { echo 'test_deb.sh: всё прошло'; exit 0; }
echo "test_deb.sh: провалов $fails"
exit 1
