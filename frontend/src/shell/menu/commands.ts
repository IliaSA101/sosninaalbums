// Команды программы (спека 010, design-ui §2): клавиши (App), строка меню (MenuBar) и кнопки шапки
// зовут один `run`, поэтому «Отменить» из меню и Ctrl+Z делают одно и то же.
// Здесь только вызовы store/Api; решения «что показать» — в menu.ts и keys.ts.
import { Api, store } from '../../base/core/store.svelte'
import { jobs } from '../../base/core/jobs.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { canvas } from '../../editor/canvas/canvas.svelte'
import { row } from '../../editor/spread/spreadrow.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { nameSearch } from '../../panels/marks/namefind.svelte'
import { ownMarks, type MarkHost } from '../../panels/marks/ownmarks.svelte'
import { photoui } from '../../editor/photo/photoui.svelte'
import { compare } from '../../panels/compare/compare.svelte'
import { shownId } from '../../editor/photo/previewlist'
import { openNew, removeSelected, toggleSelected } from '../../panels/bins/binact'
import { binsel } from '../../panels/bins/binsel.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { keysUi } from '../hotkeys/keysui.svelte'
import { guide } from '../guide/guidestate.svelte'
import { backupsUi } from '../dialogs/backups.svelte'
import { packsUi } from '../packs/packs.svelte'
import { agents } from '../agents/agents.svelte'
import { share } from '../share/share.svelte'
import { shareShut } from '../share/share'
import { SHARE_EMPTY, SHARE_SHEET } from '../share/sharetext'
import { looks } from '../looks/looks.svelte'
import { saveLookUi } from '../looks/savelook.svelte'
import { tplio } from './tplio.svelte'
import { namesUi } from '../../panels/names/names.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { clip } from '../../editor/clip/clip.svelte'
import { straighten } from '../../editor/tilt/straighten.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { naming } from '../../panels/timeline/naming.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
import { clearVignette, makeVignette, sortVignette } from '../../panels/vignette/vigact'
import { glueAsk } from '../../panels/spreadmenu/menuact.svelte'
import { liveSpreadCmds } from './menuctx'
import { vigDialog } from '../../panels/vignette/vigsession.svelte'
import { decor } from '../../panels/decor/decor.svelte'
import { defaultScope } from '../../panels/decor/decor'
import { COVER_ID, isSheet } from '../../base/geom/sheet'
import { relinkFolder } from '../../panels/library/relink'
import { anyVaries, varyTargets } from '../../panels/gallery/variants'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { align } from '../../panels/layouts/align.svelte'
import { isAlignOp, normAlignTo } from '../../panels/layouts/align'
import { capFirst, errText } from '../../base/words/errtext'
import { effH, ZOOM_STEP } from '../../editor/zoom/zoom'
import { newDoc } from '../newdoc/newdoc.svelte'
import { aboutUi } from '../dialogs/about.svelte'
import { assocUi } from '../dialogs/assoc.svelte'
import { tour } from '../tour/tourstate.svelte'
import type { MarkPatch, RecentProject } from '../../base/core/types'

// ev — клавиатурное событие, если команду вызвала клавиша: сдвиг стрелкой и Пробел берут из него клавишу (служебные клавиши).
// arg — параметр пункта меню (путь альбома у openRecent, действие у align, опора у alignTo).
type Handler = (ev?: KeyboardEvent, arg?: string) => unknown

// Откат, сохранение и экспорт ждут, пока набранное в тексте запишется.
const afterEdit = (op: () => Promise<unknown>) => edit.idle().then(() => store.run(op()))

// Листание раскладок Пробелом: у сетки и в режиме фона раскладок нет. У виньетки (сетка по сути) листаются
// её шаблоны (решение 80). На обложке (127) Пробел листает лицевую (задняя — только ‹ ›); шаг, ▦ или тост
// решает gallery.coverStep — одно место с кнопкой ›.
function layoutStep(delta: number) {
  const s = store.spread
  if (!prefs.can('variants') || !s || bgmode.cur) return
  if (store.onCover) return void gallery.coverStep('right', delta)
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
// На стенде QA Go проводник не открывает (osx quiet) — говорим тостом, как ссылки «О программе».
async function showDataFolder() {
  if (store.stand) return store.notify('На стенде папки не открываются')
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
// Под окном крупного просмотра из ленты отметка идёт показанному фото, выделение ленты не меняется (0.26-128-5).
// Разворот { ...store } не годится: методы класса лежат в прототипе и при развороте теряются.
// Под сравнением (138) — активному кадру окна.
const markHost = (): MarkHost => {
  const id = compare.activeId ?? (photoui.preview?.from === 'library' ? photoui.previewId : null)
  if (!id) return store
  return { selectedIds: [id], run: (p) => store.run(p), notify: (t) => store.notify(t) }
}
const mark = (patch: MarkPatch) => () => void ownMarks.setMarks(markHost(), patch)
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
  // 131: макеты альбома и сохранение своего альбома как макета — окна и на «Простом» (без prefs.can)
  looks: () => void looks.show(),
  saveLook: () => void saveLookUi.show(),
  collect: () => edit.idle().then(() => jobs.collectShow()),
  addFolder: () => store.run(Api.ChooseFolder()),
  relink: () => relinkFolder(),
  export: () => edit.idle().then(() => jobs.exportAll()),
  // 146: окно «Отправить по ссылке»; без листов и у виньетки — тост, как у «Экспорта» (окно не открывается над стартовым)
  share: () =>
    edit.idle().then(() => {
      if (!shareShut(store.sheets.length, isSheet(store.view.settings), share.sharing)) return share.show()
      store.notify(store.sheets.length === 0 ? SHARE_EMPTY : SHARE_SHEET)
    }),
  showExport: () => showExportFolder(),
  showData: () => showDataFolder(),
  agents: () => agents.show(), // 143: окно доступа ИИ-агентов, на всех уровнях
  toggleAssoc: () => assocUi.toggle(), // 112: связь .salbum, пункт меню без клавиши
  tplExport: () => tplio.save(),
  tplImport: () => tplio.load(),
  settings: () => modes.openSection('settings'),

  // Правка
  undo: () => edit.idle().then(() => store.runHistory(Api.Undo())),
  redo: () => edit.idle().then(() => store.runHistory(Api.Redo())),
  // Поиск в ленте (0.26-124-10): раскрыть поле и поставить в него фокус; пункта меню нет. Пустая лента, «Во весь экран», галерея — молчим.
  findPhotos: () => nameSearch.focus(),
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
  // 099: буфер Go; куда встанет вставка и что выделено — решает clip. Меню и клавиши зовут одно.
  cutItems: () => clip.cut(),
  copyItems: () => clip.copy(),
  pasteItems: () => clip.paste(),
  duplicateItems: () => clip.duplicate(),
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
  // 141: пять команд разворота из правого щелчка. Условие проверяется заново: клавиша могла прийти, пока меню не строилось;
  // серое состояние молчит, как пункт. Цель — текущий лист (0.30-141-5).
  insertSpread: () => {
    const at = liveSpreadCmds().insertAt
    if (at !== null) return store.addEmptySpread(at)
  },
  splitSpread: () => {
    const c = liveSpreadCmds().cut
    if (!c.ok || !prefs.can('timeline')) return
    return c.ask ? glueAsk.cut(c.id, c.at) : store.split(c.id, c.at)
  },
  joinSpread: () => {
    const c = liveSpreadCmds().join
    if (!c.ok || !prefs.can('timeline')) return
    return c.ask ? glueAsk.join(c.id) : store.merge(c.id)
  },
  sortVignette: () => {
    const s = store.spread
    if (isVignette(s) && prefs.can('vignette')) return sortVignette(s.id)
  },
  clearVignette: () => {
    const s = store.spread
    if (isVignette(s) && prefs.can('vignette')) return clearVignette(s.id)
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
  toggleFaceGuide: () => modes.toggleFaceGuide(),
  toggleSnap: () => placesel.toggleSnap(),
  levelSimple: () => prefs.set('simple'),
  levelStandard: () => prefs.set('standard'),
  levelFull: () => prefs.set('full'),
  levelCustom: () => prefs.set('custom'),
  // Окно уровней рисует LevelsDialog (читает prefs.compare).
  compareLevels: () => (prefs.compare = true),
  fitZoom: () => canvas.fit(),
  // Крупный просмотр фото (128): лента — выделенное, холст — одно выделенное фото разворота; нечего открыть — тост.
  photoPreview: () => photoui.openByKey(),
  // Сравнение 2–4 кадров (138): C — по выделению, Shift+C — похожие к этому; из окна 128 — с его списком.
  photoCompare: () => {
    const p = photoui.preview?.from === 'library' ? photoui.preview : null
    if (p) compare.openFrom(p.ids, shownId(p), () => photoui.handOff())
    else compare.openByKey()
  },
  photoSimilar: () => {
    const p = photoui.preview?.from === 'library' ? photoui.preview : null
    if (p) void compare.similar(shownId(p), p.ids, () => photoui.handOff())
    else void compare.similarByKey()
  },
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
  // Первые шаги (113): приветствие со списком; видео открывает браузер (адрес зашит в Go).
  tour: () => tour.start('menu'),
  videos: () => void aboutUi.link('videos'),
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
  about: () => aboutUi.show(),

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
