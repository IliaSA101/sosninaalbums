// Package album — ядро альбома (023): Editor — открытый проект, правки с отменой, свои
// отметки, сохранение. Данные и их инварианты — подпакет model, формат файла — salbum,
// тела правок — пакеты областей. Правила и грабли — CLAUDE.md рядом; здесь только карта.
//
// Правка проекта — только через Editor. Обёртка в ops_<область>.go проверяет аргументы
// (строка «Договор») и зовёт тело правки области внутри change/try; откат, холостая правка,
// rev и шаг отмены — здесь. Новая операция — тело в пакете области, обёртка — метод Editor
// в ops_<область>.go.
//
// Файлы ядра:
//   - editor.go — Editor, NewEditor и own (своя копия проекта с проверкой путей), конвейер edit
//     (change, apply, try), Save, WriteCopy, Snapshot, Ref, View;
//     медленные операции в log.txt (slow);
//   - editor_undo.go — шаги отмены, их предел по числу и по байтам, Undo и Redo; SyncOwnNames —
//     исключение «мимо change»: имена своих шаблонов виньетки в проекте и истории без шага
//     отмены (решение 0.11 №67);
//   - editor_marks.go — свои отметки (спека 004): живут в Editor рядом с проектом и идут в тот
//     же стек отмены, но не через change; перепривязка отметок;
//   - editor_restore.go — LoadBase и Restore (резервные копии, спека 019; проект проходит own),
//     черновик (WriteDraft);
//   - inside.go — доступ тестов к состоянию Editor (internal/inside, обёртки — internal/edtest);
//   - ops_grid.go — места сетки (grid); ops_design.go — кадр, тексты, подписи (design),
//     поля подписей: переменные, имена, стиль листа (names, 030);
//   - ops_layers.go — слои, выравнивание, группа (layers) и фото на фоне (backdrop);
//   - ops_spreads.go — фото, развороты, папки, раскладка, настройки (spreads);
//   - ops_salbum.go — перепривязка и Refs (salbum); ops_view.go — сборка View (view);
//   - ops_sheets.go — листы особого вида: обложка (cover, 032) и виньетка (031 G5).
//
// Подпакеты: model (данные, инварианты, типы View), salbum (.salbum), grid, design, layers,
// backdrop, spreads, view (тела правок и сборка вида: зовутся только из ops_*.go),
// names (поля подписей 030: переменные, имя на фото, вставка списка, стиль листа),
// vignette (генератор мест виньетки, 031: чистая геометрия без проекта),
// cover (обложка 032: создание, размеры, раскладки, корешок, разметка, Reflow),
// internal/inside и internal/edtest (только для тестов).
//
// Порядок шагов Editor.edit: снимок проекта, fn; SyncGrid(false) (ошибка — откат без шага
// отмены); PruneRefs; для обычной правки Normalize, KeepLayouts, design.Rescale,
// grid.Rescale, backdrop.Rescale, cover.Reflow, FixVignettes, grid.Relay (пересборка
// нетронутых виньеток, 031; Restore их пропускает); FitTexts; SyncOrder; CheckTotals;
// сравнение JSON до и после — холостая правка не даёт ни шага отмены, ни rev. Шаги от
// SyncGrid до CheckTotals — settle (editor.go): его же зовёт сухой прогон PreviewVignette.
//
// Порядок шагов Load — salbum/load.go (decodeFile). Новое поле с данными пользователя — FormatVersion + 1
// и ветка в migrate.
package album
