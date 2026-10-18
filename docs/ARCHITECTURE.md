# Архитектура

Вся логика (раскладка, правки, undo, файлы, экспорт) живёт в Go. Интерфейс на Svelte только рисует то, что прислал Go, и анимирует переходы.

Документ разрезан по разделам: читать только нужный файл из `docs/architecture/`.

- [Слои](architecture/layers.md) — пакеты Go, группы фронта, кто от кого зависит.
- [Поток правки](architecture/edit-flow.md) — `Editor.change`, `View`, undo, свои отметки фото.
- [Методы App для правок](architecture/edit-methods.md) — список методов `App`, отметки (`SetMarks`).
- [Ручная раскладка](architecture/manual-layout.md) — режим бывшей «Сетки» (спека 007).
- [События Go → UI](architecture/events.md) — какие события шлёт Go и когда.
- [Формат `.salbum`](architecture/salbum-format.md) — файл альбома, версии, миграции.
- [Перепривязка](architecture/relink.md) — недостающие фото.
- [Чтение папок и цвет](architecture/folders-color.md) — `internal/library`.
- [Библиотека](architecture/library.md) — виды, наборы, каталог, подпись ed25519, ротация ключа (спека 067).
- [Раскладка](architecture/layout.md) — `internal/layout`.
- [Автосборка](architecture/autobuild.md) — `Group`.
- [Drag-система](architecture/drag.md) — фронтенд.
- [Холст и зум](architecture/canvas-zoom.md) — фронтенд, спека 008.
- [Миниатюры](architecture/thumbs.md) — кэш и выдача превью.
- [Экспорт](architecture/export.md) — рендер JPEG, диапазон, папка.
- [Настройки программы](architecture/settings.md) — config.json, шаблоны.
- [Решения владельца по интерфейсу](architecture/owner-ux.md) — закреплено: мини-панель фото, наклон дугой; менять только по слову владельца.
