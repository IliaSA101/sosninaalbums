#!/usr/bin/env bash
# Блокирующая проверка Linux-сборки (спека 101, design §3.3): smoke.sh <.deb|.tar.gz> <папка вывода>
# Образец — scripts/mac/smoke.sh: сводка в <папка>/summary.md, подробности в logs/, код ≠ 0 при первом провале.
# Окружение: LINUX_ALIVE_SEC (10) — сколько секунд процесс обязан прожить; GLIBC_MAX (2.35) — порог стража;
# LINUX_SMOKE_SELFTEST=crash — бинарник заменяется на «exit 1», чтобы убедиться, что п. 4 действительно валит проверку.
# Запуск изолирован: своя шина dbus-run-session, свой HOME/XDG_* во временной папке и свой Xvfb. Без отдельной шины
# SingleInstance поднял бы окно запущенной у владельца программы, а настройки писались бы в его папки.
set -uo pipefail

[ "$#" -eq 2 ] || { echo "использование: smoke.sh <.deb|.tar.gz> <папка вывода>" >&2; exit 2; }
PKG=$(readlink -f -- "$1")
OUT=$(mkdir -p -- "$2" && readlink -f -- "$2")
[ -f "$PKG" ] || { echo "нет файла: $1" >&2; exit 2; }
mkdir -p "$OUT/logs"
HERE=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
ALIVE="${LINUX_ALIVE_SEC:-10}"
WORK=$(mktemp -d)
APP_PID=''
cleanup() {
  [ -n "$APP_PID" ] && kill -KILL "$APP_PID" 2>/dev/null
  rm -rf "$WORK"
}
trap cleanup EXIT

case "$PKG" in
  *.deb) KIND=deb ;;
  *.tar.gz) KIND=tar ;;
  *) echo "ожидался .deb или .tar.gz: $PKG" >&2; exit 2 ;;
esac
VERSION=$(basename "$PKG" | sed -n 's/^SosninaAlbums-\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)-linux-amd64\.\(deb\|tar\.gz\)$/\1/p')
[ -n "$VERSION" ] || { echo "имя файла не вида SosninaAlbums-X.Y.Z-linux-amd64.(deb|tar.gz): $PKG" >&2; exit 2; }

# ячейка таблицы Markdown: без «|» и переводов строк
cell() { printf '%s' "$1" | tr '\n|' ' /'; }
note() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*" | tee -a "$OUT/logs/steps.txt" >&2; }
row() {
  printf '| %s | %s | %s |\n' "$(cell "$1")" "$(cell "$2")" "$(cell "$3")" >>"$OUT/summary.md"
  note "$1: $2${3:+ — $3}"
}
pass() { row "$1" "да" "${2:-}"; }

{
  printf '# Linux: %s (%s), раннер %s\n\n' "$(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-?}")" "$(uname -m)" "${RUNNER_NAME:-локально}"
  printf '## Проверка запуска (smoke.sh, %s)\n\n| вариант | итог | почему |\n| --- | --- | --- |\n' "$(basename "$PKG")"
} >"$OUT/summary.md"

# diag <файл> — хвост stderr и log.txt программы: в сводку и свёрнутым блоком в журнал задания
diag() {
  local d="$OUT/logs/smoke-diag.txt"
  {
    echo "== stderr (50 строк): ${1:-нет}"
    [ -n "${1:-}" ] && [ -f "$1" ] && tail -n 50 "$1"
    echo "== log.txt"
    [ -f "${LOGFILE:-}" ] && tail -n 50 "$LOGFILE"
  } >"$d" 2>&1
  echo "::group::Диагностика запуска"
  cat "$d"
  echo "::endgroup::"
}

# fail <пункт> <почему> [<stderr>] — строка «нет», диагностика, выход
fail() {
  row "$1" "нет" "$2"
  echo "::error::Проверка Linux, $1: $2"
  diag "${3:-}"
  exit 1
}

# ---------- 1. Содержимое пакета ----------
if [ "$KIND" = deb ]; then
  dpkg-deb -I "$PKG" >"$OUT/logs/smoke-deb-info.txt" 2>&1 || fail "1 пакет" "dpkg-deb -I не читает $(basename "$PKG")"
  dpkg-deb -c "$PKG" >"$OUT/logs/smoke-deb-list.txt" 2>&1 || fail "1 пакет" "dpkg-deb -c не читает $(basename "$PKG")"
  field() { dpkg-deb -f "$PKG" "$1"; }
  [ "$(field Package)" = sosninaalbums ] || fail "1 пакет" "Package = '$(field Package)'"
  [ "$(field Version)" = "$VERSION" ] || fail "1 пакет" "Version = '$(field Version)', по имени файла $VERSION"
  [ "$(field Architecture)" = amd64 ] || fail "1 пакет" "Architecture = '$(field Architecture)'"
  for dep in libgtk-3-0 libwebkit2gtk-4.1-0; do
    field Depends | grep -qw -- "$dep" || fail "1 пакет" "в Depends нет $dep"
  done
  list=$(cat "$OUT/logs/smoke-deb-list.txt")
  want() { grep -E -- "$1" <<<"$list" >/dev/null || fail "1 пакет" "в пакете нет записи $2"; }
  want '^-rwxr-xr-x root/root +[0-9]+ .* \./usr/bin/sosninaalbums$' "/usr/bin/sosninaalbums с правами 0755 от root"
  want '^-rw-r--r-- root/root +[0-9]+ .* \./usr/share/applications/sosninaalbums\.desktop$' "sosninaalbums.desktop (0644)"
  want '^-rw-r--r-- root/root +[0-9]+ .* \./usr/share/mime/packages/sosninaalbums\.xml$' "mime-тип .salbum (0644)"
  want '^-rw-r--r-- root/root +[0-9]+ .* \./usr/share/icons/hicolor/512x512/apps/sosninaalbums\.png$' "значок 512x512 (0644)"
  want '^-rw-r--r-- root/root +[0-9]+ .* \./usr/share/doc/sosninaalbums/copyright$' "doc/copyright"
  want '^-rw-r--r-- root/root +[0-9]+ .* \./usr/share/doc/sosninaalbums/THIRD_PARTY_NOTICES\.txt$' "doc/THIRD_PARTY_NOTICES.txt"
  # каталоги — 0755 (umask сборщика попасть не должен)
  if grep -E '^d' <<<"$list" | grep -v '^drwxr-xr-x root/root' >/dev/null; then
    fail "1 пакет" "каталог не с правами 0755 от root: $(grep -E '^d' <<<"$list" | grep -v '^drwxr-xr-x root/root' | head -n 1)"
  fi
  mkdir -p "$WORK/root"
  dpkg-deb -x "$PKG" "$WORK/root" || fail "1 пакет" "dpkg-deb -x не распаковал пакет"
  BIN="$WORK/root/usr/bin/sosninaalbums"
  DESKTOP="$WORK/root/usr/share/applications/sosninaalbums.desktop"
  if command -v desktop-file-validate >/dev/null; then
    desktop-file-validate "$DESKTOP" >"$OUT/logs/smoke-desktop.txt" 2>&1 || fail "1 пакет" "desktop-file-validate: $(head -n 1 "$OUT/logs/smoke-desktop.txt")"
    dv="sosninaalbums.desktop валиден"
  else
    dv="desktop-file-validate нет — .desktop не проверен"
    echo "::warning::$dv"
  fi
  pass "1 пакет" "$(field Package) $VERSION amd64, Depends GTK 3 и WebKitGTK 4.1, права 0755/0644, $dv"

  # ---------- 1б. Связь .salbum (спека 112-14) ----------
  # .desktop: открывает файл (%f) и объявляет наш тип
  grep -qE '^Exec=.* %f$' "$DESKTOP" || fail "1б mime" "в Exec .desktop нет %f"
  grep -qE '^MimeType=(.*;)?application/x-sosninaalbums;' "$DESKTOP" || fail "1б mime" "в .desktop нет MimeType=application/x-sosninaalbums"
  # Установка в контейнере не нужна: базу mime собираем из xml пакета в своей XDG_DATA_HOME, система владельца не затрагивается
  if command -v update-mime-database >/dev/null && command -v xdg-mime >/dev/null; then
    MIMEHOME="$WORK/mimehome"
    mkdir -p "$MIMEHOME/mime/packages"
    cp "$WORK/root/usr/share/mime/packages/sosninaalbums.xml" "$MIMEHOME/mime/packages/" || fail "1б mime" "в пакете нет mime-файла"
    update-mime-database "$MIMEHOME/mime" >"$OUT/logs/smoke-mime.txt" 2>&1 || fail "1б mime" "update-mime-database: $(head -n 1 "$OUT/logs/smoke-mime.txt")"
    printf '{}' >"$WORK/a.salbum"
    got=$(XDG_DATA_HOME="$MIMEHOME" XDG_DATA_DIRS="$MIMEHOME:/usr/share" xdg-mime query filetype "$WORK/a.salbum" 2>>"$OUT/logs/smoke-mime.txt")
    [ "$got" = application/x-sosninaalbums ] || fail "1б mime" "xdg-mime query filetype = '$got', нужен application/x-sosninaalbums"
    pass "1б mime" "xdg-mime: a.salbum = $got, в .desktop %f и MimeType"
  else
    row "1б mime" "пропуск" "нет update-mime-database или xdg-mime; .desktop проверен (%f, MimeType)"
    echo "::warning::нет update-mime-database/xdg-mime — тип .salbum не проверен"
  fi
else
  mkdir -p "$WORK/root"
  tar -xzf "$PKG" -C "$WORK/root" || fail "1 архив" "tar не распаковал $(basename "$PKG")"
  BIN="$WORK/root/Sosnina Albums $VERSION/sosninaalbums"
  [ -f "$BIN" ] || fail "1 архив" "нет «Sosnina Albums $VERSION/sosninaalbums»"
  [ -x "$BIN" ] || fail "1 архив" "sosninaalbums без бита исполнения"
  pass "1 архив" "«Sosnina Albums $VERSION/sosninaalbums» на месте, исполняемый"
fi

# ---------- 2. Библиотеки ----------
ldd "$BIN" >"$OUT/logs/smoke-ldd.txt" 2>&1
if grep -q 'not found' "$OUT/logs/smoke-ldd.txt"; then
  fail "2 библиотеки" "ldd: $(grep 'not found' "$OUT/logs/smoke-ldd.txt" | head -n 3 | tr -s ' \t\n' ' ')"
fi
pass "2 библиотеки" "ldd: «not found» нет"

# ---------- 3. glibc ----------
g=$("$HERE/glibc.sh" "$BIN" 2>"$OUT/logs/smoke-glibc.err") || fail "3 glibc" "$(tr '\n' ' ' <"$OUT/logs/smoke-glibc.err")"
pass "3 glibc" "$g"

# Самопроверка: испорченный бинарник должен уронить п. 4 (п. 1–3 уже пройдены на настоящем)
if [ "${LINUX_SMOKE_SELFTEST:-}" = crash ]; then
  printf '#!/bin/sh\necho "selftest: crash" >&2\nexit 1\n' >"$BIN"
  chmod 755 "$BIN"
  row "самопроверка" "предупреждение" "LINUX_SMOKE_SELFTEST=crash: sosninaalbums заменён на exit 1"
fi

# ---------- 4. Запуск под своей шиной и Xvfb ----------
H="$WORK/home"
mkdir -p "$H/.config" "$H/.local/share" "$H/.cache" "$WORK/run"
chmod 700 "$WORK/run"
export HOME="$H" XDG_CONFIG_HOME="$H/.config" XDG_DATA_HOME="$H/.local/share" XDG_CACHE_HOME="$H/.cache" XDG_RUNTIME_DIR="$WORK/run"
# Песочница WebKit (bubblewrap) в контейнере и под AppArmor раннера не получает прав. GIO_USE_VFS=local — без gvfs-демона
# (он монтирует fuse в XDG_RUNTIME_DIR и мешает убрать временную папку)
export WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1 GIO_USE_VFS=local
unset WAYLAND_DISPLAY DISPLAY DBUS_SESSION_BUS_ADDRESS
LOGFILE="$XDG_CONFIG_HOME/SosninaAlbums/log.txt"
ERR="$OUT/logs/smoke-run.err"
PIDFILE="$WORK/app.pid"

# Внутренняя оболочка пишет свой pid и заменяется программой (exec): pid остаётся тем же, TERM придёт именно ей
dbus-run-session -- xvfb-run -a sh -c 'echo $$ >"$1"; exec "$2"' sh "$PIDFILE" "$BIN" >"$OUT/logs/smoke-run.out" 2>"$ERR" &
RUN_PID=$!
i=0
while [ ! -s "$PIDFILE" ] && [ "$i" -lt 30 ]; do sleep 1; i=$((i + 1)); done
[ -s "$PIDFILE" ] || fail "4 запуск" "за 30 с программа не запустилась (нет dbus-run-session или xvfb-run?)" "$ERR"
APP_PID=$(cat "$PIDFILE")
i=0
while [ "$i" -lt "$ALIVE" ]; do
  sleep 1
  i=$((i + 1))
  if ! kill -0 "$APP_PID" 2>/dev/null; then
    wait "$RUN_PID"
    fail "4 запуск" "процесс вышел через $i с, код $?" "$ERR"
  fi
done
pass "4 запуск" "жил $ALIVE с под dbus-run-session + xvfb-run"

# ---------- 5. Строка запуска в log.txt и отсутствие ошибок ----------
sleep 1
want="запуск, версия $VERSION"
[ -f "$LOGFILE" ] && grep -aqF -- "$want" "$LOGFILE" || fail "5 log.txt" "нет строки «$want» в $LOGFILE" "$ERR"
if grep -aiE 'ошибк|error|panic|fatal|не удалось' "$LOGFILE" >"$OUT/logs/smoke-log-errors.txt"; then
  fail "5 log.txt" "в журнале есть ошибки: $(head -n 1 "$OUT/logs/smoke-log-errors.txt")" "$ERR"
fi
pass "5 log.txt" "есть «$want», ошибок нет"

# ---------- 6. Папки данных WebKit ----------
# Решение 0.23-r2: Wails 2.16 вызывает g_set_prgname из linux.Options.ProgramName уже ПОСЛЕ создания окна, поэтому
# WebKit называет свои папки по имени файла программы, а не по ProgramName. В поставке имя файла постоянное
# (sosninaalbums, версия — только в имени архива), настройки интерфейса при обновлении не теряются.
if [ -d "$XDG_DATA_HOME/sosninaalbums" ]; then
  row "6 данные" "да" "WebKit хранит данные в $XDG_DATA_HOME/sosninaalbums (по имени файла программы)"
else
  webkit=$(cd "$XDG_DATA_HOME" 2>/dev/null && ls -d -- * 2>/dev/null | tr '\n' ' ')
  fail "6 данные" "нет $XDG_DATA_HOME/sosninaalbums; есть: ${webkit:-нет папок}" "$ERR"
fi

# ---------- 7. Выход по TERM ----------
kill -TERM "$APP_PID" 2>/dev/null
i=0
while kill -0 "$APP_PID" 2>/dev/null && [ "$i" -lt 10 ]; do sleep 1; i=$((i + 1)); done
if kill -0 "$APP_PID" 2>/dev/null; then
  kill -KILL "$APP_PID" 2>/dev/null
  fail "7 выход" "не завершился по TERM за 10 с" "$ERR"
fi
wait "$RUN_PID" 2>/dev/null
APP_PID=''
grep -aqF -- 'выход' "$LOGFILE" || fail "7 выход" "после TERM в журнале нет «выход»" "$ERR"
pass "7 выход" "TERM → процесс завершился, в журнале «выход»"

echo "Проверка Linux пройдена: $(basename "$PKG")"
exit 0
