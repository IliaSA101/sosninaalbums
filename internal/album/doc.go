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
//     (change, apply, try), Save, WriteCopy, Snapshot, Ref, View, Warm (прогрев раскладок с ходом, 073);
//     Close и shut — смена альбома (087-5): закрытый отклоняет правки шести входов, запись работает;
//     медленные операции и правки в log.txt (slow, slowAt: «правка: …» от 100 мс, 075-6);
//   - editor_undo.go — шаги отмены, их предел по числу и по байтам, Undo и Redo; SyncOwnNames —
//     исключение «мимо change»: имена своих шаблонов виньетки в проекте и истории без шага
//     отмены (решение 0.11 №67); LibKeys — ключи библиотеки проекта и истории (чтение);
//   - editor_marks.go — свои отметки (спека 004): живут в Editor рядом с проектом и идут в тот
//     же стек отмены, но не через change; перепривязка отметок;
//   - editor_bins.go — подборки фото (083): список в Editor, членство в отметках; шаг «рядом с
//     проектом», как у отметок (commitSide), projRev не трогает;
//   - LoadBase и Restore (резервные копии, спека 019; проект проходит own), черновик
//     (WriteDraft) — в конце editor.go;
//   - inside.go — доступ тестов к состоянию Editor (internal/inside, обёртки — internal/edtest);
//   - ops_grid.go — места сетки (grid), готовые развороты и плитка шаблона PreviewTpl (067),
//     рамочки мест ApplyBorder/SetBorder и путь картинки BorderPath (079); ops_design.go — кадр,
//     тон и прозрачность фото, тексты, подписи (design), поля подписей: переменные, имена, стиль
//     листа (names, 030), окно «Имена» — PreviewNames и PasteNames (043);
//   - ops_layers.go — слои, выравнивание, группа (layers), фото на фоне и узор (backdrop, 048);
//   - ops_spreads.go — фото, развороты, папки, раскладка, настройки (spreads);
//   - ops_auto.go — автосборка: план окна, предпросмотр, сборка и «Пересобрать» (цепочка auto,
//     живость по projRev, 078 §8), отбор похожих кадров (similar) до передачи в spreads (078-15);
//   - ops_salbum.go — перепривязка и Refs (salbum), сборка View (view);
//   - ops_sheets.go — листы особого вида: обложка (cover, 032) и виньетка (031 G5).
//   - ops_clip.go — «Копировать и вставить» (099): снимок TakeClip, вырезание CutClip (тела —
//     grid/clip.go); буфер держит App, Editor его не хранит.
//   - ops_copies.go — копии фото (098): «где стоит» WhereUsed и сведения файла FileRef; копии
//     ставят прежние правки по запросу «<ID файла>-new» (model.ResolveCopies).
//   - ops_albumbg.go — фон альбома (098): SetAlbumBg, ClearAlbumBg, ползунок SetAlbumBgFade
//     (ключ albumbg:<сессия>), «Не показывать здесь» SetNoAlbumBg, «Как в альбоме» (115):
//     SetAlbumBgToAll и SpreadBgToAlbum; тела — backdrop/album.go.
//   - ops_markup.go — разметка типографии обложки и разворотов (032, 066): выбор и «Убрать»,
//     прочитанное о файлах для вида — по слоту на разметку (guides).
//
// Подпакеты: model (данные, инварианты, типы View; model/printprof — профили типографий: разбор,
// пределы, реестр со слоями «встроенные, библиотечные, свои», 123), salbum (.salbum), grid, design, layers,
// backdrop, spreads, view (тела правок и сборка вида: зовутся только из ops_*.go),
// vigsheet (виньетка на листе 031/038: «Применить», Relay, рост, сортировка, свои шаблоны;
// вынесена из grid в 0.12 по размеру папки),
// names (поля подписей 030: переменные, имя на фото, вставка списка, стиль листа; 043: таблица
// с заголовками, учителя, столбец «Файл», имена файлов — Read, Plan, Apply),
// vignette (генератор мест виньетки, 031: чистая геометрия без проекта),
// series (серии съёмки 051: границы разворота по смене папки и паузам EXIF — чистая функция, зовёт spreads),
// similar (похожие кадры 078: какие кадры автосборка не берёт — чистая функция, зовёт ядро),
// cover (обложка 032: создание, размеры, раскладки, корешок, разметка, Reflow),
// internal/inside и internal/edtest (только для тестов).
//
// Порядок шагов Editor.edit: снимок проекта, fn; SyncGrid(false) (ошибка — откат без шага
// отмены); PruneRefs; для обычной правки Normalize, KeepLayouts, design.Rescale,
// grid.Rescale, backdrop.Rescale, cover.Reflow, FixVignettes, vigsheet.Relay (пересборка
// нетронутых виньеток, 031; Restore их пропускает); FitTexts; SyncFonts (таблица своих шрифтов,
// 082); FitStickers (пропорция стикеров, 048); SyncOrder; CheckTotals;
// сравнение JSON до и после — холостая правка не даёт ни шага отмены, ни rev. Шаги от
// SyncGrid до CheckTotals — settle (editor.go): его же зовёт сухой прогон PreviewVignette.
//
// Порядок шагов Load — salbum/load.go (decodeFile). Новое поле с данными пользователя — FormatVersion + 1
// и ветка в migrate. С v17 (098): checkHead (версия и app до полного разбора — *NewerError) →
// loadSources → loadPhotos (stat один на путь) → AlbumBg + ClampAlbumBg (до первого PruneRefs:
// фото только фона альбома не теряется) → dropNilPlaces, unionGrid → dedupe → PruneRefs → Rekey
// (номер копии «-n» из прежнего ID) → EvenFamilies (свойства файла семьи — по первой в ленте) →
// прежние шаги (ClampDesign зажимает и Asp). Первая запись поверх старого формата — копия
// «<имя> (для 0.22).salbum» (salbum.BackupName), имя для тоста — Editor.TakeBackup.
package album
