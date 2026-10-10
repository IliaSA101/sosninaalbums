# Структура кода: целевая карта

Черновик на согласование, вторая редакция (2026-10-08, dev `2b71e6c`). Пофайловая карта — `docs/structure-map.json` (`move-groups`, `album023`); ключи выполненных шагов 0.6.0 и 022 (`order`, `frontend`, `future`, `go`, `goFuture`, `split`, `testdata`, `move022`) — в `docs/history/structure-map-history.json`. Выполненные шаги переноса, «было/стало» и решения 2026-10-08 — в `docs/history/structure-migration.md`. Связи и пути проверены по импортам и прототипом переноса (копия в scratchpad, раздел «Перенос фронта»).

Решения владельца: фронт переносится разом, одним агентом, отдельным коммитом «перенос: …» (только `git mv` и пути); Go и `internal/project` — по областям, когда в область приходит задача; новые файлы сразу кладутся по карте. Go-корень (`internal/osx`, `internal/prefs`, `internal/export`) выносится в 0.6.0 сразу после фронта, каждый пакет — своим коммитом «перенос: …»; `internal/project` → `internal/geom` — когда придёт задача (решение 2026-10-08).

## Фронт: `frontend/src/<группа>/<область>/`

Компонент, его логика и тест — в одной папке области. С 022 области сгруппированы во второй уровень (на уровне ≤ 20 папок — решение владельца 2026-10-09; план — `docs/specs/0.8/022-structure/groups.md`, пофайловая карта — `structure-map.json` → `move-groups`). Таблица областей ниже — по именам областей, группа — в таблице групп.

| Группа    | Области                                                                        | Что                                                                     |
| --------- | ------------------------------------------------------------------------------ | ----------------------------------------------------------------------- |
| `base/`   | `core`, `prefs`, `ui`, `drag`, `geom`, `groupgeom`                             | низ: состояние и уровни, общие виджеты, перенос, зеркала Go-геометрии   |
| `editor/` | `canvas`, `spread`, `zoom`, `places`, `photo`, `text`, `backdrop`, `inspector` | холст, объекты на нём и карточка выделенного                            |
| `panels/` | `library`, `marks`, `timeline`, `layouts`, `details`                           | панели вокруг холста: фото, отметки, лента, раскладки, детали альбома   |
| `shell/`  | `menu`, `dialogs`, `export`, `boot`                                            | оболочка: меню и клавиши, окна, экспорт                                 |

В корне `src/` — `App.svelte`, `main.ts`, `style.css`, `vite-env.d.ts`, `assets/` и четыре группы. Ниже — исходная таблица областей (0.6.0 и 022), её цифры — на момент переноса.

| Папка        | Сейчас | После 0.6.0 | Что                                                                                                                                                                        |
| ------------ | ------ | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `core/`      | 24     | 17          | `store`, `types`, `media`, `undo`, `errtext`, `plural`, `startstate` и мелкие общие (022: настройки вынесены в `prefs/`)                                                    |
| `prefs/`     | —      | 7           | зеркало `internal/prefs` (022): `prefs.svelte`, `levels`, `levelcustom`, `view.svelte` (режимы вида)                                                                       |
| `ui/`        | 9      | 9           | общие виджеты: `Icon`, `Modal`, `ColorPicker` (+`palette`, `popover`), `RotateHandle` (022, из `places/`), `FixedTip` + `tip` (действие `tipped`: подсказка гаснет вместе с элементом), `Popover` (окно панели в body, 024) + `fitrow` (какие группы строки уходят в «⋯»)                         |
| `drag/`      | 7      | 7           | перенос: `drag`, `hit`, `drop`, `dropact`, `Overlay`                                                                                                                       |
| `menu/`      | 13     | 13          | `TopBar`, `MenuBar`, `LevelBadge`, `HotkeysHelp`, `commands`, `menu`, `hotkeys`, `keys`, `help`                                                                            |
| `canvas/`    | 29     | 14          | `Stage`, `Rulers` + `ruler`, `Marquee`, `canvas.svelte`, `field`, `pickunder*` (022: ряд и зум вынесены в `spread/`, `zoom/`)                                              |
| `spread/`    | —      | 7           | ряд разворотов (022): `Spread`, `SpreadBar`, `SpreadSlots`, `spreadrow` (+`spreadrow.svelte`)                                                                              |
| `zoom/`      | —      | 8           | зум (022): `ZoomBadge`, `ZoomSlider`, `zoom`, `zoomui` (+`zoomui.svelte`)                                                                                                  |
| `places/`    | 18     | 18          | места и выделение: `PlaceLayer`, `SelFrame`, `GroupFrame`, `MiniPanel`, `LayerMenu`, `layers`, `placesel`, жесты, `groupsel`                                               |
| `photo/`     | 12     | 12          | фото в ячейке: `Cell`, `CellOverlay`, `DpiMark`, `FrameControls`, `framelive`, `PhotoMenu`, `PhotoPreview`, `color`                                                        |
| `geom/`      | 20     | 14          | зеркала Go-геометрии: `textbox`, `frame`, `rotbox`, `bdbox` (из `backdrop/`), `grid`, `gridedit`, `snap` (022: `group*` вынесены в `groupgeom/`)                           |
| `groupgeom/` | —      | 8           | зеркала Go для группы (022): `groupxf`, `grouprot`, `groupaxes`, `groupframe`                                                                                              |
| `text/`      | 14     | 14          | `TextLayer`, `TextOverlay`, `TextEditor`, `TextHandles`, `InspectorText`, `edit`, `textedit`, `textgesture`, `fonts`                                                       |
| `backdrop/`  | 14     | 14          | `Backdrop*`, `InspectorBackdrop`, `bgmode` (022: `bdbox` → `geom/`)                                                                                                                        |
| `inspector/` | 13     | 13          | `Inspector`, карточки фото/места/разворота, `LayerRow`, `Fit/Snap/FlipToggle`, `EditRail`, `panels`                                                                        |
| `layouts/`   | 15     | 15          | раскладки и сетки: `LayoutBar`, `LayoutGallery`, `GridGallery`, `CopyGridDialog`, `AlignBar` + `align`, `MirrorMenu`, `gallery`                                            |
| `timeline/`  | 6      | 6–7         | `Timeline`, `SpreadName`, `JoinLine`, `AddColumn`, `SpreadMenu`, `spreadsel` (выделение разворотов) (+`TimelineChip`, если `Timeline.svelte` подойдёт к 600 строкам — 013) |
| `library/`   | 23     | 13          | `Library`, `LibraryBar`, `Thumb`, `MissingBar`, `scan`, `missing`, `relink`, `FolderMenu` + `folders` (число фото по папкам), `MoreMenu` «⋯» (024; 022: отметки вынесены в `marks/`)                                                             |
| `marks/`     | —      | 10          | отметки 004 (022): `marks`, `ownmarks`, `libfilter`, `MarkBar`, `MarkFilter`, `thumbmark`, `markbar`                                                                       |
| `details/`   | 9      | 9           | `DetailsPanel`, `AlbumInfo`, `SettingsForm`, `SpreadGrid`, `Section`, `stats`, `presets`                                                                                   |
| `dialogs/`   | 6      | 8           | `StartDialog`, `AutoAskDialog`, `LevelsDialog`, `BackupsDialog` + `backups`, `recent`, `whatsnew` (+`LevelCards`, `LevelMatrix` — 011)                                     |
| `export/`    | 5      | 5           | `ExportDialog`, `exportopts`, `exportrange`                                                                                                                                |

В корне `src/` — `App.svelte`, `main.ts`, `style.css`, `vite-env.d.ts`, `assets/`. Итого 205 файлов: 200 переезжают, 5 стоят.

Карточка области живёт в своей области (`InspectorText` → `text/`). `geom/` — сознательное исключение: геометрия для пяти областей и одно место паритета с Go. Плотнее всего межпапочные связи (кроме `core`): `places→geom` 17, `canvas→places` 13, `inspector→places` 10.

## Go

```
main.go               вход: go:embed frontend/dist и wails.json (embed не ходит вверх) → app.Run
internal/app          App: тонкие методы Wails, диалоги, события, автосохранение, /media, лог, окно; Run — опции Wails
internal/export       рендер (есть) + диапазон, план, папка, сборка из View, пустые развороты — из корня
internal/prefs        config.json (папки, уровень, «Свой», «Последние»), хранилище templates.json — из корня
internal/osx          «Показать в папке», открыть папку, папка WebView2 — из корня
internal/geom         чистая геометрия мм без Project/Editor — из internal/project (шаг 1)
internal/paths        проверка путей (`CheckLocalPath`), буквы дисков, префиксы перепривязки — из internal/project (021, 0.7.0)
internal/album        ядро: editor.go (Editor, edit/change, журнал медленных операций), editor_undo.go, editor_marks.go, editor_restore.go (Restore, LoadBase, черновик), inside.go, ops_<область>.go (обёртки; фон — в ops_layers.go), ops_view.go (View) — из internal/project (спека 023, 0.8); карта файлов — doc.go
internal/album/model  Project, Spread, места, тексты, фон, стек слоёв, отметки, View, шаблоны, пределы и зажим (project, places, backdrop, text, order, marks, viewtypes, templates, warm, limits, clamp); нижний слой, типы биндингов
internal/album/salbum формат .salbum: load, fix (migrate, починки), write, marks, relink (Load, Save, перепривязка); testdata/ — голдены v1, v3, v4-dev
internal/album/grid (ops, places, copy, choices, rotate), design, layers, backdrop, spreads, view   операции областей — функции над *model.Project; export_test.go — вход внешних тестов во внутренности
internal/album/internal/inside, edtest   доступ к ядру для тестов (edtest: GridInvariants, OrderInvariant, конструкторы; только из _test.go); пакеты областей снаружи дерева album линтер не пускает (depguard)
```

В корне останется 18 файлов кода из 25 (целиком уезжают `app_config`, `app_export_dir`, `app_export_range`, `app_export_blank`, `app_reveal_windows`, `app_reveal_other`, `app_webview`). Обёртки остаются: `app_api`, `app_design`, `app_grid` (в том числе `AlignOverlap`), `app_files`, `app_window`, `app_autosave`, методы `App` в остальных (`prefs()`, `prefsAndRecent`, `SetLevel`, `SetShowStart`, `SetCustom`; тест `TestExportAsksBlank`), `Export`, `colorReport`. Логика уезжает: экспорт, конфиг, «Последние» и набор «Своего» (`CustomLevel`, `normBase`, `customID(s)`, `sanitizeCustom`, `prefsOf` — 011 G1), хранилище шаблонов, пустые развороты (`blankSpreads`, `blankWarning` — 013 G3, `*exportPlan` → `export.Plan`), запуск проводника, `webviewDir`. Остаётся пока: `app_log`, `app_media`. `suggestDrive`, `missingOf` (тянули `project.DriveLetter/SwapDrive` — теперь `paths.*`, 021) ушли в `library.SuggestDrive`/`library.MissingOf` (`internal/library/relink.go`), `backdropFileInfo` — в `library.BackdropFile` (`backdrop.go`) на Ш6б.

**Ограничение Wails (`wails v2.16 internal/binding/binding.go:195`):** пространство имён в `models.ts` = имя Go-пакета; тип из пакета `export` дал бы `namespace export` — ошибка TS. Поэтому `ExportOptions` остаётся при `App` (сейчас `internal/app`, пространство `app`; 0.6.1 — перенос `App` из корня, `main.X` → `app.X`, `wailsjs/go/main` → `wailsjs/go/app`). Методы жизненного цикла `App` не экспортируются (Wails привязывает к фронту все экспортируемые): окно запускает функция `app.Run`. `Prefs`, `CustomLevel`, `RecentProject` станут `prefs.*`: `wails generate module`, `frontend/wailsjs` — в тот же коммит. Фронт типы из `models.ts` не импортирует (`lib/types.ts` — свои), ломается только сгенерированное `App.d.ts`.

**Контракт биндингов:** `testdata/bindings-contract.txt` (методы `App` и поля типов `models.ts`, пространство `main`/`app` = `APP`); `check.ps1` сверяет его. После изменения публичных методов `App` или типов — `node scripts/bindings-contract.mjs --write` и закоммитить.

`internal/export` импортирует `internal/album/model` (рабочий код) и `internal/album` (только тесты). Цикла нет: `album` не импортирует `export` и не должен.

**Задачи 0.6.0 в Go-областях (факт на `2b71e6c`):** 011 G1 (`SetCustom`, `prefs()`, `CustomLevel`) и 013 G3 (`app_export_blank.go` + тест) уже влиты в корень — уезжают на шагах 4 и 5 по `go`/`split`. 013 G1 (`internal/project/spread_new_test.go`) остаётся в `internal/project`.

## Правила

1. Перенос — отдельный коммит `перенос: <область>`. Фронт — только `git mv` и пути. Go — ещё `package`, импорты, квалификаторы, заглавные имена; поведение не меняется. Разрезаемый файл: `git mv` целиком в новое место, затем остающееся вырезать обратно.
2. Новые файлы — сразу по карте; до переноса фронта — в `lib/`/`components/` и строкой в `structure-map.json` (как `spreadrow`, `levelcustom`).
3. Go-корень (`osx`, `prefs`, `export`) — в 0.6.0 по «Порядку шагов». `internal/geom`, `internal/paths` и прочие будущие пакеты — целиком по области, когда в неё приходит задача: первый коммит задачи — перенос.
4. Фронт — разом, одним агентом (`sa-exec`), скриптом по `structure-map.json`, затем `check.ps1 -Ui`. Без открытых веток во `frontend/src`.
5. В папке 5–20 файлов `.go`/`.ts`/`.svelte` вместе с тестами (`testdata`, `wailsjs` не считаются) — и во фронте, и в Go; больше — делить. Фронт — новая область, не подпапка. Go — папка = пакет: делить по смысловой границе (новый пакет со своим API), не по алфавиту. Исключение — только с согласия владельца, строкой с причиной в `scripts/folder-exceptions.txt`; порог проверяют `check.ps1` (`CheckFolders`), pre-commit и хук Claude `file-size.js` (спека 022).
6. `geom/` не импортирует состояние (`*.svelte.ts`) — сейчас так. Пределы (`max-lines` 600, функция 80 строк, сложность 15; долги — списками с «долг:») и границы областей (`geom/`, `groupgeom/`, чистые `marks/`, `zoom/`, `ui/`) проверяет eslint: `frontend/eslint.config.js`.
7. На любом уровне кода — не больше 20 папок (решение владельца 2026-10-09). Считаются папки, где на любой глубине есть код (`.go`/`.ts`/`.svelte`/`.js`/`.mjs`/`.ps1`), без `wailsjs`, `testdata`, `node_modules`, `dist`; `docs/` — не код. Проверяют `CheckFolders` и pre-commit (алгоритм — `022-structure/groups.md` §2).
8. Фронт — ровно `frontend/src/<группа>/<область>/файлы`: файл кода прямо в группе и папка глубже области запрещены. В корне `src/` — только `App.svelte`, `main.ts`, `style.css`, `vite-env.d.ts`, `assets/` и группы. В группе 3–10 областей; порог п. 5 (≤ 20 файлов) — на область. Новая область — в группу по смыслу; не подходит ни одна или в группе была бы 11-я — новая группа (решает владелец). Имя области уникально во всём `src/`: в доках и промптах — `<область>/файл` без группы, полный путь — только в командах.
9. `base/` не импортирует `editor/`, `panels/`, `shell/`; долг `store`, `prefs`, `drag`, `dropact` снят (2026-10-10): основа зовёт верх через хуки (`core/hooks.ts`, `prefs.narrowed`, `drag.onStart`, `dropHooks`), верх подключает себя в `shell/boot/wire.ts` (`wire()` в `main.ts`; тестам, которым нужен верх, — в начале файла). Остался долг `FixedTip`, `geom/textbox`, `groupgeom/group*` → `text/textedit`; новых не добавлять.
10. После переноса — пути в комментариях, CLAUDE.md, `ARCHITECTURE.md`, незакрытых спеках; закрытые спеки и `docs/history` не трогать.
11. Спеки — `docs/specs/<версия>/NNN-имя.md` и `NNN-имя/` (папки версий: `0.1-0.3`, `0.4-0.5`, `0.6`, `0.7`, `0.8`); в `docs/specs` остаются `README.md` и `_template.md`. Правило «≤ 20 папок на уровне» действует и для `docs` (решение владельца 2026-10-09).

## Риски

- `core/store` верхних групп не импортирует (с 2026-10-10): события и данные верха — через `core/hooks.ts`, подключение — `shell/boot/wire.ts`.
- Хвосты: `geom/textbox`, `geom/groupxf` → `text/textedit`; `ui/FixedTip` → `timeline` (`tipLeft`) — убрать при правке.
- Go: пакеты `export`, `text`, `layout` затеняют одноимённые переменные — квалификаторы проверять.
