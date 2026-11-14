# Обновление программы (спека 111)

Спека — `docs/specs/archive/0.22-0.26/0.25/111-installer-update.md`, техрешение — `111-installer-update/design.md` (контракт, формат манифеста, безопасность). Здесь — как это устроено в коде.

## Пакеты

- `internal/update` — stdlib, `decor/fetch`, `osx`. Версии: `ParseVersion`, `Version.Compare`, `Newer` (x.y.z по 1…4 цифры; не версия — «не новее»; те же функции использует спека 112). Манифест: `Parse` (строгий разбор body подписанного конверта), `SetupName(ver)` (имя установщика только из проверенной версии). Источник: `GitHub()`, `Stand` / `Default` (`SA_UPDATE_URL`, `SA_UPDATE_KEY` — только в сборке с тегом `dev` или `qa`). Проверка: `Check(ctx, c, cur, seen, sys)` → `Result{Offer, Seen, Rollback, Blocked, Need, OSUnknown, Skipped}`; `sys` — `System{GOOS, Version, StandBad}` из `SystemOf(osx.OSVersion)` (спека 171). Скачивание: `Fetch`, `Verify`, `Clean`, `Dir`. Запуск: `ModeOf`, `CmdLine`, `Start` (Windows — отсоединённый процесс, остальные ОС — отказ).
- `internal/decor/fetch` — `Client.Signed(ctx, url, limit)`: подписанный конверт любого содержимого (подпись до разбора).
- `internal/osx/osver*.go` — `OSVersion() (string, bool)`: сырая строка версии ОС (darwin — `sysctl kern.osproductversion`, windows — `RtlGetVersion` «10.0.<сборка>» без подмены манифестом совместимости, остальные — `false`: правила для Linux нет). Разбор и проверка — `update.SystemOf` (цикл импорта).
- `internal/prefs/update.go` — ключ `update` в config.json: `off`, `checked`, `seen`, `pending`, `pendingAt`, `osTold`.
- `internal/app/updater` — служба: фазы, проверка, скачивание, установщик, события, расписание; за узким `Host` (поля-функции, замыкания над `App`), без Wails. `internal/app/update.go` — фасад: 7 методов `App` и тип `UpdateInfo` (определён от `updater.Info`, поэтому `models.ts` в пространстве `app`), `updHost()`. Сквозные тесты остаются в `app` (`update_test.go`, `update_rev_test.go`): сборщик `newUpdApp` пересобирает службу с подменами Host до первого вызова. Разрез — спека 126, Ш1.

## Ключи подписи

`update.Keys` — два открытых ключа выпуска (текущий и запасной); закрытые — у владельца вне git. Ключи выпуска заданы в `keys.go`; `release-sign.mjs --from` (QA) ими не подписывает. Тестовая пара — `helpers_test.go` (`testPair`), общая фикстура конверта — `internal/update/testdata/` (`update.json`, `update.pub`, `body.json`, `update-tampered.json`, `update-otherkey.json`): её выпускает `release-sign.mjs`, пересборка — `UPDATE_FIXTURE=1` в `scripts/release-sign.test.mjs`; Go-тест `TestManifestFixture` читает её же.

## Защита от отката

Предлагается только `Newer(version, cur)`. `config.update.seen` — наибольшая версия в подписанных файлах, виденных программой; файл ниже неё — старый выпуск подан повторно (`Rollback`): «последняя версия», строка в `log.txt`. `Checked` и `Seen` пишутся только после проверки подписи.

## События

`update:offer` (`updater.Info`, проверка при запуске нашла новее), `update:last` (`updater.Info` с `last`: версия недоступна системе), `update:state` (`updater.Info` при смене фазы), `update:progress` (`{done,total}`, не чаще 4 раз в секунду) — см. `events.md`.

## Минимальная система: `minOS` и `lastOn` (спека 171)

Поля тела манифеста; `format` остаётся 1, старые версии их пропускают. Правило для разработчика:

- Тип известного ключа (`minOS`, `lastOn`; ключи ОС — `darwin`, `windows`, `linux`, регистр строгий) не меняется никогда — только `format` 2. Новое правило — новым ключом верхнего уровня; неизвестные ключи программа пропускает без разбора и не считает в пределе.
- `minOS` — «ОС → минимальная версия системы выпуска». Ошибка в ключе своей ОС — `ErrManifest`; в ключе другой известной ОС — пропуск, имя (`minOS.windows`) в `Result.Skipped` и в log.txt (значения нет).
- `lastOn` — лестница «ОС → {порог системы → последняя версия программы для систем ниже порога}», не больше 8 ступеней (больше — отказ `release-sign`). Выбор — наименьший порог, больший версии системы: Mac 12 при порогах 13 и 14 получает версию для 12. Ступень строго старше выпуска, ключ ≤ `minOS`, чем выше ключ, тем не старше значение.
- Версия системы неизвестна (`SystemOf` отдал `""`, в том числе «10.16» macOS в режиме совместимости) — правило не применяется, предлагается обычно; строка в log.txt — только если у выпуска есть `minOS` своей ОС (`OSUnknown`).
- Откат проверяется раньше блокировки; `Seen` растёт и при блокировке (файл подписан). Ступень предлагается только страницей (`Offer.Page`), установщика у неё нет; Install и Download ей нечего делать.

Служба (`updater/os.go`): итог проверки о системе — `Info.OS` (GOOS, на стенде подменённый `SA_UPDATE_OS=darwin:12.6`, только в сборках dev/qa), `Info.Blocked` и `Info.OSNeed` (версия, которой нужна система новее, и её минимум; пусто — правило не сработало; ошибка следующей проверки их не стирает), `Info.Last` (одноразово, как `Just`: тост). Тост «последняя для вашей системы» — только при тихой проверке при запуске без предложения (`Blocked` и нет ступени): `osTold` сравнивается и пишется в той же записи `prefs.UpdateUpdate`, что `Checked` и `Seen`; другая недоступная версия — тост снова; событие `update:last` (`Info` с `last`) и поле `Info().last` — одно и то же, повтор отсекает интерфейс. Ручная проверка тоста не даёт и `osTold` не пишет. На стенде (`Client.Stand()`) `osTold` в config.json не пишется, «уже показан» — поле службы на сеанс. `OpenPage` без предложения: блокировка в сеансе или `osTold` новее текущей — `releases/tag/v<текущая>` (не версия — `/releases`), иначе `/releases/latest`; с предложением (в том числе ступенью) — его тег.
