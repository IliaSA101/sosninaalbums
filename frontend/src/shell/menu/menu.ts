// Строка меню «Файл · Правка · Вид · Справка» (спека 010, design-ui §2). Чистая логика: что показать
// и что серое по ситуации; выполняет команду `commands.run`, рисует MenuBar.svelte.
// Клавиши берутся из HOTKEYS — одна таблица на клавиши, справку и меню.
import type { Command } from './commands'
import { HOTKEYS } from './hotkeys'
import { ALIGN_OPS, ALIGN_TO, alignBlock, type AlignTo } from '../../panels/layouts/align'
import type { Zone } from '../../editor/places/groupsel'
import { ALL_LEVELS, LEVEL_TITLE, type FeatureId, type Level } from '../../base/prefs/levels'
import { recentName } from '../dialogs/recent'
import type { RecentProject } from '../../base/core/types'

// Функция уровня (levels.ts): `can(id)` прячет пункт, чужой уровню.
export type Feature = FeatureId

export interface MenuItem {
  id: string
  label: string
  cmd?: Command // нет — пункт раскрывает подменю sub
  arg?: string // параметр команды (путь у «Последние»)
  keys?: string // «Ctrl+Z»
  disabled?: boolean
  checked?: boolean // пункт-переключатель: галочка слева
  sub?: MenuItem[]
  sep?: boolean // линия перед пунктом
  hint?: string // подсказка при наведении (title)
}

export type MenuId = 'file' | 'edit' | 'view' | 'help'

export interface Menu {
  id: MenuId
  label: string
  items: MenuItem[]
}

export interface MenuCtx {
  can?: (feature: Feature) => boolean
  recent?: readonly RecentProject[] // последние альбомы; не заданы — список пуст
  level?: Level // текущий уровень: галочка в «Вид ▸ Уровень»; не задан — «Полный»
  spreads: number // разворотов в альбоме
  exporting?: boolean // выгрузка идёт: «Экспорт…» открывает окно хода, даже если листов не осталось (073)
  hasSpread: boolean // есть текущий разворот
  canUndo: boolean
  canRedo: boolean
  zone: Zone // где было последнее нажатие: Ctrl+A по ней
  libraryPhotos: number // видимых фото в ленте библиотеки
  anySelection: boolean // выделено что угодно (фото в ленте, место, текст)
  deletable: boolean // есть что удалять с разворота (место, фото, текст)
  flippable: boolean // выделено фото на развороте (зеркалить, повернуть)
  allGray?: boolean // 068: все выделенные фото ч/б — пункт «Сделать цветным»; нет — «Сделать ч/б»
  sticker?: boolean // выделен один стикер (048): зеркалить можно и его
  varyable?: boolean // у «Разнообразить раскладки» есть цель: развороты «Авто» с фото среди выделенных, без выделения — текущий; нет — серое
  vigable?: boolean // текущий лист можно сделать виньеткой (обычный разворот, не обложка); нет — серое
  decorable?: boolean // у текущего листа можно копировать оформление (049 Ф2): есть лист и это не обложка; нет — серое
  vigSetup?: boolean // текущий лист — виньетка: «Настроить виньетку…» доступна; нет — серое
  layerable?: boolean // «Ручная» и выделено место, фото, текст или стикер; «Авто» — стикер (с текстом или без) без мест: слои двигать есть что; нет — серое
  missing: number // пропавших фото
  collecting?: boolean // сбор проекта в папку уже идёт (069)
  big: boolean
  details: boolean
  safe: boolean
  markup?: boolean // подложка разметки типографии включена (066); нет — выключена
  rulers: boolean
  snap: boolean
  alignCount?: number // выделено объектов на текущем развороте (места, фото, тексты); нет — 0
  alignTo?: AlignTo // опора «Относительно»; нет — «Выделения»
}

// Клавиша команды для подписи пункта: короткая запись строки таблицы, иначе её show.
export function keysOf(cmd: Command): string | undefined {
  const h = HOTKEYS.find((r) => r.action === cmd && r.combos)
  return h ? (h.menu ?? h.show) : undefined
}

interface Spec extends Omit<MenuItem, 'keys' | 'sub'> {
  feature?: Feature // уровень, с которого пункт виден
  sub?: Spec[]
}

// Необязательные поля пункта, что копируются как есть (задан — копируем; disabled только true).
function plain(spec: Spec): Partial<MenuItem> {
  const o: Partial<MenuItem> = {}
  if (spec.arg !== undefined) o.arg = spec.arg
  if (spec.hint) o.hint = spec.hint
  if (spec.disabled) o.disabled = true
  if (spec.checked !== undefined) o.checked = spec.checked
  return o
}

// Группа без пунктов (все скрыты уровнем) пропадает вместе со своей чертой.
function build(groups: Spec[][], can: (f: Feature) => boolean): MenuItem[] {
  const out: MenuItem[] = []
  for (const g of groups) {
    let first = true
    for (const spec of g) {
      if (spec.feature && !can(spec.feature)) continue
      const it: MenuItem = { id: spec.id, label: spec.label, ...plain(spec) }
      if (spec.cmd) {
        it.cmd = spec.cmd
        it.keys = keysOf(spec.cmd)
      }
      if (spec.sub) {
        it.sub = build([spec.sub], can)
        if (it.sub.length === 0 && !spec.disabled) continue
      }
      if ((first || spec.sep) && out.length > 0) it.sep = true
      first = false
      out.push(it)
    }
  }
  return out
}

// Команда выбора уровня (commands.ts).
const LEVEL_CMD: Record<Level, Command> = { simple: 'levelSimple', standard: 'levelStandard', full: 'levelFull', custom: 'levelCustom' }

// «Выровнять ▸»: 6 выравниваний, 2 распределения, опора. Серое — мало выделено (минимум — align.ts).
function alignMenu(c: MenuCtx): Spec {
  const n = c.alignCount ?? 0
  const to = c.alignTo ?? 'sel'
  return {
    id: 'align',
    label: 'Выровнять',
    feature: 'align',
    disabled: n === 0,
    sub: [
      ...ALIGN_OPS.map((a) => ({
        id: `align-${a.op}`,
        label: a.label,
        cmd: 'align' as const,
        arg: a.op,
        disabled: alignBlock(a.op, to, n) !== null,
        sep: a.op === 'distH', // перед распределением
      })),
      ...ALIGN_TO.map((t, i) => ({
        id: `alignTo-${t.to}`,
        label: `Относительно ${t.label.toLowerCase()}`,
        cmd: 'alignTo' as const,
        arg: t.to,
        checked: t.to === to,
        sep: i === 0,
      })),
    ],
  }
}

function fileSpecs(c: MenuCtx): Spec[][] {
  const recent = c.recent ?? []
  return [
    [
      { id: 'new', label: 'Новый альбом', cmd: 'new' },
      // Отдельный проект-лист, не виньетка в альбоме (отзыв 14.10): та — «Правка ▸ Сделать разворот виньеткой».
      {
        id: 'newSheet',
        label: 'Новый проект: виньетка-лист…',
        cmd: 'newSheet',
        feature: 'vignette',
        hint: 'Отдельный плакат класса 20×30 или 30×40 — без альбома',
      },
      { id: 'open', label: 'Открыть…', cmd: 'open' },
      {
        id: 'recent',
        label: 'Последние',
        disabled: recent.length === 0,
        sub: [
          ...recent.map((r, i) => ({
            id: `recent-${i}`,
            label: recentName(r),
            cmd: 'openRecent' as const,
            arg: r.path,
            disabled: !r.exists, // пропавший файл виден, но не открывается
          })),
          ...(recent.length ? [{ id: 'clearRecent', label: 'Очистить список', cmd: 'clearRecent' as const, sep: true }] : []),
        ],
      },
      { id: 'save', label: 'Сохранить', cmd: 'save' },
      { id: 'saveAs', label: 'Сохранить как…', cmd: 'saveAs' },
      // 069: копия альбома и всех его файлов в одну папку для переноса; пока сбор идёт — серое
      { id: 'collect', label: 'Собрать проект в папку…', cmd: 'collect', disabled: !!c.collecting },
      { id: 'backups', label: 'Резервные копии…', cmd: 'backups' },
      // 067 Ф2: наборы фонов, стикеров и разворотов — добавить из файла, удалить
      { id: 'packs', label: 'Наборы библиотеки…', cmd: 'packs', feature: 'decor' },
    ],
    [
      { id: 'addFolder', label: 'Добавить папку с фото…', cmd: 'addFolder' },
      { id: 'relink', label: 'Найти пропавшие фото…', cmd: 'relink', disabled: c.missing === 0 },
    ],
    [
      { id: 'export', label: 'Экспорт…', cmd: 'export', disabled: c.spreads === 0 && !c.exporting },
      { id: 'showExport', label: 'Открыть папку экспорта', cmd: 'showExport' },
    ],
    [{ id: 'settings', label: 'Настройки альбома…', cmd: 'settings' }],
  ]
}

function editSpecs(c: MenuCtx): Spec[][] {
  const onCanvas = c.zone === 'canvas'
  return [
    [
      { id: 'undo', label: 'Отменить', cmd: 'undo', disabled: !c.canUndo },
      { id: 'redo', label: 'Повторить', cmd: 'redo', disabled: !c.canRedo },
    ],
    [
      {
        id: 'selectAll',
        label: 'Выделить всё',
        cmd: onCanvas ? 'selectSpread' : 'selectAll',
        feature: onCanvas ? 'groupSelect' : undefined, // в ленте Ctrl+A есть на любом уровне
        disabled: onCanvas ? !c.hasSpread : c.libraryPhotos === 0,
      },
      { id: 'clearSelection', label: 'Снять выделение', cmd: 'clearSelection', disabled: !c.anySelection },
      { id: 'removeCell', label: 'Удалить', cmd: 'removeCell', disabled: !c.deletable },
    ],
    [
      { id: 'addText', label: 'Добавить текст', cmd: 'addText', feature: 'text', disabled: !c.hasSpread },
      // 048 Ф2: то же окно «Фоны и стикеры», вид «Стикеры»; стикер встаёт в центр страницы
      { id: 'addSticker', label: 'Добавить стикер…', cmd: 'addSticker', feature: 'decor', disabled: !c.hasSpread },
      // 048: окно «Фоны» для текущего листа; область — весь лист (у обложки — первая сторонка)
      { id: 'bgGallery', label: 'Галерея фонов…', cmd: 'bgGallery', feature: 'decor', disabled: !c.hasSpread },
      // 049 Ф2: «Копировать оформление…» — окно как у «Копировать раскладку…»; обложка не источник (серое)
      { id: 'copyDecor', label: 'Копировать оформление…', cmd: 'copyDecor', feature: 'decor', disabled: !c.decorable },
      // 067: окно «Развороты» — места и оформление готовых разворотов; обложка не цель (серое)
      { id: 'spreadTpl', label: 'Готовые развороты…', cmd: 'spreadTpl', feature: 'decor', disabled: !c.decorable },
      { id: 'pasteNames', label: 'Имена из списка…', cmd: 'pasteNames', feature: 'captions', disabled: !c.hasSpread },
      { id: 'makeVignette', label: 'Сделать разворот виньеткой', cmd: 'makeVignette', feature: 'vignette', disabled: !c.vigable },
      { id: 'vigSetup', label: 'Настроить виньетку…', cmd: 'vigSetup', feature: 'vignette', disabled: !c.vigSetup },
      // 041: выделенные развороты ленты, нет выделения — текущий; серое, если менять нечего (сетки, виньетки, пустые)
      { id: 'varyLayouts', label: 'Разнообразить раскладки', cmd: 'varyLayouts', feature: 'variants', disabled: !c.varyable },
      {
        id: 'layer',
        label: 'Слой',
        feature: 'layers',
        disabled: !c.layerable,
        sub: [
          { id: 'layerFront', label: 'На передний план', cmd: 'layerFront' },
          { id: 'layerUp', label: 'Выше', cmd: 'layerUp' },
          { id: 'layerDown', label: 'Ниже', cmd: 'layerDown' },
          { id: 'layerBack', label: 'На задний план', cmd: 'layerBack' },
        ],
      },
      {
        id: 'turn',
        label: 'Повернуть',
        feature: 'turn',
        disabled: !c.flippable,
        sub: [
          { id: 'turnCw', label: '↻ 90°', cmd: 'turnCw' },
          { id: 'turnCcw', label: '↺ 90°', cmd: 'turnCcw' },
          // 045: режим «Выпрямить по линии» для выделенного фото; функция «Наклон фото» (tilt) своего уровня
          { id: 'straighten', label: 'Выпрямить по линии', cmd: 'straighten', feature: 'tilt', sep: true },
        ],
      },
      {
        id: 'flip',
        label: 'Зеркало',
        feature: 'flip',
        disabled: !c.flippable && !c.sticker, // фото или один стикер (048)
        sub: [
          { id: 'flipH', label: '↔ По горизонтали', cmd: 'flipH' },
          { id: 'flipV', label: '↕ По вертикали', cmd: 'flipV' },
        ],
      },
      {
        id: 'gray',
        label: c.allGray ? 'Сделать цветным' : 'Сделать ч/б',
        feature: 'adjust',
        disabled: !c.flippable,
        cmd: 'gray',
      },
      alignMenu(c),
    ],
  ]
}

function viewSpecs(c: MenuCtx): Spec[][] {
  const level = c.level ?? 'full'
  return [
    [
      {
        id: 'level',
        label: 'Уровень',
        sub: ALL_LEVELS.map((l) => ({ id: `level-${l}`, label: LEVEL_TITLE[l], cmd: LEVEL_CMD[l], checked: l === level })),
      },
      { id: 'compareLevels', label: 'Уровни и функции…', cmd: 'compareLevels' },
    ],
    [
      { id: 'big', label: 'Крупный просмотр', cmd: 'toggleBig', feature: 'timeline', checked: c.big },
      { id: 'details', label: 'Панель деталей', cmd: 'toggleDetails', checked: c.details },
      { id: 'safe', label: 'Безопасная зона', cmd: 'toggleSafe', checked: c.safe },
      { id: 'markup', label: 'Разметка типографии', cmd: 'toggleMarkup', checked: !!c.markup },
      { id: 'rulers', label: 'Линейки', cmd: 'toggleRulers', feature: 'rulers', checked: c.rulers },
      { id: 'snap', label: 'Привязка', cmd: 'toggleSnap', feature: 'manual', checked: c.snap },
    ],
    [
      { id: 'fit', label: 'Вписать', cmd: 'fitZoom', disabled: !c.hasSpread },
      { id: 'zoomIn', label: 'Увеличить', cmd: 'zoomIn', feature: 'zoom', disabled: !c.hasSpread },
      { id: 'zoomOut', label: 'Уменьшить', cmd: 'zoomOut', feature: 'zoom', disabled: !c.hasSpread },
      { id: 'allSpreads', label: 'Все развороты', cmd: 'allSpreads', feature: 'allSpreads' },
    ],
  ]
}

function helpSpecs(): Spec[][] {
  return [
    [
      { id: 'guide', label: 'Справочник функций…', cmd: 'guide' },
      { id: 'help', label: 'Горячие клавиши', cmd: 'help' },
      { id: 'startWindow', label: 'Стартовое окно', cmd: 'startWindow' },
      { id: 'whatsNew', label: 'Что нового', cmd: 'whatsNew' },
    ],
  ]
}

export function buildMenu(c: MenuCtx): Menu[] {
  const can = c.can ?? (() => true)
  return [
    { id: 'file', label: 'Файл', items: build(fileSpecs(c), can) },
    { id: 'edit', label: 'Правка', items: build(editSpecs(c), can) },
    { id: 'view', label: 'Вид', items: build(viewSpecs(c), can) },
    { id: 'help', label: 'Справка', items: build(helpSpecs(), can) },
  ]
}
