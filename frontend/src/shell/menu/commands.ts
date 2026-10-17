// Команды программы (спека 010, design-ui §2): клавиши (App), строка меню (MenuBar) и кнопки шапки
// зовут один `run`, поэтому «Отменить» из меню и Ctrl+Z делают одно и то же.
// Здесь только вызовы store/Api; решения «что показать» — в menu.ts и keys.ts.
import { Api, store } from '../../base/core/store.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { row } from '../../editor/spread/spreadrow.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { help } from './help.svelte'
import { guide } from '../guide/guidestate.svelte'
import { backupsUi } from '../dialogs/backups.svelte'
import { namesUi } from '../../panels/names/names.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { straighten } from '../../editor/photo/straighten.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
import { makeVignette } from '../../panels/vignette/vigact'
import { vigDialog } from '../../panels/vignette/vigsession.svelte'
import { decor } from '../../panels/decor/decor.svelte'
import { defaultScope } from '../../panels/decor/decor'
import { relinkFolder } from '../../panels/library/relink'
import { anyVaries, varyTargets } from '../../panels/gallery/variants'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { align } from '../../panels/layouts/align.svelte'
import { isAlignOp, normAlignTo } from '../../panels/layouts/align'
import { errText } from '../../base/words/errtext'
import { effH, ZOOM_STEP } from '../../editor/zoom/zoom'
import { newDoc } from '../newdoc/newdoc.svelte'
import type { KeyAction } from './hotkeys'
import type { Label, MarkPatch, RecentProject } from '../../base/core/types'

// Команды без клавиши: только меню (и кнопки).
export type MenuCommand =
  | 'addFolder'
  | 'relink'
  | 'backups'
  | 'showExport'
  | 'settings'
  | 'addText'
  | 'pasteNames'
  | 'flipH'
  | 'flipV'
  | 'align'
  | 'alignTo'
  | 'turnCw'
  | 'turnCcw'
  | 'straighten'
  | 'toggleSnap'
  | 'allSpreads'
  | 'levelSimple'
  | 'levelStandard'
  | 'levelFull'
  | 'levelCustom'
  | 'compareLevels'
  | 'openRecent'
  | 'clearRecent'
  | 'startWindow'
  | 'whatsNew'
  | 'newSheet'
  | 'makeVignette'
  | 'vigSetup'
  | 'varyLayouts'
  | 'bgGallery'
  | 'addSticker'
  | 'copyDecor'
export type Command = KeyAction | MenuCommand

// ev — клавиатурное событие, если команду вызвала клавиша: сдвиг стрелкой и Пробел берут из него клавишу.
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

// Последние альбомы. Путь Go принимает только из своего списка (config.recent).
export const openRecent = (path: string) => store.replaceState(Api.OpenRecent(path))

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

// Цвета клавиш 6–9 (как в Lightroom); фиолетовый — только мышью.
const KEY_LABEL: Record<string, Label> = { '6': 'red', '7': 'yellow', '8': 'green', '9': 'blue' }

// Отметка клавишей: без события (меню) и с чужой клавишей не делает ничего. Тост без выделения — в ownMarks.setMarks.
function mark(ev: KeyboardEvent | undefined, patch: (key: string) => MarkPatch | null) {
  const p = ev && patch(ev.key.toLowerCase())
  if (p) void ownMarks.setMarks(store, p)
}

export const COMMANDS: Record<Command, Handler> = {
  // Файл
  new: () => newDoc.replace(Api.NewProject('album')),
  open: () => store.replaceState(Api.OpenProject()),
  openRecent: (_e, path) => path && openRecent(path),
  clearRecent: () => clearRecent(),
  save: () => afterEdit(() => Api.SaveProject()),
  saveAs: () => afterEdit(() => Api.SaveProjectAs()),
  backups: () => backupsUi.show(),
  addFolder: () => store.run(Api.ChooseFolder()),
  relink: () => relinkFolder(),
  export: () => edit.idle().then(() => store.exportAll()),
  showExport: () => showExportFolder(),
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
  // Окно «Настроить виньетку» для текущего листа; не виньетка или функции нет на уровне — молчим.
  vigSetup: () => {
    const s = store.spread
    if (isVignette(s) && prefs.can('vignette')) vigDialog.open(s.id)
  },
  // 041: выделенные в ленте развороты, иначе текущий; уровень — запомненный в окне автосборки.
  varyLayouts: () => {
    if (!prefs.can('variants')) return
    const ids = varyTargets(spreadsel.ordered(store.view.spreads.map((s) => s.id)), store.spread?.id)
    if (!anyVaries(store.view.spreads, ids)) return // как серый пункт меню: сетки, виньетки, пустые — молча
    return store.varyLayouts(ids, modes.varyLevel)
  },
  // 048: окно «Фоны»; нет листа или функции на уровне — молчим, как серый пункт меню
  bgGallery: () => {
    const s = store.spread
    if (s && prefs.can('decor')) decor.show('bg', s.id, defaultScope(s, store.cover))
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
  flipH: () => placesel.flip('h'),
  flipV: () => placesel.flip('v'),
  align: (_e, op) => isAlignOp(op) && align.run(op),
  alignTo: (_e, to) => align.setTo(normAlignTo(to)),
  turnCw: () => placesel.turn(1),
  turnCcw: () => placesel.turn(-1),
  straighten: () => straighten.startPhoto(),

  // Вид
  toggleBig: () => modes.toggleBig(),
  toggleDetails: () => modes.toggleDetails(),
  toggleSafe: () => modes.toggleSafe(),
  toggleRulers: () => modes.toggleRulers(),
  toggleSnap: () => placesel.toggleSnap(),
  levelSimple: () => prefs.set('simple'),
  levelStandard: () => prefs.set('standard'),
  levelFull: () => prefs.set('full'),
  levelCustom: () => prefs.set('custom'),
  // Окно уровней рисует LevelsDialog (читает prefs.compare).
  compareLevels: () => (prefs.compare = true),
  fitZoom: () => canvas.fit(),
  markRating: (e) => mark(e, (k) => (/^[0-5]$/.test(k) ? { rating: Number(k) } : null)),
  markStep: (e) => mark(e, (k) => ({ step: k === ']' || k === 'ъ' ? 1 : -1 })),
  markLabel: (e) => mark(e, (k) => (KEY_LABEL[k] ? { label: KEY_LABEL[k], toggle: true } : null)),
  markPick: (e) => mark(e, () => ({ flag: 1 })),
  markReject: (e) => mark(e, () => ({ flag: -1 })),
  markUnflag: (e) => mark(e, () => ({ flag: 0 })),
  zoomIn: () => zoomBy(ZOOM_STEP),
  zoomOut: () => zoomBy(1 / ZOOM_STEP),
  allSpreads: () => modes.openSection('grid'),
  help: () => (help.open = true),
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
  // Ctrl+R ниже «Полного»: окно не перезагружаем, больше ничего.
  reload: () => {},
}

export function run(cmd: Command, ev?: KeyboardEvent, arg?: string) {
  return COMMANDS[cmd](ev, arg)
}
