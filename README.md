# Sosnina Albums

[![CI](https://github.com/IliaSA101/sosninaalbums/actions/workflows/ci.yml/badge.svg)](https://github.com/IliaSA101/sosninaalbums/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Sosnina Albums** is a desktop application for laying out wedding photo albums, built for a working photographer. Pick folders with photos, drag frames onto spreads (or let auto-assembly do it), flip through layout variants, and export spreads as 300 dpi JPEGs for the print shop. Layouts are computed exactly and **never crop a photo**: every frame keeps its aspect ratio; margins and gaps are set in millimetres.

- Platform: Windows (WebView2). Stack: Go 1.26 + [Wails](https://wails.io) v2 + Svelte 5 + TypeScript.
- Status: version 0.6.1, actively developed; the UI is in Russian.
- Built together with photographer Marina Sosnina ([Sosnina films](https://sosnina-films.ru)) and used in her work.

![Sosnina Albums: spread, album strip and photo library](docs/screenshot.png)

Build from source (Go 1.26, Node.js, Wails CLI v2.16):

```
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
wails dev                                                      # dev server with hot reload
powershell -ExecutionPolicy Bypass -File scripts/build.ps1     # release exe in build/bin
powershell -ExecutionPolicy Bypass -File scripts/check.ps1     # gofmt, vet, tests, svelte-check, build
```

License: MIT, see [LICENSE](LICENSE). Third-party components: [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt). Author: [IliaSA101](https://github.com/IliaSA101).

---

## О программе

Sosnina Albums — десктопная программа для вёрстки свадебных фотоальбомов. Рассчитана на фотографа: выбираешь папки со снимками, перетаскиваешь кадры на развороты (или отдаёшь всё автосборке), листаешь варианты раскладки и выгружаешь развороты в JPEG 300 dpi для типографии. Раскладка считается точно и без обрезки фото: каждый кадр сохраняет свои пропорции, поля и промежутки заданы в миллиметрах.

Программу делает Илья Соснин (GitHub: IliaSA101) вместе с фотографом Мариной Сосниной ([Sosnina films](https://sosnina-films.ru)); Марина — первый пользователь и источник требований. Об упоминании и логотипе есть согласие.

## Возможности (0.6.1)

- Библиотека: несколько папок с подпапками, JPEG/PNG, поворот по EXIF, естественная сортировка (`img-9` раньше `img-10`), быстрые миниатюры с дисковым кэшем, пересчёт Adobe RGB в sRGB.
- Автораскладка разворота: перебор деревьев «ряды/столбцы», оценка и до 12 вариантов; переключение пробелом. Режим layflat (фото через корешок) и обычный; фото «в край» (bleed).
- Автосборка альбома из выбранных или ещё не использованных фото: программа сама делит их на развороты.
- Ручная раскладка: закреплённые места, шаблоны сеток, поворот и зеркало фото, слои, выравнивание, групповое выделение и масштаб группы, карточка X/Y/Ш/В.
- Холст — ряд разворотов, как у SmartAlbums: колесо листает ряд, Ctrl+колесо и щипок меняют масштаб; Alt+щелчок выбирает объект под курсором.
- Перетаскивание мышью: из ленты на разворот, на мини-разворот, между разворотами, обмен местами, перестановка самих разворотов.
- Текст и подписи (вшитые шрифты), фон разворота и фото на фоне, своя палитра цветов, пустой разворот по «+ Новый разворот».
- Отмена и возврат (Ctrl+Z / Ctrl+Y), автосохранение через секунду после правки, черновик при первом запуске.
- Перенос проекта на другой компьютер: перепривязка недостающих фото.

### Уровни интерфейса

Программа подстраивается под опыт пользователя: **Простой**, **Стандартный**, **Полный** и **Свой** (набор функций выбираешь сам в матрице на стартовом окне). Есть строка меню (Файл, Правка, Вид, Справка, Альбом), линейки в мм и справка по F1 для текущего уровня; на стартовом окне — «Последние» альбомы.

### Формат `.salbum`

Проект — обычный JSON-файл `.salbum`; фото остаются на месте, в проекте лежат только пути. Файл версионируется (старые форматы мигрируют при открытии, файл более новой версии программа не трогает), запись атомарная. Описание — в разделе «Формат `.salbum`» [docs/architecture/salbum-format.md](docs/architecture/salbum-format.md).

### Экспорт

Окно экспорта: диапазон разворотов, выгрузка по страницам или разворотами, качество и dpi (по умолчанию 300). Файлы `01.jpg`, `02.jpg`… с отметкой dpi, профилем sRGB и комментарием; пустые развороты можно выгрузить белыми. Ограничения на размер холста защищают от случайных гигантских файлов.

## Сборка из исходников

Нужно: Go 1.26, Node.js (LTS), Wails CLI v2.16, Windows с WebView2 (есть в Windows 11; на Windows 10 ставится отдельно).

```
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
cd frontend && npm ci && cd ..
wails dev                                                      # разработка; интерфейс ещё и на http://localhost:34115
powershell -ExecutionPolicy Bypass -File scripts/build.ps1     # выпуск: build/bin/SosninaAlbums-<версия>.exe
powershell -ExecutionPolicy Bypass -File scripts/check.ps1     # gofmt, vet, тесты, svelte-check, сборка
```

`check.ps1` понимает флаги `-Go` и `-Ui` (только Go или только фронт) и `-Race`. Порядок выпуска — [docs/RELEASE.md](docs/RELEASE.md). Те же проверки (без Wails) гоняет GitHub Actions: [.github/workflows/ci.yml](.github/workflows/ci.yml).

### Антивирус

Kaspersky (и некоторые другие) ложно срабатывает на свежесобранные Go-exe и блокирует запуск. Добавьте папку проекта в исключения. Временные файлы сборки лучше держать внутри неё: `GOTMPDIR=.gotmp` (папка в `.gitignore`).

### macOS

Собирается на самом Mac: `wails build`. Приложение не подписано, поэтому при первом запуске открывайте его через ПКМ → Открыть. Основная платформа — Windows.

## Где лежат данные пользователя

| Что                      | Где                                                                     |
| ------------------------ | ----------------------------------------------------------------------- |
| Лог, настройки, черновик | `%APPDATA%\SosninaAlbums` (`log.txt`, `config.json`, `черновик.salbum`) |
| Кэш миниатюр             | `<UserCacheDir>\SosninaAlbums\thumbs` (на Windows `%LOCALAPPDATA%`)     |

Кэш можно удалять в любой момент, он пересоздастся.

## Структура и документация

- `main.go` — окно Wails; `internal/app` — методы `App` для интерфейса; `internal/layout` — раскладка и автосборка; `internal/album` — модель, правки, файл; `internal/library` — папки, EXIF, миниатюры; `internal/export` — рендер JPEG; `frontend/src` — интерфейс по областям.
- [Структура кода](docs/STRUCTURE.md) — карта папок и файлов.
- [Архитектура](docs/ARCHITECTURE.md)
- [Сборка для macOS](docs/BUILD-macOS.md)
- [История изменений](CHANGELOG.md)
- [План развития](ROADMAP.md)

## Лицензия

MIT — см. [LICENSE](LICENSE). Встроенные шрифты — SIL Open Font License 1.1, см. [internal/text/fonts](internal/text/fonts/README.md). Остальные зависимости — [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt).

## Товарные знаки

Логотип и имя «Sosnina films» принадлежат Марине Сосниной и не входят в лицензию кода. Файлы логотипа: `frontend/src/assets/logo.png` и `build/appicon.png`. При форке замените их своими.
