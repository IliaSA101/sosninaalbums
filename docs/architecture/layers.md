## Слои

```
Svelte (frontend/src)            store.svelte.ts — единая точка состояния
   │  вызовы Api.X(...)  ▲  события EventsOn(...)
   ▼                     │
Wails: биндинги frontend/wailsjs (генерируются из методов App) + события
   │
App (internal/app) — тонкий слой: диалоги, автосохранение, рассылка событий, /media
   ├─ internal/album    — ядро: Editor (правки, undo/redo, отметки, Restore, View), ops_<область>.go — обёртки над функциями областей
   │     ├─ album/model   — Project, Spread, View, шаблоны, инварианты (нижний слой, типы биндингов)
   │     ├─ album/salbum  — формат .salbum: Load, Save, migrate, перепривязка
   │     ├─ album/grid, vigsheet, design, layers, backdrop, spreads, view — операции областей (функции над *model.Project)
   │     └─ album/internal/inside, edtest — доступ к ядру для тестов (только из _test.go)
   │     └─ internal/layout  — раскладка и автосборка (чистые функции)
   ├─ internal/decor    — библиотека фонов и стикеров: встроенные (decor/gen — узоры и рисунки), свои файлы, импорт, проверка SVG, /media/gen и /media/lib
   ├─ internal/library  — сканирование папок, XMP, перепривязка, кэш миниатюр
   │     └─ internal/imgfile  — один файл фото: заголовок JPEG (EXIF, ICC, XMP), пределы, декод с бюджетом памяти
   │           └─ internal/colorspace  — Adobe RGB → sRGB, встроенный профиль sRGB
   ├─ internal/paths    — граница безопасности путей: `CheckLocalPath`, буквы дисков, префиксы (без зависимостей; зовут app, prefs, album/model, album/salbum, library)
   ├─ internal/export   — план, диапазон, папка и запись JPEG, сборка из View
   │     └─ internal/render  — рисование одного разворота в картинку (фото, фон, тексты; мм → px)
   ├─ internal/backup   — резервные копии файла альбома (спека 019)
   └─ internal/osx      — Проводник, папка WebView2, проба тома (`Reachable`/`Volumes`)
```

Границы пакетов (кто кого вправе импортировать, в том числе `imgfile`, `render` и пакеты дерева `album`: области и `model` не импортируют ядро, `edtest` — только из тестов, пакеты областей снаружи дерева `album` не импортируются; правки модели мимо Editor ловит forbidigo — [Поток правки](edit-flow.md)) проверяет depguard в конфиге линтера; пределы функций — funlen (80 строк / 50 операторов) и gocognit (30), существующие нарушения перечислены там же в `exclusions` («до 023»).

`internal/backup` (`%LOCALAPPDATA%\SosninaAlbumsackups\<ключ>\ГГГГММДД-ччммсс[-N].salbum`): `Key(path)` — имя альбома и 8 hex от sha256 пути, `DraftKey` — «черновик». `Before(path, key)` копирует прежний файл байт в байт перед записью; не чаще `Every` = 10 мин (часы ушли назад — копия снимается), копия, совпавшая с самой новой, не пишется. `Force` — без интервала, `ForceKeep` — ещё и чистка не удаляет копию `keep` (восстанавливаемую). `List(key)` — копии новыми первыми (`Spreads`/`Photos` лёгким разбором, без `salbum.Load`), `Prune` — общие пределы: 10 копий на альбом (`Keep`), 30 папок (`MaxDirs`), 300 МБ (`MaxBytes`), 180 дней (`MaxAge`), папка одного альбома ≤ `MaxBytes/3`; чужие файлы, ссылки и junction не трогаются. `osx.Reachable(path)` / `osx.Volumes` — отвечает ли том: проба корня тома ждёт не дольше 1,5 с (`ProbeTimeout`), одна проба на том, ответ помнится 5 с (жив) или 15 с (мёртв), не уложившаяся проба доживает в горутине.

Картинки идут не через биндинги, а по HTTP: `Middleware` ассет-сервера отдаёт `/media/t/<id>` (миниатюра 400 px) и `/media/p/<id>` (превью 2048 px). Шрифты для текста — `/media/font/<файл>`: только имена из встроенной таблицы `text.FontFile` (белый список, путь из запроса в ФС не попадает), `font/ttf`, `max-age=86400`; список гарнитур — `State.fonts`.
