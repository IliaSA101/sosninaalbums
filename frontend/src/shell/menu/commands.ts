// Команды программы (спека 010, design-ui §2): клавиши (App), строка меню (MenuBar) и кнопки шапки
// зовут один `run`, поэтому «Отменить» из меню и Ctrl+Z делают одно и то же.
// Здесь только вызовы store/Api; решения «что показать» — в menu.ts и keys.ts.
import { Api, store } from '../../base/core/store.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { row } from '../../editor/spread/spreadrow.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { gallery } from '../../panels/layouts/gallery.svelte'
import { help } from './help.svelte'
import { backupsUi } from '../dialogs/backups.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { relinkFolder } from '../../panels/library/relink'
import { align } from '../../panels/layouts/align.svelte'
import { isAlignOp, normAlignTo } from '../../panels/layouts/align'
import { errText } from '../../base/core/errtext'
import { ZOOM_STEP } from '../../editor/zoom/zoom'
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
  | 'flipH'
  | 'flipV'
  | 'align'
  | 'alignTo'
  | 'turnCw'
  | 'turnCcw'
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
export type Command = KeyAction | MenuCommand

// ev — клавиатурное событие, если команду вызвала клавиша: сдвиг стрелкой и Пробел берут из него клавишу.
// arg — параметр пункта меню (путь альбома у openRecent, действие у align, опора у alignTo).
type Handler = (ev?: KeyboardEvent, arg?: string) => unknown

// Откат, сохранение и экспорт ждут, пока набранное в тексте запишется.
const afterEdit = (op: () => Promise<unknown>) => edit.idle().then(() => store.run(op()))

// Листание раскладок Пробелом: у сетки и в режиме фона раскладок нет.
function layoutStep(delta: number) {
  if (prefs.can('variants') && store.spread && !store.spread.grid && !bgmode.cur) gallery.step('spread', delta)
}

// Зум клавишами — к центру окна холста; посреди жеста масштаб не меняется (как у колеса).
function zoomBy(factor: number) {
  if (!store.spread || canvas.busy || canvas.pressed || row.atNew) return
  row.finish() // зум не начинается посреди движения ряда
  canvas.zoomAt(factor, canvas.vp.w / 2, canvas.vp.h / 2)
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
  new: () => store.replaceState(Api.NewProject('album')),
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
  undo: () => afterEdit(() => Api.Undo()),
  redo: () => afterEdit(() => Api.Redo()),
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
  flipH: () => placesel.flip('h'),
  flipV: () => placesel.flip('v'),
  align: (_e, op) => isAlignOp(op) && align.run(op),
  alignTo: (_e, to) => align.setTo(normAlignTo(to)),
  turnCw: () => placesel.turn(1),
  turnCcw: () => placesel.turn(-1),

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
  startWindow: () => (prefs.startOpen = true),
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
    // Иначе кнопка в фокусе (например, ⊞) нажмётся пробелом ещё раз.
    ;(e.target as HTMLElement | null)?.closest<HTMLElement>('button')?.blur()
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
