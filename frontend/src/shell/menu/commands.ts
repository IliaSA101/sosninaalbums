// Команды программы (спека 010, design-ui §2): клавиши (App), строка меню (MenuBar) и кнопки шапки
// зовут один `run`, поэтому «Отменить» из меню и Ctrl+Z делают одно и то же.
// Здесь только вызовы store/Api; решения «что показать» — в menu.ts и keys.ts.
import { Api, store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { row } from '../../editor/spread/spreadrow.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { openNew, removeSelected, toggleSelected } from '../../panels/bins/binact'
import { binsel } from '../../panels/bins/binsel.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { keysUi } from '../hotkeys/keysui.svelte'
import { guide } from '../guide/guidestate.svelte'
import { backupsUi } from '../dialogs/backups.svelte'
import { packsUi } from '../dialogs/packs.svelte'
import { tplio } from './tplio.svelte'
import { namesUi } from '../../panels/names/names.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { straighten } from '../../editor/tilt/straighten.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { naming } from '../../panels/timeline/naming.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
import { makeVignette } from '../../panels/vignette/vigact'
import { vigDialog } from '../../panels/vignette/vigsession.svelte'
import { decor } from '../../panels/decor/decor.svelte'
import { defaultScope } from '../../panels/decor/decor'
import { COVER_ID } from '../../base/geom/sheet'
import { relinkFolder } from '../../panels/library/relink'
import { anyVaries, varyTargets } from '../../panels/gallery/variants'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { align } from '../../panels/layouts/align.svelte'
import { isAlignOp, normAlignTo } from '../../panels/layouts/align'
import { capFirst, errText } from '../../base/words/errtext'
import { effH, ZOOM_STEP } from '../../editor/zoom/zoom'
import { newDoc } from '../newdoc/newdoc.svelte'
import type { MarkPatch, RecentProject } from '../../base/core/types'

// ev — клавиатурное событие, если команду вызвала клавиша: сдвиг стрелкой и Пробел берут из него клавишу (служебные клавиши).
// arg — параметр пункта меню (путь альбома у openRecent, действие у align, опора у alignTo).
type Handler = (ev?: KeyboardEvent, arg?: string) => unknown

// Откат, сохранение и экспорт ждут, пока набранное в тексте запишется.
const afterEdit = (op: () => Promise<unknown>) => edit.idle().then(() => store.run(op()))

// Листание раскладок Пробелом: у сетки и в режиме фона раскладок нет. У виньетки (сетка по сути) листаются
// её шаблоны (решение 80).
function layoutStep(delta: number) {
  const s = store.spread
  if (!prefs.can('variants') || !s || store.onCover || bgmode.cur) return
  if (isVignette(s) || !s.grid) gallery.step('spread', delta)
}

// Зум клавишами — к центру окна холста; посреди жеста масштаб не меняется (как у колеса).
function zoomBy(factor: number) {
  if (!store.spread || canvas.busy || canvas.pressed || row.atNew) return
  row.finish() // зум не начинается посреди движения ряда
  canvas.zoomAt(factor, canvas.vp.w / 2, effH(canvas.vp) / 2)
}

async function showExportFolder() {
  try {
    await Api.ShowInExplorer()
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}

// Папка настроек и шаблонов (config.json, templates.json, log.txt): как «Открыть папку экспорта», ошибка — тостом.
async function showDataFolder() {
  try {
    await Api.ShowDataFolder()
  } catch (e) {
    store.notify(capFirst(errText(e)), 'error')
  }
}

// Последние альбомы. Путь Go принимает только из своего списка (config.recent).
// Смена альбома ждёт набранный текст и имена: иначе они уйдут уже в новый альбом (087-5). Вызов Api — внутри then.
export const openRecent = (path: string) => edit.idle().then(() => store.replaceState(Api.OpenRecent(path)))
export const openProject = () => edit.idle().then(() => store.replaceState(Api.OpenProject()))

export async function forgetRecent(path: string) {
  try {
    store.recent = (await Api.ForgetRecent(path)) as unknown as RecentProject[]
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}

async function clearRecent() {
  try {
    store.recent = (await Api.ClearRecent()) as unknown as RecentProject[]
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}

// Отметка выделенным фото ленты: свой id у каждой клавиши (074-7), событие не нужно. Тост без выделения — в ownMarks.setMarks.
const mark = (patch: MarkPatch) => () => void ownMarks.setMarks(store, patch)
const rate = (n: number) => mark({ rating: n })
const label = (l: 'red' | 'yellow' | 'green' | 'blue') => mark({ label: l, toggle: true })
const alignOp = (op: Parameters<typeof align.run>[0]) => () => align.run(op)

// Все команды каталога клавиш (internal/keymap) и команды меню с параметром; commands.contract.test сверяет с каталогом.
const HANDLERS = {
  // Файл
  new: () => newDoc.replace(() => Api.NewProject('album')),
  open: () => openProject(),
  openRecent: (_e, path) => path && openRecent(path),
  clearRecent: () => clearRecent(),
  save: () => afterEdit(() => Api.SaveProject()),
  saveAs: () => afterEdit(() => Api.SaveProjectAs()),
  backups: () => backupsUi.show(),
  packs: () => {
    if (prefs.can('decor')) void packsUi.show() // без «Фонов и стикеров» на уровне наборам нечего показывать
  },
  collect: () => edit.idle().then(() => jobs.collectShow()),
  addFolder: () => store.run(Api.ChooseFolder()),
  relink: () => relinkFolder(),
  export: () => edit.idle().then(() => jobs.exportAll()),
  showExport: () => showExportFolder(),
  showData: () => showDataFolder(),
  tplExport: () => tplio.save(),
  tplImport: () => tplio.load(),
  settings: () => modes.openSection('settings'),

  // Правка
  undo: () => edit.idle().then(() => store.runHistory(Api.Undo())),
  redo: () => edit.idle().then(() => store.runHistory(Api.Redo())),
  selectAll: () => {
    for (const p of store.visible) store.selected.add(p.id)
  },
  // В режиме фона Ctrl+A ничего не выделяет.
  selectSpread: () => {
    if (!bgmode.cur) placesel.selectAll()
  },
  clearSelection: () => {
    edit.clear()
    store.selectedCell = null
    store.selected.clear()
    spreadsel.clear()
  },
  // Выделен блок текста или подпись — убираем их, иначе выбранное фото или место (placesel.deleteSelected).
  removeCell: () => (edit.sel ? edit.removeSelected() : placesel.deleteSelected()),
  addText: () => edit.addText(),
  pasteNames: () => namesUi.show(),
  // Как кнопка в карточке разворота: один шаг SetVignette, тост с «Вернуть». Обложка и виньетка — мимо.
  makeVignette: () => {
    const s = store.spread
    if (s && canMakeVignette(s, prefs.can)) return makeVignette(s)
  },
  // F2 (094 §3.2): поле названия текущего разворота в ленте; на обложке и без разворотов — молчим, как серый пункт.
  renameSpread: () => {
    const s = store.spread
    if (!store.onCover && s && prefs.can('names')) naming.start(s.id)
  },
  // Окно «Настроить виньетку» для текущего листа; не виньетка или функции нет на уровне — молчим.
  vigSetup: () => {
    const s = store.spread
    if (isVignette(s) && prefs.can('vignette')) vigDialog.open(s.id)
  },
  // 041: выделенные в ленте развороты, иначе текущий; уровень и семейства (065) — запомненные в окне автосборки.
  varyLayouts: () => {
    if (!prefs.can('variants')) return
    const ids = varyTargets(spreadsel.ordered(store.view.spreads.map((s) => s.id)), store.spread?.id)
    if (!anyVaries(store.view.spreads, ids)) return // как серый пункт меню: сетки, виньетки, пустые — молча
    return store.varyLayouts(ids, modes.varyLevel, modes.autoFamilies)
  },
  // 048: окно «Фоны»; нет листа или функции на уровне — молчим, как серый пункт меню
  bgGallery: () => {
    const s = store.spread
    if (s && prefs.can('decor')) decor.show('bg', s.id, defaultScope(s, store.cover))
  },
  // 079: окно «Рамочки»; «Куда» — выделенные места, без выделения — весь лист. Нет листа или функции — молчим
  borderGallery: () => {
    const s = store.spread
    if (s && prefs.can('border')) decor.show('frame', s.id, 'spread')
  },
  // 048 Ф2: окно «Стикеры»; щелчок по плитке ставит стикер в центр страницы. Нет листа или функции на уровне — молчим
  addSticker: () => {
    const s = store.spread
    if (s && prefs.can('decor')) decor.show('sticker', s.id, 'spread')
  },
  // 049 Ф2: окно «Копировать оформление» с текущего листа; обложка (не источник) или функции нет на уровне — молчим
  copyDecor: () => {
    const s = store.spread
    if (s && prefs.can('decor')) gridAct.openDecor(s.id)
  },
  // 067: окно «Развороты» с текущего листа; обложка (не цель), нет листа или функции на уровне — молчим
  spreadTpl: () => {
    const s = store.spread
    if (s && s.id !== COVER_ID && prefs.can('decor')) decor.show('spread', s.id, 'spread')
  },
  flipH: () => placesel.flip('h'),
  flipV: () => placesel.flip('v'),
  gray: () => placesel.gray(),
  // Выровнять ▸: пункт меню с параметром и отдельные команды для клавиш (074-7)
  align: (_e: KeyboardEvent | undefined, op?: string) => isAlignOp(op) && align.run(op),
  alignTo: (_e: KeyboardEvent | undefined, to?: string) => align.setTo(normAlignTo(to)),
  alignLeft: alignOp('left'),
  alignHcenter: alignOp('hcenter'),
  alignRight: alignOp('right'),
  alignTop: alignOp('top'),
  alignVcenter: alignOp('vcenter'),
  alignBottom: alignOp('bottom'),
  alignDistH: alignOp('distH'),
  alignDistV: alignOp('distV'),
  turnCw: () => placesel.turn(1),
  turnCcw: () => placesel.turn(-1),
  straighten: () => straighten.startPhoto(),

  // Вид
  toggleBig: () => modes.toggleBig(),
  toggleDetails: () => modes.toggleDetails(),
  toggleSafe: () => modes.toggleSafe(),
  toggleRulers: () => modes.toggleRulers(),
  toggleMarkup: () => modes.toggleMarkup(),
  toggleSnap: () => placesel.toggleSnap(),
  levelSimple: () => prefs.set('simple'),
  levelStandard: () => prefs.set('standard'),
  levelFull: () => prefs.set('full'),
  levelCustom: () => prefs.set('custom'),
  // Окно уровней рисует LevelsDialog (читает prefs.compare).
  compareLevels: () => (prefs.compare = true),
  fitZoom: () => canvas.fit(),
  // Отметки фото ленты (004)
  markRating0: rate(0),
  markRating1: rate(1),
  markRating2: rate(2),
  markRating3: rate(3),
  markRating4: rate(4),
  markRating5: rate(5),
  markUp: mark({ step: 1 }),
  markDown: mark({ step: -1 }),
  markRed: label('red'),
  markYellow: label('yellow'),
  markGreen: label('green'),
  markBlue: label('blue'),
  markPick: mark({ flag: 1 }),
  markReject: mark({ flag: -1 }),
  markUnflag: mark({ flag: 0 }),
  // Подборки фото (083): B, Shift+B, Del ленты; «Все фото» — без клавиши (пункт «Вид»)
  binToggle: () => void toggleSelected(),
  binNew: () => openNew(),
  binRemove: () => void removeSelected(),
  binAll: () => binsel.showAll(),
  zoomIn: () => zoomBy(ZOOM_STEP),
  zoomOut: () => zoomBy(1 / ZOOM_STEP),
  allSpreads: () => modes.openSection('grid'),
  help: () => keysUi.show(),
  guide: () => guide.openFresh(),
  startWindow: () => (prefs.startOpen = true),
  // «Файл ▸ Новый проект: виньетка-лист…»: стартовое окно сразу на выборе листа (размер и «Создать» — там).
  newSheet: () => {
    newDoc.showSheet()
    prefs.startOpen = true
  },
  // Стартовое окно с раскрытым «Что нового» (StartDialog читает prefs.news).
  whatsNew: () => {
    prefs.news = true
    prefs.startOpen = true
  },

  // Только клавиши
  prevSpread: () => row.step(-1),
  nextSpread: () => row.step(1),
  prevLayout: () => layoutStep(-1),
  nextLayout: () => layoutStep(1),
  addSelected: () => store.addToCurrent(store.selectedIds),
  // Сначала закрывается галерея, и только она: выделение остаётся.
  closeGallery: () => gallery.close(),
  nudgePlace: (e) => e && placesel.nudge(e.key, e.shiftKey),
  layerFront: () => placesel.layer('front'),
  layerUp: () => placesel.layer('up'),
  layerDown: () => placesel.layer('down'),
  layerBack: () => placesel.layer('back'),
  spaceDown: (e) => {
    if (!e) return
    // Иначе кнопка в фокусе (например, ⊞) нажмётся пробелом ещё раз. Цель бывает document/window — без closest.
    if (e.target instanceof Element) e.target.closest<HTMLElement>('button')?.blur()
    canvas.spaceDown(e.shiftKey)
  },
  bgExit: () => bgmode.exit(),
  bgNudge: (e) => e && bgmode.nudge(e.key, e.shiftKey),
  bgRemove: () => bgmode.remove(),
} satisfies Record<string, Handler>

export type Command = keyof typeof HANDLERS
export const COMMANDS: Record<Command, Handler> = HANDLERS

export function run(cmd: Command, ev?: KeyboardEvent, arg?: string) {
  return COMMANDS[cmd](ev, arg)
}

// Клавиша: id пришёл из каталога Go. Которого нет в интерфейсе (каталог новее) — молча пропускаем;
// commands.contract.test не даёт такому случиться в выпуске.
export function runKey(id: string, ev: KeyboardEvent) {
  const h = (COMMANDS as Record<string, Handler>)[id]
  return h?.(ev)
}
