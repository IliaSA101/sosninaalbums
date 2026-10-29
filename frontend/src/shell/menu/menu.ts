// Строка меню «Файл · Правка · Разворот · Вид · Справка» (спека 010, design-ui §2). Чистая логика: что показать
// и что серое по ситуации; выполняет команду `commands.run`, рисует MenuBar.svelte.
// Клавиши берутся из реестра (base/keys) — один список на клавиши, справку и меню.
import type { Command } from './commands'
import { keymap } from '../../base/keys/keymap.svelte'
import { ALIGN_OPS, ALIGN_TO, alignBlock, alignToBlock, type AlignTo } from '../../panels/layouts/align'
import type { Zone } from '../../editor/places/groupsel'
import { ALL_LEVELS, LEVEL_TITLE, type FeatureId, type Level } from '../../base/prefs/levels'
import { recentName } from '../dialogs/recent'
import { AGENTS_MENU } from '../agents/agentstext'
import { SHARE_MENU } from '../share/sharetext'
import { shareShut } from '../share/share'
import type { RecentProject } from '../../base/core/types'
import type { IconName } from '../../base/ui/Icon.svelte'
import { ACTS, GROUP_ICON } from '../../editor/photomenu/photoacts'
import { LAYER_OPS, type LayerOp } from '../../editor/places/minipanel'
import { VIG_CLEAR_TITLE, VIG_SORT_TITLE } from '../../panels/vignette/vigmenu'
import { gateHint, type Gate } from './spreadcmds'

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
  icon?: IconName // значок слева: только у пунктов «Правки» и «Разворота» с кнопкой-двойником (владелец 23.10)
}

// Столбцы списка: значки — если есть хоть один; отметки ✓ — если есть хоть одна или значков нет (прежний вид).
// Так пункты без значка встают ровно по тексту пунктов со значком, как в MenuList.
export const hasIcons = (list: MenuItem[]): boolean => list.some((i) => !!i.icon)
export const hasMarks = (list: MenuItem[]): boolean => list.some((i) => i.checked !== undefined) || !hasIcons(list)

export type MenuId = 'file' | 'edit' | 'spread' | 'view' | 'help'

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
  sharing?: boolean // идёт своя выгрузка по ссылке (146): «Отправить по ссылке…» открывает окно хода
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
  vigSetup?: boolean // текущий лист — виньетка: «Настроить виньетку…», «Сортировать по фамилии», «Обычный разворот» доступны; нет — серое
  insertable?: boolean // 141: есть текущий лист — «Вставить разворот после»; нет — серое
  cutGate?: Gate // 141: «Разрезать перед выделенным фото»: ok, why (серое), ask (подсказка доступного); не задан — серое
  joinGate?: Gate // 141: «Склеить со следующим», как cutGate
  copyable?: boolean // на листе выделено то, что копируется (099): «Вырезать», «Копировать», «Дублировать» без этого серые
  pasteable?: boolean // буфер не пуст и есть лист (099): «Вставить» без этого серое
  layerable?: boolean // «Ручная» и выделено место, фото, текст или стикер; «Авто» — стикер (с текстом или без) без мест: слои двигать есть что; нет — серое
  missing: number // пропавших фото
  collecting?: boolean // сбор проекта в папку уже идёт (069)
  sheetDoc?: boolean // проект — отдельный лист-виньетка 031 (settings.sheet): макеты альбома недоступны (131, design §8)
  big: boolean
  details: boolean
  safe: boolean
  faceGuide?: boolean // 130: силуэт под лицо включён («Вид ▸ Силуэт под лицо»)
  markup?: boolean // подложка разметки типографии включена (066); нет — выключена
  rulers: boolean
  snap: boolean
  assocCan?: boolean // 112: выпуск на Windows — пункт «Открывать .salbum этой программой» есть; нет — пункта нет
  assocOn?: boolean // 112: .salbum сейчас открывается этой программой — галочка
  binShown?: boolean // в ленте показана подборка (083): «Показать все фото» без неё серое; не задан — серое
  alignCount?: number // выделено объектов на текущем развороте (места, фото, тексты); нет — 0
  alignTo?: AlignTo // опора «Относительно»; нет — «Выделения»
}

// Id команды каталога для пункта: align с параметром — отдельные alignLeft…alignDistV (074-7);
// команды с путём или опорой (openRecent, clearRecent, alignTo) клавиши не имеют; share (146) — вне каталога Go, пока владелец не просил клавишу.
export function keyId(cmd: Command, arg?: string): string | undefined {
  if (cmd === 'align') return arg ? 'align' + arg[0].toUpperCase() + arg.slice(1) : undefined
  return cmd === 'alignTo' || cmd === 'openRecent' || cmd === 'clearRecent' || cmd === 'share' ? undefined : cmd
}

// Клавиша команды для подписи пункта: первая из действующих (в том числе своя).
export function keysOf(cmd: Command, arg?: string): string | undefined {
  const id = keyId(cmd, arg)
  return (id && keymap.hint(id)) || undefined
}

// Команды каталога, чей пункт меню виден и не серый (ни он сам, ни его подменю): по ним клавиша
// команды без своих условий срабатывает (074-8). Пункт под серым родителем не считается.
export function enabledCmds(menus: Menu[]): Set<string> {
  const out = new Set<string>()
  const walk = (items: MenuItem[]) => {
    for (const it of items) {
      if (it.disabled) continue
      const id = it.cmd && keyId(it.cmd, it.arg)
      if (id) out.add(id)
      if (it.sub) walk(it.sub)
    }
  }
  for (const m of menus) walk(m.items)
  return out
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
  if (spec.icon) o.icon = spec.icon
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
        it.keys = keysOf(spec.cmd, spec.arg)
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
        icon: a.icon, // как у кнопок полосы «Выровнять»
        disabled: alignBlock(a.op, to, n) !== null,
        sep: a.op === 'distH', // перед распределением
      })),
      ...ALIGN_TO.map((t, i) => ({
        id: `alignTo-${t.to}`,
        label: `Относительно ${t.label.toLowerCase()}`,
        cmd: 'alignTo' as const,
        arg: t.to,
        checked: t.to === to,
        disabled: alignToBlock(t.to, n) !== null, // «Выделения» у одного — серое (спека 100 п. 4)
        hint: alignToBlock(t.to, n) ?? undefined,
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
      // Отдельный проект-лист, не виньетка в альбоме (отзыв 14.10): та — «Разворот ▸ Сделать разворот виньеткой».
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
      // 131: готовые макеты альбома и свой альбом как макет; без feature — с «Простого». У проекта-листа 031 серые (методы Go
      // отвечают ошибкой); saveLook серый ещё и без разворотов
      { id: 'looks', label: 'Макеты альбома…', cmd: 'looks', disabled: !!c.sheetDoc },
      { id: 'saveLook', label: 'Сохранить альбом как макет…', cmd: 'saveLook', disabled: !c.spreads || !!c.sheetDoc },
      // 086: свои шаблоны сеток и виньеток — в файл и из файла; без шаблонов и виньетки на уровне подменю нечего показывать
      ...(!c.can || c.can('templates') || c.can('vignette')
        ? [
            {
              id: 'tpl',
              label: 'Мои шаблоны',
              sub: [
                { id: 'tplExport', label: 'Сохранить в файл…', cmd: 'tplExport' as const },
                { id: 'tplImport', label: 'Загрузить из файла…', cmd: 'tplImport' as const },
              ],
            },
          ]
        : []),
      { id: 'showData', label: 'Открыть папку данных', cmd: 'showData' },
      // 112: связь .salbum — галочка, как «Вид ▸ Безопасная зона»; только Windows-выпуск (Go: assoc.can). Значка нет: «Файл» без значков
      ...(c.assocCan
        ? [{ id: 'assoc', label: 'Открывать .salbum этой программой', cmd: 'toggleAssoc' as const, checked: !!c.assocOn }]
        : []),
      // 143: доступ ИИ-агентов (MCP), окно — shell/agents; без feature: на всех уровнях. После связи .salbum: 112 держит свой пункт сразу за папкой данных
      { id: 'agents', label: AGENTS_MENU, cmd: 'agents' },
    ],
    [
      { id: 'addFolder', label: 'Добавить папку с фото…', cmd: 'addFolder' },
      { id: 'relink', label: 'Найти пропавшие фото…', cmd: 'relink', disabled: c.missing === 0 },
    ],
    [
      { id: 'export', label: 'Экспорт…', cmd: 'export', disabled: c.spreads === 0 && !c.exporting },
      // 146: ссылка на альбом для клиента; без разворотов и у листа-виньетки (031) серая, как «Экспорт…» без листов
      { id: 'share', label: SHARE_MENU, cmd: 'share', disabled: shareShut(c.spreads, !!c.sheetDoc, !!c.sharing) },
      { id: 'showExport', label: 'Открыть папку экспорта', cmd: 'showExport' },
    ],
    [{ id: 'settings', label: 'Настройки альбома…', cmd: 'settings' }],
  ]
}

// 099: порядок системной «Правки»; клавиши холста, поэтому в зоне ленты фото пункты серые.
function clipSpecs(c: MenuCtx): Spec[] {
  const off = c.zone !== 'canvas'
  return [
    { id: 'cutItems', label: 'Вырезать', cmd: 'cutItems', feature: 'clipboard', disabled: off || !c.copyable },
    {
      id: 'copyItems',
      label: 'Копировать',
      cmd: 'copyItems',
      icon: ACTS.clipCopy.icon,
      feature: 'clipboard',
      disabled: off || !c.copyable,
    },
    {
      id: 'pasteItems',
      label: 'Вставить',
      cmd: 'pasteItems',
      icon: ACTS.clipPaste.icon,
      feature: 'clipboard',
      disabled: off || !c.pasteable,
    },
    { id: 'duplicateItems', label: 'Дублировать', cmd: 'duplicateItems', feature: 'clipboard', disabled: off || !c.copyable },
  ]
}

// Операции над выделенным: слой, поворот, зеркало, ч/б, выравнивание — у большинства есть кнопка-двойник со значком.
function photoOpSpecs(c: MenuCtx): Spec[] {
  return [
    {
      id: 'layer',
      label: 'Слой',
      icon: ACTS.layer.icon,
      feature: 'layers',
      disabled: !c.layerable,
      sub: [
        { id: 'layerFront', label: 'На передний план', cmd: 'layerFront', icon: layerIcon('front') },
        { id: 'layerUp', label: 'Выше', cmd: 'layerUp', icon: layerIcon('up') },
        { id: 'layerDown', label: 'Ниже', cmd: 'layerDown', icon: layerIcon('down') },
        { id: 'layerBack', label: 'На задний план', cmd: 'layerBack', icon: layerIcon('back') },
      ],
    },
    {
      id: 'turn',
      label: 'Повернуть',
      icon: GROUP_ICON.turn,
      feature: 'turn',
      disabled: !c.flippable,
      sub: [
        { id: 'turnCw', label: '↻ 90°', cmd: 'turnCw', icon: ACTS.turnCw.icon },
        { id: 'turnCcw', label: '↺ 90°', cmd: 'turnCcw', icon: ACTS.turnCcw.icon },
        // 045: режим «Выпрямить по линии» для выделенного фото; функция «Наклон фото» (tilt) своего уровня
        { id: 'straighten', label: 'Выпрямить по линии', cmd: 'straighten', icon: ACTS.straighten.icon, feature: 'tilt', sep: true },
      ],
    },
    {
      id: 'flip',
      label: 'Зеркало',
      icon: GROUP_ICON.mirror,
      feature: 'flip',
      disabled: !c.flippable && !c.sticker, // фото или один стикер (048)
      sub: [
        { id: 'flipH', label: '↔ По горизонтали', cmd: 'flipH', icon: ACTS.flipH.icon },
        { id: 'flipV', label: '↕ По вертикали', cmd: 'flipV', icon: ACTS.flipV.icon },
      ],
    },
    {
      id: 'gray',
      label: c.allGray ? 'Сделать цветным' : 'Сделать ч/б',
      icon: ACTS.gray.icon,
      feature: 'adjust',
      disabled: !c.flippable,
      cmd: 'gray',
    },
    alignMenu(c),
  ]
}

// Значок операции слоя — тот же, что у кнопок «Расположение» и пунктов «⋯» (LAYER_OPS).
const layerIcon = (op: LayerOp): IconName => LAYER_OPS.filter((o) => o.op === op).map((o) => o.icon)[0]

function editSpecs(c: MenuCtx): Spec[][] {
  const onCanvas = c.zone === 'canvas'
  return [
    [
      { id: 'undo', label: 'Отменить', cmd: 'undo', icon: 'undo', disabled: !c.canUndo },
      { id: 'redo', label: 'Повторить', cmd: 'redo', icon: 'redo', disabled: !c.canRedo },
    ],
    clipSpecs(c),
    [
      {
        id: 'selectAll',
        label: 'Выделить всё',
        cmd: onCanvas ? 'selectSpread' : 'selectAll',
        feature: onCanvas ? 'groupSelect' : undefined, // в ленте Ctrl+A есть на любом уровне
        disabled: onCanvas ? !c.hasSpread : c.libraryPhotos === 0,
      },
      { id: 'clearSelection', label: 'Снять выделение', cmd: 'clearSelection', disabled: !c.anySelection },
      { id: 'removeCell', label: 'Удалить', cmd: 'removeCell', icon: ACTS.delete.icon, disabled: !c.deletable },
    ],
    photoOpSpecs(c),
  ]
}

// «Разворот» (владелец 2026-10-24, 0.26-124-1): всё, что правит лист целиком. Четыре группы по макету владельца;
// id, клавиши и условия серого прежние — меняется только меню (снимок menu.parity.test.ts).
function spreadSpecs(c: MenuCtx): Spec[][] {
  return [
    [
      { id: 'addText', label: 'Добавить текст', cmd: 'addText', icon: 'text', feature: 'text', disabled: !c.hasSpread },
      // 048 Ф2: то же окно «Фоны и стикеры», вид «Стикеры»; стикер встаёт в центр страницы
      { id: 'addSticker', label: 'Добавить стикер…', cmd: 'addSticker', icon: 'sticker', feature: 'decor', disabled: !c.hasSpread },
    ],
    [
      // 048: окно «Фоны» для текущего листа; область — весь лист (у обложки — первая сторонка)
      { id: 'bgGallery', label: 'Галерея фонов…', cmd: 'bgGallery', feature: 'decor', disabled: !c.hasSpread },
      // 079: окно «Рамочки» — рамочка на выделенные места или на весь лист; сочетания по умолчанию нет
      { id: 'borderGallery', label: 'Рамочки…', cmd: 'borderGallery', feature: 'border', disabled: !c.hasSpread },
      // 049 Ф2: «Копировать оформление…» — окно как у «Копировать раскладку…»; обложка не источник (серое)
      { id: 'copyDecor', label: 'Копировать оформление…', cmd: 'copyDecor', feature: 'decor', disabled: !c.decorable },
      // 067: окно «Развороты» — места и оформление готовых разворотов; обложка не цель (серое)
      { id: 'spreadTpl', label: 'Готовые развороты…', cmd: 'spreadTpl', feature: 'decor', disabled: !c.decorable },
    ],
    [
      // 094: поле названия текущего разворота в ленте; обложка и пустой альбом — серое (decorable)
      { id: 'renameSpread', label: 'Название разворота…', cmd: 'renameSpread', feature: 'names', disabled: !c.decorable },
      { id: 'pasteNames', label: 'Имена из списка…', cmd: 'pasteNames', feature: 'captions', disabled: !c.hasSpread },
      // 041: выделенные развороты ленты, нет выделения — текущий; серое, если менять нечего (сетки, виньетки, пустые)
      { id: 'varyLayouts', label: 'Разнообразить раскладки', cmd: 'varyLayouts', feature: 'variants', disabled: !c.varyable },
    ],
    [
      // 141: те же действия, что в правом щелчке по группе ленты; «вставить перед» остаётся там (0.30-141-3).
      // Разрез идёт перед выделенным фото (0.30-141-2), цель — текущий лист (0.30-141-5), слова постоянные (0.30-141-6)
      { id: 'insertSpread', label: 'Вставить разворот после', cmd: 'insertSpread', disabled: !c.insertable },
      {
        id: 'splitSpread',
        label: 'Разрезать перед выделенным фото',
        cmd: 'splitSpread',
        feature: 'timeline',
        disabled: !c.cutGate?.ok,
        hint: gateHint(c.cutGate),
      },
      {
        id: 'joinSpread',
        label: 'Склеить со следующим',
        cmd: 'joinSpread',
        feature: 'timeline',
        disabled: !c.joinGate?.ok,
        hint: gateHint(c.joinGate),
      },
    ],
    [
      {
        id: 'makeVignette',
        label: 'Сделать разворот виньеткой',
        cmd: 'makeVignette',
        icon: 'vignette',
        feature: 'vignette',
        disabled: !c.vigable,
      },
      { id: 'vigSetup', label: 'Настроить виньетку…', cmd: 'vigSetup', feature: 'vignette', disabled: !c.vigSetup },
      // 141: как кнопка карточки и пункт «⋯»; в правом щелчке «Обычный разворот» остаётся на любом уровне (0.30-141-4)
      {
        id: 'sortVignette',
        label: 'Сортировать по фамилии',
        cmd: 'sortVignette',
        feature: 'vignette',
        disabled: !c.vigSetup,
        hint: VIG_SORT_TITLE,
      },
      {
        id: 'clearVignette',
        label: 'Обычный разворот',
        cmd: 'clearVignette',
        feature: 'vignette',
        disabled: !c.vigSetup,
        hint: VIG_CLEAR_TITLE,
      },
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
      { id: 'big', label: 'Во весь экран', cmd: 'toggleBig', feature: 'timeline', checked: c.big },
      { id: 'details', label: 'Панель деталей', cmd: 'toggleDetails', checked: c.details },
      { id: 'safe', label: 'Безопасная зона', cmd: 'toggleSafe', checked: c.safe },
      { id: 'markup', label: 'Разметка типографии', cmd: 'toggleMarkup', checked: !!c.markup },
      { id: 'rulers', label: 'Линейки', cmd: 'toggleRulers', feature: 'rulers', checked: c.rulers },
      { id: 'snap', label: 'Привязка', cmd: 'toggleSnap', feature: 'manual', checked: c.snap },
      { id: 'faceGuide', label: 'Силуэт под лицо', cmd: 'toggleFaceGuide', feature: 'vignette', checked: !!c.faceGuide },
    ],
    [
      { id: 'fit', label: 'Вписать', cmd: 'fitZoom', disabled: !c.hasSpread },
      { id: 'zoomIn', label: 'Увеличить', cmd: 'zoomIn', feature: 'zoom', disabled: !c.hasSpread },
      { id: 'zoomOut', label: 'Уменьшить', cmd: 'zoomOut', feature: 'zoom', disabled: !c.hasSpread },
      { id: 'allSpreads', label: 'Все развороты', cmd: 'allSpreads', feature: 'allSpreads' },
    ],
    // Без клавиши по умолчанию — пункт меню нужен, чтобы её можно было назначить (074-8).
    [{ id: 'binAll', label: 'Показать все фото (без подборки)', cmd: 'binAll', feature: 'bins', disabled: !c.binShown }],
  ]
}

function helpSpecs(): Spec[][] {
  return [
    [
      { id: 'guide', label: 'Справочник функций…', cmd: 'guide' },
      { id: 'tour', label: 'Первые шаги', cmd: 'tour' },
      { id: 'videos', label: 'Видео о программе', cmd: 'videos' },
      { id: 'help', label: 'Горячие клавиши', cmd: 'help' },
      { id: 'startWindow', label: 'Стартовое окно', cmd: 'startWindow' },
      { id: 'whatsNew', label: 'Что нового', cmd: 'whatsNew' },
    ],
    // Последним пунктом и на любом уровне (101): без feature.
    [{ id: 'about', label: 'О программе…', cmd: 'about' }],
  ]
}

// У каждого меню есть пункт без feature (новый, правка, вставка разворота, уровень, «О программе»): строка не пустеет на любом уровне.
export function buildMenu(c: MenuCtx): Menu[] {
  const can = c.can ?? (() => true)
  return [
    { id: 'file', label: 'Файл', items: build(fileSpecs(c), can) },
    { id: 'edit', label: 'Правка', items: build(editSpecs(c), can) },
    // Название постоянное, и в проекте «виньетка-лист» (0.26-124-1)
    { id: 'spread', label: 'Разворот', items: build(spreadSpecs(c), can) },
    { id: 'view', label: 'Вид', items: build(viewSpecs(c), can) },
    { id: 'help', label: 'Справка', items: build(helpSpecs(), can) },
  ]
}
