import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { catalogView, dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { keyMarks } from '../../base/keys/keytext'
import { ACTS, type ActId } from '../../editor/photomenu/photoacts'
import { ACT_GROUPS, inventory } from './inventory'
import { buildMenu, type Menu, type MenuCtx, type MenuItem } from '../menu/menu'
import { ARTICLES, placesOf, type Article } from '../guidetext/articles'
import { search } from './search'

// Стражи полноты справочника F1 (спека 136, design §4): каждая функция из каталога-источника описана статьёй
// или записана в исключения с причиной. Новая функция без статьи роняет тест.
// Источники: S1 меню всех уровней, S2 команды с клавишей, S3 жесты MOUSE_HELP, S4 действия ACTS, S5 окна с <Modal>.
// S3 берёт жесты из общей описи (inventory.ts, спека 140). S2 проверяет весь каталог клавиш, S4 — все ACTS кроме
// групп-подменю: у статьи эти id кормят поиск, поэтому страж шире описи, которая считает только число функций.
const sources = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

// Исключения: причина обязательна. Запись про описанное или несуществующее — провал.
type Exceptions = Record<string, string>

// Очередь недописанного (спека 136): запись «Т5: нет статьи → id» в списках PENDING_* — функция есть, статьи ещё нет.
// Статья появилась — запись становится «про описанное» и роняет тест, пока её не уберут. После Т5 списки пусты;
// механизм остаётся, чтобы новую функцию можно было записать в очередь на один шаг, а не в постоянные исключения.
export const T5 = 'Т5:'
const t5 = (to: string): string => `${T5} нет статьи → ${to}`

// Меню, которое видит страж S1: всё доступно, «Открывать .salbum…» и «Последние» с записью.
const MENU_CTX: MenuCtx = {
  level: 'full',
  spreads: 1,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 1,
  anySelection: true,
  deletable: true,
  flippable: true,
  varyable: true,
  layerable: true,
  missing: 1,
  big: false,
  details: false,
  safe: false,
  rulers: false,
  snap: false,
  alignCount: 2,
  assocCan: true,
  recent: [{ path: '/a/b.salbum', name: 'b', exists: true, modified: 1 }],
}

// ── Общая проверка «каталог ↔ покрытие ↔ исключения» ──────────────────────────────────────────

// all — элементы каталога, covered — какие из них описаны статьёй. Возвращает список нарушений (пустой — зелёное).
function audit(all: readonly string[], covered: (x: string) => boolean, perm: Exceptions, pend: Exceptions): string[] {
  const out: string[] = []
  const have = new Set(all)
  for (const x of all) if (!covered(x) && !(x in perm) && !(x in pend)) out.push(`нет статьи и нет исключения: ${x}`)
  for (const [x, why] of Object.entries(perm)) {
    if (why.trim().length === 0) out.push(`исключение без причины: ${x}`)
    if (why.startsWith(T5)) out.push(`«${T5}» — только в списке Т5: ${x}`)
    if (!have.has(x)) out.push(`исключение про несуществующее: ${x}`)
    else if (covered(x)) out.push(`исключение про уже описанное: ${x}`)
  }
  for (const [x, why] of Object.entries(pend)) {
    if (!why.startsWith(T5)) out.push(`запись Т5 без «${T5}»: ${x}`)
    if (x in perm) out.push(`запись и в постоянных исключениях, и в Т5: ${x}`)
    if (!have.has(x)) out.push(`запись Т5 про несуществующее: ${x}`)
    else if (covered(x)) out.push(`запись Т5 про уже описанное (статья появилась — уберите): ${x}`)
  }
  return out
}

// ── S1 Меню ────────────────────────────────────────────────────────────────────────────────

interface MenuRow {
  id: string
  path: string // «Файл ▸ Последние ▸ Очистить список»
  label: string
  top: string // id верхнего пункта
  nested: boolean
}

// Все пункты с командой или подменю на всех уровнях; разделители и пункты без cmd/sub не в счёте.
function menuRows(tree: Menu[]): MenuRow[] {
  const out: MenuRow[] = []
  const walk = (list: MenuItem[], head: string, top: string | null): void => {
    for (const it of list) {
      const t = top ?? it.id
      if (it.cmd || it.sub) out.push({ id: it.id, path: `${head} ▸ ${it.label}`, label: it.label, top: t, nested: top !== null })
      if (it.sub) walk(it.sub, `${head} ▸ ${it.label}`, t)
    }
  }
  for (const m of tree) walk(m.items, m.label, null)
  return out
}

// Id верхних пунктов, которые называет статья: a.menu и места menu:*.
const menuIds = (a: Article): string[] => [
  ...(a.menu ?? []),
  ...placesOf(a).flatMap((p) => (p.spot.startsWith('menu:') ? [p.spot.slice(5)] : [])),
]

// Пункт покрыт, если его id назван статьёй; вложенный — если назван его верхний пункт и поиск по подписи
// пункта ставит эту статью в первую тройку (подменю в индексе поиска, design §3). Записи «Последних» —
// recent-N с именем файла — идут через пункт «Последние», поиск по имени файла им не нужен.
function s1(tree: Menu[], arts: readonly Article[], perm: Exceptions, pend: Exceptions): string[] {
  const rows = menuRows(tree)
  const byId = new Map(rows.map((r) => [r.id, r]))
  const direct = new Set(arts.flatMap(menuIds))
  const covered = (id: string): boolean => {
    if (direct.has(id)) return true
    const r = byId.get(id)
    if (!r?.nested || !direct.has(r.top)) return false
    if (/^recent-\d+$/.test(id)) return true
    const owners = new Set(arts.filter((a) => menuIds(a).includes(r.top)).map((a) => a.id))
    return search(r.label)
      .hits.slice(0, 3)
      .some((h) => owners.has(h.a.id))
  }
  const ids = rows.map((r) => r.id)
  const bad = audit(ids, covered, perm, pend)
  const path = (id: string): string => byId.get(id)?.path ?? id
  return bad.map((s) => s.replace(/: (.+)$/, (_m, id: string) => `: ${path(id)} [${id}]`))
}

// Пункты меню без статьи, и это правильно.
const NO_MENU: Exceptions = {
  whatsNew: 'окно «Что нового» — список изменений версии, а не функция редактора',
  about: 'окно «О программе»: версия, автор, лицензия и ссылки, а не функция редактора',
}
// Списки Т5 (запись «Т5: …» — статья ещё не написана) после Т5 пусты; механизм остаётся для самопроверок.
const PENDING_MENU: Exceptions = {}
// ── S2 Клавиши ─────────────────────────────────────────────────────────────────────────────

interface Cmd {
  id: string
  keys: string[]
}

// Команда названа статьёй: в keys или меткой {key:id} в тексте.
function s2(cmds: readonly Cmd[], arts: readonly Article[], perm: Exceptions, pend: Exceptions): string[] {
  const named = new Set<string>()
  for (const a of arts) {
    for (const k of a.keys ?? []) named.add(k)
    for (const id of keyMarks(`${a.what} ${a.where ?? ''} ${a.how ?? ''}`)) named.add(id)
  }
  return audit(
    cmds.filter((c) => c.keys.length > 0).map((c) => c.id),
    (id) => named.has(id),
    perm,
    pend,
  )
}

const NO_KEY: Exceptions = {}
const PENDING_KEY: Exceptions = {}

// ── S3 Жесты ───────────────────────────────────────────────────────────────────────────────

function s3(rows: readonly { show: string }[], arts: readonly Article[], perm: Exceptions, pend: Exceptions): string[] {
  const named = new Set(arts.flatMap((a) => a.keys ?? []))
  return audit(
    rows.map((r) => r.show),
    (s) => named.has(s),
    perm,
    pend,
  )
}

const NO_MOUSE: Exceptions = {}
const PENDING_MOUSE: Exceptions = {}

// ── S4 Действия фото ───────────────────────────────────────────────────────────────────────

function s4(acts: readonly string[], arts: readonly Article[], perm: Exceptions, pend: Exceptions): string[] {
  const named = new Set<string>(arts.flatMap((a) => a.acts ?? []))
  return audit(acts, (id) => named.has(id), perm, pend)
}

// «Ещё», «Слой», «Целиком / с обрезкой» — подменю (ACT_GROUPS), их статьи не называют.
const NO_ACT: Exceptions = {}
// Все действия меню фото, в том числе с командой (они в описи как команды), без групп-подменю.
const allActs = (): string[] => Object.keys(ACTS).filter((id) => !ACT_GROUPS.includes(id as ActId))
const PENDING_ACT: Exceptions = {}

// ── S5 Окна ────────────────────────────────────────────────────────────────────────────────

// Файлы с <Modal: проверяется разметка, не имена.
const modalFiles = (src: Record<string, string>): string[] =>
  Object.entries(src)
    .filter(([, text]) => text.includes('<Modal'))
    .map(([p]) => p)
    .sort()

// Реестр: путь .svelte → id статьи или «нет: причина» (вопрос подтверждения, ход загрузки, подокно функции X).
const WINDOWS: Record<string, string> = {
  '/src/editor/text/FontPicker.svelte':
    'нет: вопрос «Убрать из библиотеки?» подтверждает удаление шрифта; сам выбор шрифта описан в text-style и own-fonts',
  '/src/panels/albumbg/AlbumOwnAsk.svelte': 'нет: вопрос — переписать ли свои цвета страниц при смене фона',
  '/src/panels/decor/DecorDialog.svelte': 'decor',
  '/src/panels/decor/QrDialog.svelte': 'decor',
  '/src/panels/decorbrowse/GetAsk.svelte': 'нет: вопрос «Скачать набор?» перед скачиванием образца — общий для галерей и «Макетов альбома»',
  '/src/panels/decor/SpreadSave.svelte': 'spread-tpl',
  '/src/panels/folders/RemoveAsk.svelte': 'нет: вопрос подтверждения при удалении папки из библиотеки',
  '/src/panels/gallery/GridTplModals.svelte': 'templates',
  '/src/panels/layouts/CopyGridDialog.svelte': 'copy-grid',
  '/src/panels/names/NamesDialog.svelte': 'names',
  '/src/panels/vignette/VigDialog.svelte': 'vig-settings',
  '/src/panels/vignette/VigTplSave.svelte': 'vig-own',
  '/src/shell/ask/AutoAskDialog.svelte': 'нет: вопрос подтверждения перед автораскладкой',
  '/src/shell/ask/CopyAskDialog.svelte': 'нет: вопрос подтверждения при копировании раскладки',
  '/src/shell/ask/DropAskDialog.svelte': 'нет: вопрос подтверждения при падении файлов в окно',
  '/src/shell/ask/NewerAskDialog.svelte': 'нет: вопрос подтверждения — файл новее программы',
  '/src/shell/autoalbum/RebuildAskDialog.svelte': 'нет: вопрос подтверждения пересборки альбома',
  '/src/shell/autobuild/AutoBuildDialog.svelte': 'autobuild',
  '/src/shell/collect/CollectDialog.svelte': 'collect',
  '/src/shell/dialogs/AboutDialog.svelte': 'нет: окно «О программе» — версия, автор, лицензия, а не функция редактора',
  '/src/shell/dialogs/BackupsDialog.svelte': 'backups',
  '/src/shell/dialogs/LevelsDialog.svelte': 'levels',
  '/src/shell/dialogs/OpenProgress.svelte': 'нет: ход загрузки альбома, без действий',
  '/src/shell/dialogs/StartDialog.svelte': 'start-window',
  '/src/shell/export/ExportDialog.svelte': 'export',
  '/src/shell/export/PrintDone.svelte': 'нет: итог выгрузки для типографии — следствие экспорта, описан в fsfera',
  '/src/shell/hotkeys/KeyAsk.svelte': 'нет: вопрос подтверждения при занятой клавише, подокно окна «Горячие клавиши»',
  '/src/shell/hotkeys/KeysDialog.svelte': 'hotkeys',
  '/src/shell/guide/Guide.svelte': 'guide',
  '/src/shell/guide/VideoWindow.svelte': 'нет: окно ролика статьи F1 — подокно справочника, описано в статье guide',
  '/src/shell/agents/AgentsDialog.svelte': 'agents',
  '/src/shell/agents/AgentsPerms.svelte': 'нет: блок «Разрешения» окна «ИИ-агенты» и его вопрос-согласие — часть статьи agents',
  '/src/shell/agents/AgentAskDialog.svelte':
    'нет: вопрос подтверждения — агент просит создать альбом или добавить папку, описан в статье agents-edit',
  '/src/shell/share/ShareDialog.svelte': 'share-link',
  '/src/shell/looks/LooksDialog.svelte': 'looks',
  '/src/shell/looks/SaveLookDialog.svelte': 'looks-save',
  '/src/shell/packs/NetConsent.svelte': 'нет: вопрос подтверждения «Обратиться к GitHub?» — общий для «Наборов» и роликов F1',
  '/src/shell/packs/PacksDialog.svelte': 'library-packs',
  '/src/shell/printprof/PrintProfileDialog.svelte': 'print-profile',
}
const PENDING_WINDOWS: Exceptions = {}

const NO = 'нет: '

function s5(src: Record<string, string>, reg: Record<string, string>, arts: readonly Article[], pend: Exceptions): string[] {
  const ids = new Set(arts.map((a) => a.id))
  const files = modalFiles(src)
  const perm: Exceptions = {}
  const named = new Set<string>()
  const out: string[] = []
  for (const [p, v] of Object.entries(reg)) {
    if (v.startsWith(NO)) perm[p] = v.slice(NO.length)
    else if (ids.has(v)) named.add(p)
    else out.push(`окно ${p}: «${v}» — не id статьи и не «нет: причина»`)
  }
  out.push(...audit(files, (p) => named.has(p), perm, pend))
  // Целевая статья записи Т5 не должна уже существовать: иначе запись надо перенести в реестр.
  for (const [p, why] of Object.entries(pend)) {
    const to = /→ (\S+)$/.exec(why)?.[1]
    if (to && ids.has(to)) out.push(`запись Т5 ${p}: статья ${to} уже есть — перенесите в WINDOWS`)
  }
  return out
}

// ── Тесты ──────────────────────────────────────────────────────────────────────────────────

// Опись, как её видит страж: то же дерево меню, что у S1, и каталог клавиш.
const inv = (): ReturnType<typeof inventory> => inventory(buildMenu(MENU_CTX), catalogView.commands)

describe('стражи полноты справочника: каталоги покрыты статьями', () => {
  it('S1: каждый пункт меню на всех уровнях описан', () => {
    expect(s1(buildMenu(MENU_CTX), ARTICLES, NO_MENU, PENDING_MENU)).toEqual([])
  })
  it('S1: меню содержит «Открывать .salbum…» и «Последние»', () => {
    const ids = menuRows(buildMenu(MENU_CTX)).map((r) => r.id)
    expect(ids).toContain('assoc')
    expect(ids).toContain('recent')
    expect(ids).toContain('recent-0')
  })
  it('S2: каждая команда с клавишей названа', () => {
    expect(s2(catalogView.commands, ARTICLES, NO_KEY, PENDING_KEY)).toEqual([])
  })
  it('S3: каждый жест MOUSE_HELP назван', () => {
    expect(
      s3(
        inv().gestures.map((show) => ({ show })),
        ARTICLES,
        NO_MOUSE,
        PENDING_MOUSE,
      ),
    ).toEqual([])
  })
  it('S4: каждое действие ACTS названо в acts', () => {
    expect(s4(allActs(), ARTICLES, NO_ACT, PENDING_ACT)).toEqual([])
  })
  it('S4: acts статей — существующие действия', () => {
    const bad = ARTICLES.flatMap((a) => (a.acts ?? []).filter((id) => !(id in ACTS)).map((id) => `${a.id}: ${id}`))
    expect(bad).toEqual([])
  })
  it('опись: группы-подменю не в счёте, а каждая команда действия есть в описи', () => {
    const i = inv()
    for (const g of ACT_GROUPS) expect(i.acts.has(g)).toBe(false)
    const lost = Object.entries(ACTS).flatMap(([id, a]) => (a.cmd && !i.commands.has(a.cmd) ? [`${id} → ${a.cmd}`] : []))
    expect(lost).toEqual([])
  })
  it('S5: каждое окно с <Modal учтено в реестре', () => {
    expect(s5(sources, WINDOWS, ARTICLES, PENDING_WINDOWS)).toEqual([])
  })
})

describe('стражи полноты: самопроверки (подменённый каталог роняет страж)', () => {
  const empty: Exceptions = {}
  const extraArticle: Article = { id: 'zz', title: 'Тест', group: 'start', feature: 'base', what: 'x', how: 'x' }

  it('S1: пункт без статьи — провал; вложенный без находимости — провал', () => {
    const base = buildMenu(MENU_CTX)
    const plus: Menu[] = [
      { ...base[0], items: [...base[0].items, { id: 'zzNew', label: 'Совсем новая функция', cmd: 'new' }] },
      ...base.slice(1),
    ]
    expect(s1(plus, ARTICLES, NO_MENU, PENDING_MENU).join()).toContain('zzNew')
    // Верхний пункт назван, но подпись вложенного поиск к статье не приводит.
    const art: Article = { ...extraArticle, id: 'zz-save', menu: ['save'] }
    const nested: Menu[] = [
      {
        ...base[0],
        items: base[0].items.map((i) =>
          i.id === 'save' ? { ...i, sub: [{ id: 'zzSub', label: 'Нечто совершенно постороннее', cmd: 'save' as const }] } : i,
        ),
      },
    ]
    expect(s1(nested, [art], empty, empty).join()).toContain('zzSub')
  })
  it('S2: команда с клавишей без статьи — провал; без клавиши — нет', () => {
    const cmds = [
      { id: 'zzCmd', keys: ['Ctrl+9'] },
      { id: 'zzFree', keys: [] },
    ]
    const bad = s2(cmds, ARTICLES, empty, empty).join()
    expect(bad).toContain('zzCmd')
    expect(bad).not.toContain('zzFree')
    expect(s2(cmds, [{ ...extraArticle, keys: ['zzCmd'] }], empty, empty)).toEqual([])
    expect(s2(cmds, [{ ...extraArticle, what: 'Нажмите{key:zzCmd| %}' }], empty, empty)).toEqual([])
  })
  it('S3: новый жест без статьи — провал', () => {
    expect(
      s3(
        [...inv().gestures, 'Тройной щелчок'].map((show) => ({ show })),
        ARTICLES,
        NO_MOUSE,
        PENDING_MOUSE,
      ).join(),
    ).toContain('Тройной щелчок')
  })
  it('S4: новое действие без статьи — провал', () => {
    expect(s4([...allActs(), 'zzAct'], ARTICLES, NO_ACT, PENDING_ACT).join()).toContain('zzAct')
    expect(s4(['zzAct'], [{ ...extraArticle, acts: ['zzAct' as ActId] }], empty, empty)).toEqual([])
  })
  it('S4: действие с командой (relink) без acts в статьях — провал', () => {
    const cut = ARTICLES.map((a) => ({ ...a, acts: a.acts?.filter((id) => id !== 'relink') }))
    expect(s4(allActs(), cut, NO_ACT, PENDING_ACT).join()).toContain('relink')
  })
  it('S5: новое окно с <Modal без записи — провал; без <Modal запись — мусор', () => {
    const src = { ...sources, '/src/shell/zz/ZzDialog.svelte': '<Modal title="Новое"></Modal>' }
    expect(s5(src, WINDOWS, ARTICLES, PENDING_WINDOWS).join()).toContain('ZzDialog')
    const noModal = { ...WINDOWS, '/src/shell/zz/NoModal.svelte': 'нет: причина' }
    expect(s5(sources, noModal, ARTICLES, PENDING_WINDOWS).join()).toContain('NoModal')
    const badId = { ...WINDOWS, '/src/shell/guide/Guide.svelte': 'no-such-article' }
    expect(s5(sources, badId, ARTICLES, {}).join()).toContain('no-such-article')
  })
  it('исключение без причины, про описанное или несуществующее — провал', () => {
    const all = ['a', 'b']
    const covered = (x: string): boolean => x === 'a'
    expect(audit(all, covered, { b: '  ' }, empty).join()).toContain('без причины')
    expect(audit(all, covered, { a: 'зачем' }, empty).join()).toContain('уже описанное')
    expect(audit(all, covered, { b: 'ок', c: 'нет такого' }, empty).join()).toContain('несуществующее')
    expect(audit(all, covered, empty, { b: 'просто так' }).join()).toContain('без «Т5')
    expect(audit(all, covered, empty, { a: t5('x') }).join()).toContain('уже описанное')
    expect(audit(all, covered, { b: t5('x') }, empty).join()).toContain('только в списке Т5')
    expect(audit(all, covered, { b: 'ок' }, { b: t5('x') }).join()).toContain('и в постоянных')
    expect(audit(all, covered, empty, empty).join()).toContain('нет исключения: b')
    expect(audit(all, covered, { b: 'ок' }, empty)).toEqual([])
  })
  it('S5: запись Т5 про статью, которая уже есть, — провал', () => {
    const pend = { '/src/shell/guide/Guide.svelte': t5('undo') }
    expect(s5(sources, { ...WINDOWS }, ARTICLES, pend).join()).toContain('уже есть')
  })
})
