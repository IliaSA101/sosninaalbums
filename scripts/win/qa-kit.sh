#!/usr/bin/env bash
# Набор проверки 0.25 в exe на ноутбуке Windows (спека 111 design §9, 112 design §8, 113; сценарии — docs/qa/update.md).
# Собирается на Ubuntu в .gotmp/qa-kit-0.25/ и целиком переносится на ноутбук:
#   scripts/win/qa-kit.sh [--out <папка внутри .gotmp>] [--ref <коммит>] [--no-rel] [--dry] [--fake]
#     --ref      откуда брать код (по умолчанию HEAD; незакоммиченное в набор не попадает)
#     --no-rel   без «выпускного» exe 0.25.0 (он нужен для связи .salbum и первого запуска)
#     --dry      только напечатать шаги, ничего не собирая
#     --fake     вместо сборки exe и установщиков — файлы-заглушки (проверка ленты и раскладки без wails и makensis)
# Что собирается (временная копия дерева `git worktree` под .gotmp/qa-kit-work, версия в wails.json и whatsnew.ts):
#   zip/…-0.25.0-qa-…, …-0.25.1-qa-… — exe с тегом qa (SA_STAND и SA_UPDATE_URL читаются, как у wails dev; связь .salbum не пишется);
#   zip/…-0.25.0-rel-… — тот же код без тега: «как выпуск» (вопрос о связи .salbum, тур, настоящий адрес обновлений);
#   setup/ — установщики 0.25.0 и 0.25.1 (scripts/win/installer.sh, нужен makensis; NSISDIR — как у installer.sh);
#   feed/ — лента: SosninaAlbums-update.json (тестовый ключ, release-sign.mjs --from) + v0.25.1/<установщик>;
#   feed-bad-byte/, feed-wrong-key/, feed-old-setup/ — испорченные ленты для QA-4; pub.txt — открытый тестовый ключ;
#   albums/ — .salbum «из новой версии» (формат 99); *.cmd, README.txt, update.md, installer-check.ps1.
# Закрытые тестовые ключи живут в .gotmp и в набор не попадают; временная копия дерева удаляется всегда.
# Время: npm ci ≈ 0,5 мин, три сборки exe ≈ 2–3 мин каждая, три установщика ≈ 0,5 мин — всего 8–12 мин.
set -euo pipefail

# Версии набора: старая (установлена), новая (в ленте), «старее старой» (ловит откат в установщике)
OLD=0.25.0 NEW=0.25.1 STALE=0.24.9
ASSET=SosninaAlbums-update.json

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
out="$root/.gotmp/qa-kit-0.25"
ref=HEAD dry=0 fake=0 rel=1
die() { echo "qa-kit: $*" >&2; exit 1; }
say() { printf '==> %s\n' "$*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --out) [ $# -ge 2 ] || die '--out: нужна папка'; out=$2; shift 2 ;;
    --ref) [ $# -ge 2 ] || die '--ref: нужен коммит'; ref=$2; shift 2 ;;
    --no-rel) rel=0; shift ;;
    --dry) dry=1; shift ;;
    --fake) fake=1; shift ;;
    *) echo "неизвестный флаг: $1 (есть --out, --ref, --no-rel, --dry, --fake)" >&2; exit 2 ;;
  esac
done

# Папка набора стирается целиком: только внутри .gotmp этого дерева, не корень и не чужое место
out=$(realpath -m -- "$out")
case "$out" in "$root/.gotmp/"?*) ;; *) die "папка набора должна быть внутри $root/.gotmp/ (получено: $out)" ;; esac
work="$root/.gotmp/qa-kit-work"
src="$work/src"
key1="$root/.gotmp/qa-kit-key1.pem" key2="$root/.gotmp/qa-kit-key2.pem"

if [ $dry = 1 ]; then
  cat >&2 <<EOF
Шаги (--dry, ничего не выполняется):
  1. проверка инструментов: git node npm python3 sha256sum$([ $fake = 1 ] || echo ' wails makensis')
  2. $([ $fake = 1 ] || echo "git worktree add --detach $src $ref; npm ci в frontend; ")wails.json и whatsnew.ts — версия $OLD / $NEW
  3. exe qa $OLD, exe qa $NEW$([ $rel = 1 ] && echo ", exe rel $OLD") — scripts/build.sh --tags qa (CGO_ENABLED=0, -trimpath)
  4. zip/ — три архива «Sosnina Albums <версия>-<qa|rel>/SosninaAlbums-<версия>.exe»
  5. setup/ — установщики $OLD (из exe qa $OLD) и $NEW (из exe qa $NEW); для feed-old-setup — установщик $STALE из exe qa $NEW
  6. тестовые ключи → pub.txt; feed/ — release-sign.mjs v$NEW --from … --skip-rebuild --whatsnew …; затем три испорченные ленты
  7. albums/ — .salbum формата 99 («из новой версии»); *.cmd, README.txt, update.md, installer-check.ps1, kit-sha256.txt
  8. самопроверка лент (подпись, sha256), удаление временной копии и закрытых ключей
Результат: $out
EOF
  exit 0
fi

need() { command -v "$1" >/dev/null 2>&1 || die "нет $1${2:+ ($2)}"; }
need git; need node; need python3; need sha256sum
if [ $fake = 0 ]; then
  need npm
  . "$root/scripts/env.sh"
  need wails 'папка инструментов проекта, scripts/env.sh'
  if ! command -v makensis >/dev/null 2>&1 && [ -n "${NSISDIR:-}" ]; then
    # makensis из распакованного пакета: рядом с NSISDIR (…/usr/share/nsis) лежит ../../bin
    for d in "$NSISDIR/bin" "$NSISDIR/../../bin" "$NSISDIR"; do
      [ -x "$d/makensis" ] && { PATH="$d:$PATH"; export PATH; break; }
    done
  fi
  need makensis 'Ubuntu: sudo apt install nsis; без установки — NSISDIR'
fi

cleanup() {
  if [ -d "$src/.git" ] || [ -f "$src/.git" ]; then
    git -C "$root" worktree remove --force "$src" >/dev/null 2>&1 || rm -rf "$src"
  fi
  git -C "$root" worktree prune >/dev/null 2>&1 || true
  rm -rf "$work" "$root/.gotmp/release-sign/v$NEW"
  rm -f "$key1" "$key2"
}
trap cleanup EXIT

rm -rf "$out" "$work"
mkdir -p "$out/zip" "$out/setup" "$out/albums" "$work/stage"
if [ -n "$(git -C "$root" status --porcelain --untracked-files=no)" ]; then
  echo 'внимание: в дереве есть незакоммиченные правки — в набор попадёт только коммит '"$ref" >&2
fi

# ---------- сборка ----------

# blob <файл> <метка> — заглушка для --fake: 2,2 МБ (установщик не бывает меньше 1 МБ)
blob() { { printf 'FAKE %s\n' "$2"; head -c 2200000 /dev/zero; } > "$1"; }

if [ $fake = 0 ]; then
  say "временная копия дерева ($ref)"
  git -C "$root" worktree prune
  git -C "$root" worktree add --detach "$src" "$ref" >&2
  grep -q -- '--tags' "$src/scripts/build.sh" || die "в $ref нет build.sh --tags: ветку набора нужно закоммитить"
  grep -q 'dev || qa' "$src/internal/osx/stand_dev.go" || die "в $ref нет тега qa (internal/osx/stand_dev.go)"
  say 'npm ci'
  (cd "$src/frontend" && npm ci >&2)
fi

# set_version <X.Y.Z> — версия сборки: wails.json (её читает программа) и whatsnew.ts (страж равенства; текст «Что нового» 0.25 не трогаем)
set_version() {
  sed -i -E 's/("productVersion": *")[^"]*(")/\1'"$1"'\2/' "$src/wails.json"
  sed -i -E "s/(WHATS_NEW_VERSION = ')[^']*(')/\\1$1\\2/" "$src/frontend/src/shell/dialogs/whatsnew.ts"
  grep -q "\"productVersion\": \"$1\"" "$src/wails.json" || die 'не заменилась версия в wails.json'
  grep -q "WHATS_NEW_VERSION = '$1'" "$src/frontend/src/shell/dialogs/whatsnew.ts" || die 'не заменилась версия в whatsnew.ts'
}

# build_exe <версия> <qa|rel> <куда> — exe под Windows; qa — с тегом qa
build_exe() {
  say "exe $1 ($2)"
  if [ $fake = 1 ]; then blob "$3" "exe $1 $2"; return; fi
  set_version "$1"
  if [ "$2" = qa ]; then bash "$src/scripts/build.sh" --tags qa >&2; else bash "$src/scripts/build.sh" >&2; fi
  cp -- "$src/build/bin/SosninaAlbums-$1.exe" "$3"
}

# mkzip <exe> <версия> <qa|rel> — zip как у выпуска: папка «Sosnina Albums <версия>-<вид>» и exe с версией в имени
mkzip() {
  local dest="$out/zip/SosninaAlbums-$2-$3-windows-x64.zip"
  python3 -I - "$1" "Sosnina Albums $2-$3" "SosninaAlbums-$2.exe" "$dest" <<'PY'
import sys, zipfile
exe, folder, inner, dest = sys.argv[1:5]
with zipfile.ZipFile(dest, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as z:
    info = zipfile.ZipInfo(folder + "/" + inner, date_time=(2026, 10, 23, 0, 0, 0))
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = 0o644 << 16
    with open(exe, "rb") as f:
        z.writestr(info, f.read())
PY
}

# setup_name <версия> — имя установщика (то же, что SetupName в программе и setupName в release-sign)
setup_name() { printf 'SosninaAlbums-%s-windows-x64-setup.exe' "$1"; }

# make_setup <exe> <версия> <папка> — установщик; --fake — заглушка
make_setup() {
  say "установщик $2"
  mkdir -p "$3"
  if [ $fake = 1 ]; then blob "$3/$(setup_name "$2")" "setup $2 $(sha256sum "$1" | cut -c1-8)"; return; fi
  bash "$root/scripts/win/installer.sh" --exe "$1" --version "$2" --out "$3" >/dev/null
}

exe_old="$work/stage/old-qa.exe" exe_new="$work/stage/new-qa.exe" exe_rel="$work/stage/old-rel.exe"
build_exe "$OLD" qa "$exe_old"
build_exe "$NEW" qa "$exe_new"
[ $rel = 0 ] || build_exe "$OLD" rel "$exe_rel"
mkzip "$exe_old" "$OLD" qa
mkzip "$exe_new" "$NEW" qa
[ $rel = 0 ] || mkzip "$exe_rel" "$OLD" rel
make_setup "$exe_old" "$OLD" "$out/setup"
make_setup "$exe_new" "$NEW" "$out/setup"
make_setup "$exe_new" "$STALE" "$work/stale"

# ---------- лента ----------

say 'тестовые ключи и лента'
mkkey() { node "$root/scripts/release-sign.mjs" --test-key "$1" | sed -n 's/^открытый ключ[^:]*: *//p'; }
pub1=$(mkkey "$key1")
pub2=$(mkkey "$key2")
[ -n "$pub1" ] && [ -n "$pub2" ] && [ "$pub1" != "$pub2" ] || die 'не вышло создать тестовые ключи'
printf '%s\n' "$pub1" > "$out/pub.txt"

today=$(date +%F)
cat > "$work/whatsnew.ts" <<EOF
// Временный «Что нового» для тестовой ленты QA (qa-kit.sh): настоящий берётся из whatsnew.ts метки выпуска.
export const WHATS_NEW_VERSION = '$NEW'

export const WHATS_NEW: readonly string[] = [
  'Тестовая лента QA: это строка «Что нового» из подписанного файла ленты',
  'Вторая строка — проверка, что текст из ленты показывается в «О программе» и в тосте',
]
EOF

# make_feed <папка ленты> <установщик> <ключ> — лента: подписанный файл в корне, установщик в v<версия>/ под настоящим именем
make_feed() {
  local dir="$out/$1" st="$work/stage-$1" name
  name=$(setup_name "$NEW")
  rm -rf "$st"
  mkdir -p "$st" "$dir/v$NEW"
  cp -- "$2" "$st/$name"
  (cd "$st" && sha256sum "$name" > SHA256SUMS.txt)
  node "$root/scripts/release-sign.mjs" "v$NEW" --key "$3" --from "$st" --skip-rebuild \
    --whatsnew "$work/whatsnew.ts" --date "$today" >&2
  cp -- "$root/.gotmp/release-sign/v$NEW/$ASSET" "$dir/$ASSET"
  cp -- "$2" "$dir/v$NEW/$name"
}

good_setup="$out/setup/$(setup_name "$NEW")"
make_feed feed "$good_setup" "$key1"
make_feed feed-wrong-key "$good_setup" "$key2"
make_feed feed-old-setup "$work/stale/$(setup_name "$STALE")" "$key1"
# битый байт: подпись и манифест целы, не сходится только sha256 установщика
cp -r -- "$out/feed" "$out/feed-bad-byte"
python3 -I - "$out/feed-bad-byte/v$NEW/$(setup_name "$NEW")" <<'PY'
import sys
p = sys.argv[1]
b = bytearray(open(p, "rb").read())
b[len(b) // 2] ^= 0xFF
open(p, "wb").write(b)
PY

# ---------- .salbum «из новой версии» ----------

# Формат 99 больше любого нынешнего: программа читает только «голову» (версия, app) и отказывает окном «Альбом из новой версии»
printf '{"version":99,"app":"0.26.0"}\n' > "$out/albums/NewerTest-0.26.salbum"
printf '{"version":99,"app":"%s"}\n' "$NEW" > "$out/albums/NewerTest-$NEW.salbum"
printf '{"version":99}\n' > "$out/albums/NewerTest-unknown.salbum"

# ---------- файлы для ноутбука ----------

# cmdfile <путь> — stdin → файл .cmd: UTF-8 без BOM и CRLF (chcp 65001 в первой строке нужен для русских сообщений)
cmdfile() { sed 's/$/\r/' > "$1"; }

cmdfile "$out/run-feed.cmd" <<'EOF'
@echo off
chcp 65001 >nul
rem Тестовая лента обновлений на 127.0.0.1:8765. Окно не закрывать, пока идёт проверка; остановить — Ctrl+C.
rem Какая лента: feed (настоящая), feed-bad-byte, feed-wrong-key, feed-old-setup — первым аргументом.
set "FEED=%~1"
if "%FEED%"=="" set "FEED=feed"
if not exist "%~dp0%FEED%\SosninaAlbums-update.json" goto nofeed
cd /d "%~dp0%FEED%"
echo Лента %FEED%: http://127.0.0.1:8765/   ^(Ctrl+C - остановить^)
python -m http.server 8765 --bind 127.0.0.1
if errorlevel 1 py -m http.server 8765 --bind 127.0.0.1
goto :eof
:nofeed
echo Нет папки ленты "%FEED%" рядом с этим файлом. Есть: feed, feed-bad-byte, feed-wrong-key, feed-old-setup.
pause
EOF

# start_cmd <файл> <заголовок> <как найти exe> — запуск QA-сборки со стендом qa и лентой
start_cmd() {
  cmdfile "$out/$1" <<EOF
@echo off
chcp 65001 >nul
rem $2
rem Свой стенд qa: настройки в %APPDATA%\\SosninaAlbums-stand-qa, настоящие не трогаются. Лента - run-feed.cmd.
setlocal
if not exist "%~dp0pub.txt" goto nopub
set /p SA_UPDATE_KEY=<"%~dp0pub.txt"
set SA_STAND=qa
set SA_UPDATE_URL=http://127.0.0.1:8765/
$3
if not exist "%EXE%" goto noexe
start "" "%EXE%"
goto :eof
:nopub
echo Нет pub.txt рядом с этим файлом.
pause
goto :eof
:noexe
echo Не найден exe: %EXE%
echo $4
pause
EOF
}
start_cmd start-0.25.0.cmd 'Переносная QA-сборка 0.25.0 из zip (путь к exe можно дать первым аргументом).' \
  'set "EXE=%~1"
if "%EXE%"=="" set "EXE=%~dp0work\Sosnina Albums 0.25.0-qa\SosninaAlbums-0.25.0.exe"' \
  'Распакуйте zip/SosninaAlbums-0.25.0-qa-windows-x64.zip в папку work рядом с этим файлом.'
start_cmd start-installed.cmd 'Установленная копия (после setup) со стендом qa и лентой.' \
  'set "EXE=%LOCALAPPDATA%\Programs\Sosnina Albums\SosninaAlbums.exe"' \
  'Сначала поставьте программу установщиком из папки setup.'

cmdfile "$out/profile-save.cmd" <<'EOF'
@echo off
chcp 65001 >nul
rem Откладывает настоящие данные программы (настройки и кэш) в папки *.qa-save, чтобы проверка шла с чистого листа.
rem Вернуть: profile-restore.cmd. Папки стендов (SosninaAlbums-stand-qa) не трогает.
if exist "%APPDATA%\SosninaAlbums.qa-save" goto busy
if exist "%LOCALAPPDATA%\SosninaAlbums.qa-save" goto busy
if exist "%APPDATA%\SosninaAlbums" ren "%APPDATA%\SosninaAlbums" SosninaAlbums.qa-save
if exist "%LOCALAPPDATA%\SosninaAlbums" ren "%LOCALAPPDATA%\SosninaAlbums" SosninaAlbums.qa-save
echo Готово: настоящие данные отложены в SosninaAlbums.qa-save (AppData\Roaming и AppData\Local).
pause
goto :eof
:busy
echo Уже есть SosninaAlbums.qa-save - данные отложены раньше. Сначала profile-restore.cmd.
pause
EOF

cmdfile "$out/profile-restore.cmd" <<'EOF'
@echo off
chcp 65001 >nul
rem Возвращает настоящие данные, отложенные profile-save.cmd; данные, созданные при проверке, удаляются.
echo Будут удалены папки SosninaAlbums (Roaming и Local), созданные во время проверки, и возвращены отложенные.
choice /m "Продолжить"
if errorlevel 2 goto :eof
if exist "%APPDATA%\SosninaAlbums.qa-save" (
  if exist "%APPDATA%\SosninaAlbums" rmdir /s /q "%APPDATA%\SosninaAlbums"
  ren "%APPDATA%\SosninaAlbums.qa-save" SosninaAlbums
)
if exist "%LOCALAPPDATA%\SosninaAlbums.qa-save" (
  if exist "%LOCALAPPDATA%\SosninaAlbums" rmdir /s /q "%LOCALAPPDATA%\SosninaAlbums"
  ren "%LOCALAPPDATA%\SosninaAlbums.qa-save" SosninaAlbums
)
echo Готово.
pause
EOF

cmdfile "$out/stand-clean.cmd" <<'EOF'
@echo off
chcp 65001 >nul
rem Стирает данные стенда qa (настройки, кэш, скачанные обновления): следующий запуск start-*.cmd - как первый.
rem Закройте программу перед этим.
if exist "%APPDATA%\SosninaAlbums-stand-qa" rmdir /s /q "%APPDATA%\SosninaAlbums-stand-qa"
if exist "%LOCALAPPDATA%\SosninaAlbums-stand-qa" rmdir /s /q "%LOCALAPPDATA%\SosninaAlbums-stand-qa"
echo Данные стенда qa стёрты.
pause
EOF

cp -- "$root/scripts/win/installer-check.ps1" "$out/installer-check.ps1"
# В дереве ветки winqa/* (без кухни проекта) docs/qa нет: набор тогда без update.md, это не ошибка
[ ! -f "$root/docs/qa/update.md" ] || cp -- "$root/docs/qa/update.md" "$out/update.md"

# ---------- генератор альбомов ----------

# tools/qa-albums.exe — фото и Alpha/Beta для прогона в Actions (scripts/qa-albums.go, спека 118 Т1).
# Исходника в коммите нет — шаг пропускается, набор собирается и без инструмента.
mkdir -p "$out/tools"
if [ $fake = 1 ]; then
  blob "$out/tools/qa-albums.exe" 'tool qa-albums'
elif [ -f "$src/scripts/qa-albums.go" ]; then
  say 'qa-albums.exe'
  (cd "$src" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$out/tools/qa-albums.exe" scripts/qa-albums.go) >&2
else
  say 'нет scripts/qa-albums.go - qa-albums.exe пропущен'
  rmdir "$out/tools"
fi

# ---------- README ----------

cat > "$out/README.txt" <<EOF
Набор проверки Sosnina Albums 0.25 в exe (Windows). Собран $today скриптом scripts/win/qa-kit.sh.
Сценарии, галочки и что искать в журнале - update.md (рядом; то же, что docs/qa/update.md в проекте).

ЧТО КУДА КОПИРОВАТЬ
  Всю эту папку скопируйте на ноутбук, например в C:\\qa\\kit (не в Temp и не в папку, которую чистит антивирус).
  Проверьте копию: сравните хеши из kit-sha256.txt (PowerShell: Get-FileHash <файл> -Algorithm SHA256).
  Нужен Python 3 (python или py в PATH) - только для run-feed.cmd.
  Сам exe нигде не запускается из zip: распакуйте нужный zip (Проводник, "Извлечь все") в папку work рядом с файлами набора.

СОСТАВ
  zip/SosninaAlbums-$OLD-rel-windows-x64.zip   как выпуск: вопрос о связи .salbum, тур, настоящий адрес обновлений.
                                               Для блоков про связь .salbum, двойной щелчок, первый запуск и тур.
  zip/SosninaAlbums-$OLD-qa-windows-x64.zip    QA-сборка: читает SA_STAND и SA_UPDATE_URL. Переносная копия для QA-6.
  zip/SosninaAlbums-$NEW-qa-windows-x64.zip    то же, версия $NEW (для сверки).
  setup/...-$OLD-...-setup.exe                 установщик $OLD (QA-сборка): ставьте его первым.
  setup/...-$NEW-...-setup.exe                 установщик $NEW: ждёт в ленте как обновление.
  feed/                                        тестовая лента: $NEW над $OLD, подписана тестовым ключом (pub.txt).
  feed-bad-byte/ feed-wrong-key/ feed-old-setup/   испорченные ленты для QA-4.
  albums/NewerTest-*.salbum                    альбомы "из новой версии" (формат 99) для окна "Альбом из новой версии".
  run-feed.cmd [папка ленты]                   лента на 127.0.0.1:8765 (по умолчанию feed). Окно не закрывать.
  start-$OLD.cmd, start-installed.cmd          запуск QA-сборки со стендом qa и лентой (переносная / установленная).
  profile-save.cmd, profile-restore.cmd        отложить и вернуть настоящие данные программы на ноутбуке.
  stand-clean.cmd                              стереть данные стенда qa.
  installer-check.ps1                          автоматическая проверка установщика (необязательно, см. update.md).
  pub.txt                                      открытый тестовый ключ (читает start-*.cmd).

ПОРЯДОК (около 45 минут; остальное проверяет раннер GitHub Actions)
  Подробно - update.md, раздел "Ноутбук". Перед началом - там же "Подготовка ноутбука".
  0. Подготовка.
  1. Что говорит Edge, когда скачиваете установщик и архив (В1).
  2. Установка: синее окно по-русски, логотип на вашем экране, кнопки "Повтор" и "Отмена" (В2, В4, В10).
  3. Программа из архива: синее окно по-русски; старая 0.24 и "Всегда" в "Чем открыть" (А1, Б7).
  4. Обновление самой программой при включённом Kaspersky (Ж5).
  5. Удаление при включённом Kaspersky и его отчёт (Ж1, Ж5).
  6. Уборка.
  7. Windows 10 - только если есть такой компьютер: шаги 0-6 на нём.
  Итоги записывайте в update.md, раздел "Журнал прогона".

Тестовая лента и стенд работают только в QA-сборке (тег qa). Выпускная программа переменные SA_STAND, SA_UPDATE_URL, SA_UPDATE_KEY не читает.
EOF

( cd "$out" && find . -type f ! -name kit-sha256.txt -printf '%P\n' | LC_ALL=C sort | xargs -d '\n' sha256sum > kit-sha256.txt )

# ---------- самопроверка ----------

say 'самопроверка лент'
node --input-type=module -e '
const [, root, out, pub, ver] = process.argv
const { pathToFileURL } = await import("node:url")
const fs = await import("node:fs")
const crypto = await import("node:crypto")
const { openEnvelope, setupName } = await import(pathToFileURL(`${root}/scripts/release-sign.mjs`).href)
const fail = (m) => { console.error("qa-kit: самопроверка: " + m); process.exit(1) }
const read = (feed) => {
  const text = fs.readFileSync(`${out}/${feed}/SosninaAlbums-update.json`, "utf8")
  const sum = crypto.createHash("sha256").update(fs.readFileSync(`${out}/${feed}/v${ver}/${setupName(ver)}`)).digest("hex")
  return { text, sum }
}
const ok = (feed) => {
  const { text, sum } = read(feed)
  const body = openEnvelope(text, pub)
  if (body.version !== ver) fail(`${feed}: версия ${body.version}`)
  return { body, sum, file: body.files["windows-x64-setup"] }
}
const f = ok("feed")
if (f.file.sha256 !== f.sum) fail("feed: sha256 установщика не сошёлся с манифестом")
const o = ok("feed-old-setup")
if (o.file.sha256 !== o.sum) fail("feed-old-setup: sha256 установщика не сошёлся с манифестом")
if (o.sum === f.sum) fail("feed-old-setup: установщик не отличается от нового")
const b = ok("feed-bad-byte")
if (b.file.sha256 === b.sum) fail("feed-bad-byte: байт не испорчен")
if (b.file.sha256 !== f.file.sha256) fail("feed-bad-byte: манифест не тот, что у feed")
let bad = true
try { openEnvelope(read("feed-wrong-key").text, pub) } catch { bad = false }
if (bad) fail("feed-wrong-key: подпись сошлась с чужим ключом")
console.error("лента: подписи и хеши как задумано")
' -- "$root" "$out" "$pub1" "$NEW"

say "готово: $out"
du -sh -- "$out" >&2
echo "$out"
