# ИИ-агенты (MCP)

Спеки: чтение — `docs/specs/0.31/143-mcp-read.md` (техрешение и таблица инструментов — `143-mcp-read/design.md` §1–5, 12; безопасность — `security.md`, `security-b1.md`), сверка собеседника — 148, правки — `0.33/149-mcp-edit.md`, разрешения — `0.34/150-mcp-permissions.md`, инструменты ступени 3б — `0.34/151-mcp-tools-2.md`, фон альбома, стикеры на обложку и было/стало семейства — `0.35/154-mcp-tools-3.md` (код: `internal/app/agents_edit35.go`, `agentask_bg.go`). Здесь — как это устроено в коде. Служба читает альбом и правит его, но только замыканиями `Host`, которые заполняет фасад: `Editor` службе не дан, rev, шаги отмены и `Editor.change` остаются в `internal/album`.

Разделы:

- этот файл — канал, служба, уровни, Host, методы `App`, жизненный цикл, настройки;
- [agents-edit.md](agents-edit.md) — правки (149, 151): шлюз, Expect/rev, вопрос человеку, правила папки, события, карта файлов;
- [agents-perms.md](agents-perms.md) — разрешения по инструментам (150): «Всегда · Спрашивать · Запретить», группы, места съёмок, тихий путь папки, пауза после отказов.

QA — `docs/qa/agents.md` (143, 148, 149) и `docs/qa/agents-034.md` (150, 151).

## Схема

```
агент (Claude Desktop, Claude Code, OpenCode…)
   │ stdio: JSON-RPC построчно
   ▼
SosninaAlbums --mcp          посредник, internal/mcpbridge (тот же exe; до Wails, памяти, замка экземпляра, log.txt)
   │ строки байт в байт
   ▼
сокет ОС <папка данных>/mcp/sock     (osx.MCPSock; у стенда QA своя папка данных)
   ▼
internal/app/agents.Service          accept → mcp.Server (go-sdk), сессия на подключение
   → разрешение (150) → уровень → инструмент → Host (замыкания над App) → вид, фото, отметки, библиотека, правки
```

Не TCP: порта нет, значит нет CSRF с веб-страниц и чужого процесса на порту. Права на сокет даёт ОС (папка `mcp` 0700, сокет 0600; Windows — DACL «пользователь + SYSTEM»).

## Пакеты и файлы

- `internal/mcpbridge` — посредник. Копирует строки, пока программа доступна; когда нет — отвечает заглушкой `program_status` и `start_program` (программа закрыта / доступ выключен / слишком много подключений). `start_program` при «Запретить» в `agentPerms.tools` заглушка не показывает вовсе (`accessOn`, `start.go`; 0.34-150-2). Своих данных не читает; пределы строки 1 МБ от клиента, 4 МБ от программы, config.json читает не больше 64 КБ.
- `internal/app/agents` — служба без Wails и без `internal/album` (кроме `model`): `agents.go` (Host, Options, Service, Start/Close/State), `channel.go` + `perm_unix.go`/`perm_windows.go` (сокет, проверка папки), `peer.go` (сверка собеседника, 148), `sessions.go` (подключения, клиенты), `guard.go` (обёртка инструмента `serve`: разрешение, уровень, схема, паника, предел ответа, ошибки из списка), `errs.go`, `tools.go` (таблица инструментов), `dto*.go` (проекции и уровни), `preview.go` (JPEG-превью), `sheet.go` (лист-сетка, 151), `journal.go` (журнал вызовов), `setups.go` (тексты настройки клиентов), `mcpb.go` + `mcpb_icon.png` (сборка .mcpb только в байты), `unknowntool.go` (вызов инструмента, которого нет в этой версии, — обычный отказ с русским текстом вместо ошибки протокола). Правки и разрешения — в [agents-edit.md](agents-edit.md) и [agents-perms.md](agents-perms.md).
- `internal/contactsheet` — сборка листа-сетки из миниатюр (151): раскладка, цифры номеров, заглушки клеток, кодирование JPEG без метаданных. Пакет чистый: миниатюры ему даёт фасад (`internal/app/agents_sheet.go`).
- `internal/app/agents.go` — фасад: методы `App` для окна, события, запуск и остановка. `internal/app/agents_host.go` — `Host` на `App` (чтение), `agents_host_edit.go`, `agents_edit34.go` — замыкания правок. `internal/app/agents_mcpb.go` — `SaveClaudeExtension`. `internal/prefs/agents.go` — ключ `agents` в config.json; `internal/prefs/agentperms.go` — ключ `agentPerms` (150). Окно — `frontend/src/shell/agents`.

## Уровни доступа

`prefs.Agents.Level` (1…3, вне — зажим к 1), читается **на каждый вызов**: смена в окне действует сразу. С Host.PermLevel уровень и разрешение берутся из одного чтения config.json.

1. Устройство альбома: числа, форматы, состояния. Имён нет — ни файлов, ни папок, ни людей, ни своего оформления (вместо них «Свой фон 3», свои типографии — маски `own-<хэш>`, «Формат <хэш>»).
2. Плюс имена файлов, названия папок и альбома, тексты и подписи на листах. Таблица имён людей наружу не идёт ни на каком уровне.
3. Плюс картинки: `get_photo_preview`, `get_spread_preview` (JPEG ≤ 600 КБ, без метаданных), `get_contact_sheet` (151). Ниже уровня 3 — отказ «Картинки агенту выключены: Файл ▸ ИИ-агенты ▸ «+ картинки»», Host не зовётся.

Уровень и разрешение независимы: «Всегда» у чтения не открывает картинки на уровне 1, уровень 3 не снимает «Запретить».

Инструменты (24 у службы + `program_status` и `start_program` у посредника; список — `testdata/mcp-tools.json`, страж — `TestToolsContract`):

- чтение (12): `get_album`, `list_spreads`, `get_spread`, `list_photos`, `get_photo`, `get_photo_preview`, `get_spread_preview`, `get_contact_sheet`, `list_decor`, `list_packs`, `list_print_profiles`, `list_albums`;
- показ: `show_spread`;
- правки 149: `create_album`, `add_folder`, `await_answer` (служебный, вне разрешений), `autobuild`, `vary_layouts`, `undo_last_edit`;
- правки 151: `set_marks`, `set_cover_photo`, `set_background`, `add_sticker`, `apply_print`.

Наружу идёт то, что выбрала проекция в `dto*.go`, а не поля `model` целиком; пути, `url`, `peek`, EXIF (кроме времени и размеров) не уходят. Строки из данных чистятся от Cc/Cf и обрезаются. `list_spreads` и `get_spread` отдают `family` — семейство раскладки листа (`Host.Families`; у `grid` и листов без раскладки пусто, 151).

## Host

Единственная связь службы с программой: `agents.Host` — замыкания, список полей закреплён тестом `TestHostFields`.

- Общие: `CfgDir`, `Version`, `Sock`, `Changed`.
- Чтение: `View`, `Photos`, `Marks`, `Status`, `Decor`, `Packs`, `Prints`, `Recent`, `FontsMissing`, `Preview`, `Render`.
- Разрешения (150): `Perm`, `PermLevel` — [agents-perms.md](agents-perms.md).
- Правки 149: `Scanning`, `Busy`, `Backup`, `Ask`, `Auto`, `Vary`, `Show`, `Undo` (`hostedit.go`; без любого из них и без `Perm` правящих инструментов нет — `canEdit`).
- Правки 151: `Sheet`, `Families`, `Mark`, `Cover`, `Bg`, `Stickers`, `Print` (`hostedit34.go`, `canEdit34`).

Реализация чтения — `agents_host.go`, правок — `agents_host_edit.go`, `agents_edit34.go`, `agents_sheet.go`. Контракт: чтение ничего не пишет (ни файлов, ни настроек), не ждёт долгих операций под `a.mu`; ошибки без путей и имён файлов. Типы запросов и ответов правок — скаляры и срезы строк (`TestHostTypesScalar`).

- `View` — последний вид, отданный окну (`wire.sent`, несохранённое тоже), иначе `Editor.View()`; альбома ещё нет — `edID 0`.
- `Status` — пропавшие и нечитаемые фото; к пропавшим добавлены фото на недоступном томе и с нелокальным путём (UNC): рендер рисует их пустыми.
- `Preview` идёт тем же путём, что `/media/p` (`mediaPhoto`: `CheckLocalPath`, том, пропавшие), чтение через `imgfile` с пределом размера и бюджетом памяти. `Render` — рендер пипетки `colorpick.Picker.Sheet`; ошибки с именем файла заменяются безымянным текстом.
- `Sheet` — миниатюры 400 px из кэша ленты классом `bg` (`library.Cache.FileBgCtx`: запрос окна идёт раньше), сборка `internal/contactsheet`; клетка `ok | pending | broken`.
- Событий и фоновых запусков в Host нет: события службы идут через `Host.Changed` (состояние), `Options.OnCall` (вызов) и `Options.OnDenied` (первый отказ «Запретить»), остальное фасад делает сам.

## Методы App и события

| Метод                                          | Что делает                                                                                                     |
| ---------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `AgentsInfo()`                                 | состояние для окна и значка при открытии                                                                       |
| `SetAgents(on)`                                | `Start`/`Close`; `On` в config.json пишется только при удаче; выключение принимается и в `error` (0.31-143-42) |
| `SetAgentsLevel(level)`                        | зажим 1…3, запись в config.json                                                                                |
| `ClearAgentsLog()`                             | очищает список вызовов; счётчик «за сегодня» остаётся (0.31-143-72)                                            |
| `SetAgentPerm(tool, mode)`                     | 150: разрешение одному инструменту; значение вне `PermCan` и чужое имя — без изменений                         |
| `SetAgentPermGroup(group, mode)`               | 150: группа `read` или `edit`; запуск и показ разворота группа не трогает                                      |
| `ChooseAgentPlace()`                           | 150: «Папка со съёмками…» — окно ОС, папка становится местом (`chosen`)                                        |
| `ForgetAgentPlaces()`                          | 150: «Забыть места» — список пуст, разрешения целы                                                             |
| `AnswerAgent(id, yes)`, `AnswerAgentOther(id)` | ответ на вопрос `add` и `edit`; «Выбрать другую…» (150) — окно ОС в родителе показанного пути                  |
| `AgentAnswerCreate(id, other)`                 | ответ на вопрос `create`; `other` — «Выбрать другую…»                                                          |
| `AgentOpenPrev()`                              | 150: «Открыть прежний» из тоста тихого нового альбома                                                          |
| `AgentsBusy(on)`, `AllowAgentAsks()`           | 149: «человек занят»; «Разрешить вопросы» после третьего отказа                                                |
| `SaveClaudeExtension()`                        | файл `.mcpb` для Claude Desktop (Г7, 0.31-143-80): диалог, `CheckLocalPath`, без перезаписи; на Linux — ошибка |

`SetAgentsEdit` (флажок 149) в 0.34 убран: его место заняли разрешения.

`AgentsInfo`: `status` (`off|on|error`), `hint` (мёртвый путь exe, `XDG_CONFIG_HOME`), `error` (словами, без путей), `level`, `portable`, `setups`, `clients`, `calls` (≤ 50, новые первыми), `today`; `edit` — группа правок не вся «Запретить» (`editGroupOn`, не зеркало `agents.edit`), `editBlocked` (`""|elevated`), `asksMuted`; `perms` (строки окна «Разрешения»), `places` (места съёмок, новые первыми). Срезы никогда не `null`. `AgentCall` — плюс `edit` и `note` (`запрещено`, `без вопроса`, `да`, `отказано`, …).

События (`events.md`): `agents:state` (`AgentsInfo` целиком), `agents:call` (вызов и `today`, не чаще 4 в секунду; лишние склеиваются в одно `agents:state`, 0.31-143-73); правки — `agents:ask`, `agents:askdone`, `agents:edited`, `agents:show`, `agents:denied` ([agents-edit.md](agents-edit.md) «События»). Имя клиента (`clientInfo.name`) в интерфейсе — только текстом, без `{@html}`.

Журнал вызовов живёт в памяти службы (на диск не пишется, при выходе пропадает) и хранит только кто, что, удачно ли, итог и за сколько — ни аргументов, ни ответов.

## Жизненный цикл

- **Запуск.** `startup` → `startAgents`: в фоне (`goBG`), под `agentsMu`, `Service.StartIfOn()` — открывает сокет, только если в config.json `agents.on`. Выключено или нет папки данных — на диск не трогает. Отказ (папка без прав, чужой файл на месте сокета, сокет занят) — состояние `error` с причиной словами; программа работает.
- **Включение.** `SetAgents(true)` → `Start`: создать `mcp` 0700, проверить папку (Unix: владелец и режим; Windows: DACL), убрать только прежний `ModeSocket`, `Listen`, `Chmod 0600`. Закрывающаяся программа (`stopped`) сокет не открывает.
- **Подключение.** До 16 клиентов, лишнему — одна строка-ошибка и закрытие; `initialize` за 10 с; строка запроса ≤ 1 МБ; запись клиенту ≤ 15 с, KeepAlive 30 с. Одновременно ≤ 4 вызовов, срок вызова 10 с (рендер и лист — дольше, вопрос — плюс ожидание ответа); рендер разворота идёт по одному.
- **Выход.** `shutdown` → `stopAgents` до `flushSave`: таймер журнала снят, открытый вопрос снят (`closeAgentAsk`), `Close` закрывает слушатель и все подключения и ждёт горутины не дольше 1 с — рендер агента выход не держит.
- Паника в accept, сессии и обработчике перехватывается (`guardPanic`): подключение закрыто, программа жива; в log.txt строка без путей.

## Настройки

`config.json` → `agents: {on, level, edit}` (`prefs.AgentsOf`, `prefs.UpdateAgents` под замком config.json; пустое значение по умолчанию не пишется) и `agentPerms: {tools, places, edit}` (150; `prefs.AgentPermsOf`, `prefs.UpdateAgentPerms`). `agents.edit` в 0.34 — строгое зеркало разрешений для отката на 0.33 (true только при «Всегда» у `autobuild`, `vary_layouts`, `undo_last_edit` и не «Запретить» у папки и альбома); пишется той же записью, что `agentPerms`. Пишет **только** фасад; служба писать не вправе (правила линтера для пакета). Подробнее — [agents-perms.md](agents-perms.md) «Хранение», `settings.md`.

## Пределы и безопасность

- Чтение: `TestToolsReadOnly` сверяет rev, dirty, config.json, templates.json и файлы папки данных до и после всех веток входа.
- Все входы агента — недоверенные: схема входа с запретом лишних полей, предел страницы `limit`, предел ответа ≈ 1 МБ строки, предел картинки 600 КБ, ограничение размера и памяти до декодирования превью; лист — сетка не больше 8 × 8, сторона 768…1568, ≤ 64 фото, один рендер.
- Данные альбома в ответах — данные, а не указания: в описании каждого инструмента это сказано; в F1 — предупреждение про агента с правом команд.
- Ошибки наружу — из фиксированного списка (`errs.go`), без путей; всё прочее — «внутренняя ошибка программы», подробности в log.txt без путей (`TestAgentsLogClean`).
- Сверка собеседника (148): `peer.go`, `perm_*.go`; что осталось — design 143 §12.
- Правки — [agents-edit.md](agents-edit.md); разрешения, места и тихий путь — [agents-perms.md](agents-perms.md) «Безопасность».

## Тесты

`internal/app/agents/*_test.go` (канал, пределы, инструменты, превью, лист, уровни — методом меток, разрешения — `perms_test.go`, `perms_flow_test.go`), `internal/contactsheet`, `internal/mcpbridge`, фасад — `internal/app/agents*_test.go`, `agentplaces*_test.go`, `agentquiet*_test.go`, `agentask*_test.go`; `internal/prefs` (`agentperms`); страж событий — `events_contract_test.go`; фронт — `shell/agents/*.test.ts`.
