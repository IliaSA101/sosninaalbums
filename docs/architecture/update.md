# Обновление программы (спека 111)

Спека — `docs/specs/0.25/111-installer-update.md`, техрешение — `111-installer-update/design.md` (контракт, формат манифеста, безопасность). Здесь — как это устроено в коде.

## Пакеты

- `internal/update` — stdlib, `decor/fetch`, `osx`. Версии: `ParseVersion`, `Version.Compare`, `Newer` (x.y.z по 1…4 цифры; не версия — «не новее»; те же функции использует спека 112). Манифест: `Parse` (строгий разбор body подписанного конверта), `SetupName(ver)` (имя установщика только из проверенной версии). Источник: `GitHub()`, `Stand` / `Default` (`SA_UPDATE_URL`, `SA_UPDATE_KEY` — только в сборке с тегом `dev`). Проверка: `Check(ctx, c, cur, seen)` → `Result{Offer, Seen, Rollback}`. Скачивание: `Fetch`, `Verify`, `Clean`, `Dir`. Запуск: `ModeOf`, `CmdLine`, `Start` (Windows — отсоединённый процесс, остальные ОС — отказ).
- `internal/decor/fetch` — `Client.Signed(ctx, url, limit)`: подписанный конверт любого содержимого (подпись до разбора).
- `internal/prefs/update.go` — ключ `update` в config.json: `off`, `checked`, `seen`, `pending`, `pendingAt`.
- `internal/app/updater` — служба: фазы, проверка, скачивание, установщик, события, расписание; за узким `Host` (поля-функции, замыкания над `App`), без Wails. `internal/app/update.go` — фасад: 7 методов `App` и тип `UpdateInfo` (определён от `updater.Info`, поэтому `models.ts` в пространстве `app`), `updHost()`. Сквозные тесты остаются в `app` (`update_test.go`, `update_rev_test.go`): сборщик `newUpdApp` пересобирает службу с подменами Host до первого вызова. Разрез — спека 126, Ш1.

## Ключи подписи

`update.Keys` — два открытых ключа выпуска (текущий и запасной); закрытые — у владельца вне git. Пока в `keys.go` пустые строки: подпись не сходится ни с чем, предложений нет. Тестовая пара — `helpers_test.go` (`testPair`), общая фикстура конверта — `internal/update/testdata/` (`update.json`, `update.pub`, `body.json`, `update-tampered.json`, `update-otherkey.json`): её выпускает `release-sign.mjs`, пересборка — `UPDATE_FIXTURE=1` в `scripts/release-sign.test.mjs`; Go-тест `TestManifestFixture` читает её же.

## Защита от отката

Предлагается только `Newer(version, cur)`. `config.update.seen` — наибольшая версия в подписанных файлах, виденных программой; файл ниже неё — старый выпуск подан повторно (`Rollback`): «последняя версия», строка в `log.txt`. `Checked` и `Seen` пишутся только после проверки подписи.

## События

`update:offer` (`updater.Info`, проверка при запуске нашла новее), `update:state` (`updater.Info` при смене фазы), `update:progress` (`{done,total}`, не чаще 4 раз в секунду) — см. `events.md`.
