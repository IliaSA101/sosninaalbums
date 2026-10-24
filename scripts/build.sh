#!/usr/bin/env bash
# Сборка — двойник scripts/build.ps1 для Linux и macOS:
#   scripts/build.sh            — exe для Windows (wails build -platform windows/amd64): build/bin/SosninaAlbums-<версия>.exe
#   scripts/build.sh --native   — программа под эту ОС (Linux — с тегом webkit2_41): build/bin/SosninaAlbums-<версия>-<ос>
#   scripts/build.sh --native --deb — то же и пакет build/bin/SosninaAlbums-<версия>-linux-amd64.deb (scripts/linux/deb.sh;
#                                 только Linux, .ps1-двойника нет). Выпускной .deb и tar.gz собирает CI (release.yml, задания linux и pack).
#   scripts/build.sh --installer — и установщик для Windows рядом с exe (нужен makensis; scripts/win/installer.sh, спека 111):
#                                 build/bin/SosninaAlbums-<версия>-windows-x64-setup.exe. Выпускной установщик собирает CI (задание pack).
#   scripts/build.sh --tags qa  — свои теги сборки (только здесь, у .ps1 двойника нет): exe набора QA (scripts/win/qa-kit.sh) читает
#                                 SA_STAND и SA_UPDATE_URL, как wails dev. Тег dev для wails build не годится: Wails сам ставит
#                                 production, и его app_dev.go с app_production.go вместе не собираются. Выпуск (CI) тегов не передаёт.
# ПРАВЯТСЯ ВМЕСТЕ с build.ps1: версия из wails.json, -clean -trimpath, проверка на личные пути в бинарнике.
# Выпускной exe собирает CI на windows-latest (docs/RELEASE.md); exe отсюда — быстрая проверка и сверка сборки двух машин,
# не выпуск: тулчейн тот же (go.mod), но загрузчик WebView2 и метаданные сборки могут отличаться от раннера.
set -eu

native=0
deb=0
installer=0
tags=''
while [ $# -gt 0 ]; do
  a=$1
  shift
  case "$a" in
    --native | -native | -Native) native=1 ;;
    --deb) deb=1 ;;
    --installer | -installer | -Installer) installer=1 ;;
    --tags) [ $# -gt 0 ] || { echo '--tags: нужны теги (например qa)' >&2; exit 2; }; tags=$1; shift ;;
    *) echo "неизвестный флаг: $a (есть --native, --deb, --installer, --tags)" >&2; exit 2 ;;
  esac
done
[ $deb = 0 ] || [ $native = 1 ] || { echo '--deb только вместе с --native' >&2; exit 2; }
[ $installer = 0 ] || [ $native = 0 ] || { echo '--installer — установщик Windows, без --native' >&2; exit 2; }
[ $deb = 0 ] || [ "$(uname -s)" = Linux ] || { echo '--deb собирается только на Linux' >&2; exit 2; }
# makensis нужен в конце; не тратить минуты сборки, чтобы упасть на последнем шаге
[ $installer = 0 ] || command -v makensis >/dev/null || { echo 'для --installer нужен makensis (Ubuntu: sudo apt install nsis)' >&2; exit 1; }

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
  all_tags=${wails_tags:+$wails_tags${tags:+,}}$tags
  wails build -clean -trimpath -skipbindings ${all_tags:+-tags "$all_tags"}
  os=$(go env GOOS)
  out="$bin/SosninaAlbums-$ver-$os"
  if [ "$os" = darwin ]; then
    rm -rf "$out.app" && mv "$bin/sosninaalbums.app" "$out.app" && out="$out.app/Contents/MacOS/sosninaalbums"
  else
    mv -f "$bin/sosninaalbums" "$out"
  fi
else
  # CGO_ENABLED=0 явно: так же обязан собирать CI (release.yml), иначе установщик не сойдётся с пересборкой release-sign.mjs
  CGO_ENABLED=0 wails build -platform windows/amd64 -clean -trimpath -skipbindings ${tags:+-tags "$tags"}
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

if [ $installer = 1 ]; then
  # bash явно: в метке режим файла 100644 (как в CI, где pack зовёт installer.sh через bash)
  # exe в установщике — тот же файл, что проверен выше на личные пути; имя внутри установки постоянное (SosninaAlbums.exe)
  bash "$root/scripts/win/installer.sh" --exe "$out" --version "$ver" --out "$bin"
fi
