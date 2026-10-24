# Выпуск сборки

Чек-лист для каждой сборки, которая уходит пользователю (exe) или на GitHub (`main`).

## 1. Версия

- `wails.json` → `info.productVersion`: семвер, `0.X.0` — новая функциональность, `0.X.Y` — исправления. Это единственный источник: Go встраивает его через `go:embed` в `main.go` и передаёт в `app.Run` (разбор — `internal/app/version.go`), версия видна в шапке окна и пишется в `log.txt` при запуске.
- Строка `Status` в `README.md` (английская, в начале файла) — та же версия.
- В `README.md`, раздел «Возможности» — пункт новой версии «(X.Y)» (`release-check` ловит его отсутствие).
- Строка в «Что нового» ([release/whatsnew.md](release/whatsnew.md)) — для пользователя, без технических слов.
- Сверка перед меткой: `node scripts/release-check.mjs --version X.Y.Z` — версия в `wails.json`, README «Status», `whatsnew.ts` и раздел CHANGELOG, спеки версии в таблице `docs/specs/README.md`, `needs` публикации в `release.yml`, эталоны для arm64. Код выхода ≠ 0 — не ставить метку. Тесты скрипта — `node --test "scripts/*.test.mjs"` (в CI).

## 2. Проверки (всё зелёное, ноль предупреждений)

```
powershell -ExecutionPolicy Bypass -File scripts/check.ps1 -Race
```

gofmt, go vet, golangci-lint, `go test -race` (нужен gcc, см. CLAUDE.md), prettier, eslint, svelte-check, vitest, сборка фронта, размер файлов, эталон тестов. Эталон силы тестов (`testdata/test-baseline.json`, `scripts/test-baseline.mjs`): счёт «имя теста → сколько раз» (Go — с подтестами, vitest — полное имя) и покрытие Go-пакетов (не ниже эталона минус 0,5 п. п.); провал, если по имени стало меньше (перенос в другой пакет/файл под тем же именем допустим). Обновляется только `node scripts/test-baseline.mjs --write`, с причиной в сообщении коммита и с согласия владельца, если что-то уменьшилось. Перед публикацией на GitHub дополнительно — `govulncheck ./...` и `npm audit --omit=dev` в `frontend`.

QA по `docs/QA.md`: затронутые циклом пункты, консоль браузера без ошибок и предупреждений. Для релиза с изменениями формата `.salbum` — открыть альбом прошлой версии.

Заставка, снимок и ролики README (`docs/media/`, mp4-вложения GitHub) — раздел «Ролики README» ниже: решить, перезаписывать ли, до подъёма версии; запись — после него.

## 3. Сборка

```
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

Локальная сборка — проверка до публикации и выдача exe до выхода на GitHub; файлы для GitHub Release собирает Actions (§6). Скрипт делает `wails build -clean -trimpath` (с `GOTMPDIR`), переименовывает exe по версии из `wails.json` и проверяет, что в exe нет личных путей. Результат — `build/bin/SosninaAlbums-X.Y.Z.exe` (один файл, установка не нужна). WebView2: в Windows 11 он встроен; в Windows 10 при его отсутствии exe сам предложит скачать загрузчик (режим `-webview2 download` по умолчанию). Если у пользователя нет интернета — собирать с `-webview2 embed`.

## 4. Смоук собранного exe (2 минуты, руками)

1. Запустить: окно открывается, в шапке нужная версия, миниатюры грузятся (белое окно или пустые миниатюры — это CSP или WebView2).
2. Запустить второй раз — активируется уже открытое окно, второе не появляется.
3. Добавить папку, автосборка, листание раскладок, экспорт одного альбома в пустую папку, «Открыть папку».
4. Закрыть и открыть — альбом на месте.
5. `%APPDATA%\SosninaAlbums\log.txt` — строка «запуск, версия X», ошибок нет.

## 5. Передача пользователю

- exe. Опросник Марины и анкета для тестировщиков (`docs/private/make_docx.py`, `make_beta_docx.py`) — **только по отдельному запросу владельца** (владелец 2026-10-21: «не используется»); в план версии не ставить.
- Unsigned exe: SmartScreen («Подробнее» → «Выполнить в любом случае»), антивирус может спросить — инструкция в «Как установить».

## 6. Git

- Коммиты цикла — в локальной `dev`, без атрибуции Claude, автор — владелец репозитория.
- На GitHub — только `main` (один коммит на версию) и метки `vX.Y.Z`; каждый push — только после явного «да» владельца.
- В публичный `main` идут версии с новой функциональностью (`0.X.0`) и исправления `0.X.Y`, когда они нужны пользователям или чинят CI на `main` (0.9.1 — по решению владельца).
- Публикация версии на GitHub (с 0.9.1, сборка — `.github/workflows/release.yml`):
  1. До метки: `wails.json` → `X.Y.Z`, раздел `## X.Y.Z — дата` в `CHANGELOG.md` пользовательским языком (он целиком идёт в текст релиза), `frontend/src/shell/dialogs/whatsnew.ts` обновлён (версия и 4–6 строк «Что нового» для стартового окна), `THIRD_PARTY_NOTICES.txt` перегенерирован, §2–4 пройдены на локальной сборке. Ролики README — по разделу «Ролики README» ниже: перезапись решена — запись после подъёма версии, `--publish`, загрузка mp4 и их адреса в README; не решена — в README остаются прежние адреса. В обоих случаях `grep -n TODO-README-MEDIA README.md` пуст.
     1а. Эталон реплея Windows (спека 110, Т2; пока ведущая — Ubuntu): на ноутбуке Windows `go test ./internal/album -run Replay -count=1`; разошёлся — `-update`, коммит в dev (с «да» владельца на push из той сессии). Без зелёного прогона на Windows `publish-main` не делать: Ubuntu проверяет только `replay-unix.json.gz`. Эталон реплея Windows проверяется заданием «Реплей Windows» в win-qa.yml (`node scripts/winqa-push.mjs <имя> --push`, зелёное задание = зелёный прогон на Windows; ноутбук не нужен); разошёлся — задание красное и выгружает артефакт `replay-windows` (эталон, перезаписанный `-update` на Windows): `gh run download <id> -n replay-windows`, сверить с прежним `internal/album/testdata/replay.json.gz` и закоммитить в dev; ноутбук — запасной путь.
     До `publish-main` на `winqa/<имя>` зелёные оба прогона: win-qa и CI (`ci.yml` идёт и на ветках `winqa/**`, то же дерево, что уйдёт в `main`: тесты под Windows, дерево без внутренних доков, actionlint/shellcheck — находки 0.25.0 видны до публикации). Красный CI — чинить в dev, ветку отправить заново (прежний прогон отменится).
     1б. Пробная сборка выпуска на ветке (спека 110, Т4, до публикации): после `winqa-push … --push` — `gh workflow run release.yml --ref winqa/<имя> -f dry_run=true` (тем же «да», что и отправка ветки). Прогон собирает всё как выпуск (smoke Linux/Mac, test_deb.sh в jammy, установщик на Windows, упаковка), метку не ищет и черновик не создаёт; ловит то, что видит только release.yml. Зелёный — пп. 2 и 3. Раннеры Ubuntu закреплены на 24.04 (NSIS 3.09 для сверки release-sign).
  2. `node scripts/publish-main.mjs <коммит dev> -m "Sosnina Albums X.Y.Z"` (без внутренних доков, список — `scripts/publish-exclude.txt`) → `git push origin main` → дождаться зелёного CI на этом коммите.
     Служебный коммит в main без выпуска (например, workflow): `node scripts/publish-main.mjs <коммит dev> --only <путь>… -m "ci: …" [--dry]` — поверх прежнего main берёт из dev только указанные пути; отказывает при расхождении с `origin/main` и при путях из `publish-exclude.txt`; `--dry` показывает `A/M/D` и статистику, ничего не меняя.
     2а. Пробный прогон (спека 110, Т4): зелёный CI на `main` → Actions → Release → «Run workflow» на ветке `main` с галочкой `dry_run` (метку не вводить) → зелёный → метка (п. 3). Прогон собирает всё и проверяет по `github.sha` ветки и версии из `wails.json`, но метку на GitHub не ищет, а задание «Черновик релиза» пропускает. Служебный коммит в `main` между пробой и меткой — проба повторяется.
  3. `git tag vX.Y.Z main && git push origin vX.Y.Z` — Actions «Release» собирает exe Windows, `.app` macOS и бинарник Linux, задание «Архивы выпуска» (`pack`, `scripts/win/pack.py` и `pack_linux.py`, спеки 039 и 101) пакует их в два zip и один tar.gz с папкой `Sosnina Albums X.Y.Z`, рядом строит `.deb` (`scripts/linux/deb.sh`) и установщик Windows (`scripts/win/installer.sh`, NSIS; спека 111), задание «Проверка установщика Windows» (`win-check`, `scripts/win/installer-check.ps1`) ставит, обновляет поверх и удаляет его на windows-latest, после чего создаётся **черновик** релиза: `SosninaAlbums-X.Y.Z-windows-x64.zip`, `SosninaAlbums-X.Y.Z-macos-universal.zip`, `SosninaAlbums-X.Y.Z-linux-amd64.tar.gz`, `SosninaAlbums-X.Y.Z-linux-amd64.deb`, `SosninaAlbums-X.Y.Z-windows-x64-setup.exe`, `SHA256SUMS.txt` (sha256 всех файлов черновика, по нему `release-sign.mjs` сверяет установщик), `THIRD_PARTY_NOTICES.txt`, инструкции (ниже) и текст (шаблон `.github/release-notes.md` + раздел CHANGELOG), ~15 мин. Голого exe на странице нет (программа без установки — внутри zip). Внутри каждого архива: программа, «Как установить (Windows|Mac|Linux).pdf» (нет pdf — docx), «Что нового.txt» (раздел CHANGELOG), `THIRD_PARTY_NOTICES.txt`, `LICENSE` и `NOTICE.md` (п. 4 и 6 GPL: копия лицензии вместе с программой; `pack.py check` отказывает архиву без любого из них; в tar.gz ещё значок `sosninaalbums.png`, в `.deb` лицензии лежат в `/usr/share/doc/sosninaalbums/`). Вызов: `pack.py <windows|macos|linux> … --notices THIRD_PARTY_NOTICES.txt --license LICENSE --notice NOTICE.md`.
  4. Публикация черновика — оркестратор, тем же «да» владельца на выпуск (владелец 2026-10-18: «сразу публиковать Release… с моего подтверждения тут»): после зелёных заданий Release (включая «Проверка Mac» и «Проверка установщика Windows») сверить ассеты черновика (`gh release view vX.Y.Z --json assets,isDraft`) — два zip, tar.gz и .deb Linux, установщик Windows, `SHA256SUMS.txt`, THIRD_PARTY, инструкции — затем **подписать выпуск**: `node scripts/release-sign.mjs vX.Y.Z` (ключ `docs/private/release-key.pem`, вне git; скрипт сверяет установщик с `SHA256SUMS.txt`, затем **сам пересобирает установщик из метки на этой машине** (`scripts/release-rebuild.mjs`: чистая копия метки под `.gotmp/release-sign/`, `npm ci`, `scripts/build.sh --installer`, `CGO_ENABLED=0`) и сверяет с черновиком **побайтно** — не совпал, ничего не подписано; ждать ~2–4 мин (холодный кэш Go — до 6); нужны `makensis` той же версии, что в CI (`sudo apt install nsis`, Ubuntu 24.04 — 3.09) и метка в локальном репозитории; берёт «Что нового» из метки и загружает `SosninaAlbums-update.json` в черновик; `--dry` — всё, кроме загрузки, пересборка и в нём обязательна; для выпусков до 0.25.1 (установщик хранит mtime `wizard.bmp`/`header.bmp` = момент checkout в задании `pack` CI) добавить `--bmp-mtime <unix-секунды>` (или `SA_INSTALLER_BMP_MTIME`): время выставляется в копии метки перед сборкой, остальная сверка побайтно; секунды: `7z l <setup.exe>` покажет время у `$PLUGINSDIR\modern-wizard.bmp` (заголовок при `/SOLID lzma` сжат, прямо в байтах FILETIME не лежит; без 7z — распаковать LZMA или подобрать), для 0.25.0 — `1792809130`; `--skip-rebuild` — только вместе с `--from` (QA на готовых файлах, не выпуск); спека 111, design §5.4). Перезапуск workflow для уже подписанного черновика удаляет старый `SosninaAlbums-update.json` — подписать заново, и только потом `gh release edit vX.Y.Z --draft=false`. Смоук zip Windows ниже — владелец по желанию после публикации (оркестратор exe не запускает). Прежний порядок для ручной проверки: Releases → черновик: скачать zip Windows, распаковать («Извлечь все») и запустить `SosninaAlbums-X.Y.Z.exe`, смоук §4 (zip Mac — если есть на чём проверить); задание «Проверка Mac» (smoke.sh на macOS-раннере) уже прошло на итоговом zip с папкой — иначе черновика нет; к черновику приложены шесть вариантов инструкции `SosninaAlbums-X.Y.Z-install-guide[-windows|-linux|-macos-26|-macos-15|-macos-12-14]-ru.docx` (и pdf, если справился LibreOffice) — собираются из [docs/install/guide.md](install/guide.md) (устройство — спека 029); седьмой вариант `…-install-guide-macos-ru` лежит только внутри архива Mac. Ошибка упаковки (`pack.py` или `check`) — провал задания `pack`, черновика нет. При желании поправить текст → «Publish release».
  5. Раздавать exe новой версии (Марине тоже) — только после push метки `v<версия>`: ссылки «О программе» ведут на метку, до неё они открывают пустую страницу.
  - Падение задания, раздел CHANGELOG до метки, повторный прогон — [release/ci-failures.md](release/ci-failures.md).
- Лицензии (GPL-3.0-only + п. 7(b), состав файлов) — [release/license.md](release/license.md).

## Ролики README

Запись, перезапись и загрузка роликов, адреса mp4, заглушка — [release/readme-videos.md](release/readme-videos.md).

## Что нового

Пользовательские тексты версий 0.1.0–0.21.0 — [release/whatsnew.md](release/whatsnew.md).
