#!/usr/bin/env bash
# Сборка — двойник scripts/build.ps1 для Linux и macOS:
#   scripts/build.sh            — exe для Windows (wails build -platform windows/amd64): build/bin/SosninaAlbums-<версия>.exe
#   scripts/build.sh --native   — программа под эту ОС (Linux — с тегом webkit2_41): build/bin/SosninaAlbums-<версия>-<ос>
#   scripts/build.sh --native --deb — то же и пакет build/bin/SosninaAlbums-<версия>-linux-amd64.deb (scripts/linux/deb.sh;
#                                 только Linux, .ps1-двойника нет). Выпускной .deb и tar.gz собирает CI (release.yml, задания linux и pack).
# ПРАВЯТСЯ ВМЕСТЕ с build.ps1: версия из wails.json, -clean -trimpath, проверка на личные пути в бинарнике.
# Выпускной exe собирает CI на windows-latest (docs/RELEASE.md); exe отсюда — быстрая проверка и сверка сборки двух машин,
# не выпуск: тулчейн тот же (go.mod), но загрузчик WebView2 и метаданные сборки могут отличаться от раннера.
set -eu

native=0
deb=0
for a in "$@"; do
  case "$a" in
    --native | -native | -Native) native=1 ;;
    --deb) deb=1 ;;
    *) echo "неизвестный флаг: $a (есть --native, --deb)" >&2; exit 2 ;;
  esac
done
[ $deb = 0 ] || [ $native = 1 ] || { echo '--deb только вместе с --native' >&2; exit 2; }
[ $deb = 0 ] || [ "$(uname -s)" = Linux ] || { echo '--deb собирается только на Linux' >&2; exit 2; }

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
. "$root/scripts/env.sh"

ver=$(node -p "require('./wails.json').info.productVersion || ''")
[ -n "$ver" ] || { echo 'нет info.productVersion в wails.json' >&2; exit 1; }
command -v wails >/dev/null || { echo 'wails не найден (папка инструментов проекта, scripts/env.sh)' >&2; exit 1; }

# Дерево после сборки — как до неё (сверка сборки двух машин идёт из чистого дерева):
# - биндинги не пересоздаём: они в git и сверены контрактом, а Wails на Linux пишет их с битом исполнения;
# - frontend:install (npm install) дописывает в lock пометки своей версии npm — чистый до сборки lock возвращаем.
lock_clean=0
git diff --quiet -- frontend/package-lock.json 2>/dev/null && lock_clean=1
restore_lock() { [ $lock_clean = 1 ] && git checkout -- frontend/package-lock.json 2>/dev/null; return 0; }
trap restore_lock EXIT

bin="$root/build/bin"
if [ $native = 1 ]; then
  wails build -clean -trimpath -skipbindings ${wails_tags:+-tags "$wails_tags"}
  os=$(go env GOOS)
  out="$bin/SosninaAlbums-$ver-$os"
  if [ "$os" = darwin ]; then
    rm -rf "$out.app" && mv "$bin/sosninaalbums.app" "$out.app" && out="$out.app/Contents/MacOS/sosninaalbums"
  else
    mv -f "$bin/sosninaalbums" "$out"
  fi
else
  wails build -platform windows/amd64 -clean -trimpath -skipbindings
  out="$bin/SosninaAlbums-$ver.exe"
  mv -f "$bin/sosninaalbums.exe" "$out"
fi

# Защита от утечки личных путей: -trimpath должен вычистить их из бинарника. Образцы — из окружения сборщика,
# а не из скрипта: так в репозиторий не попадает ни имя, ни путь. На Linux домашний каталог — /home/<имя>.
for s in "$HOME" "/home/${USER:-$(id -un)}" "$root" "$main"; do
  if LC_ALL=C grep -aqF -- "$s" "$out"; then
    echo "в сборке найдена строка '$s' — личные данные в бинарнике, публиковать нельзя" >&2
    exit 1
  fi
done
echo "Готово: $out"

if [ $deb = 1 ]; then
  # Страж glibc здесь только сообщает: локальная машина новее Ubuntu 22.04, блокирует его CI (ubuntu-24.04, release.yml)
  "$root/scripts/linux/glibc.sh" "$out" || echo 'внимание: порог glibc выше 2.35 — пакет не проверен на Ubuntu 22.04 (в выпуске страж блокирующий)' >&2
  "$root/scripts/linux/deb.sh" --bin "$out" --version "$ver" --out "$bin" --date "$(git log -1 --format=%cI)"
fi
