import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { dropCatalog, useCatalog } from '../../base/keys/keymap.fixture'
import { keymap } from '../../base/keys/keymap.svelte'
import { can, type Custom, type FeatureId, type Level } from '../../base/prefs/levels'
import { features } from '../../base/prefs/levelcustom'
import { buildMenu, enabledCmds, keyId, keysOf, type Menu, type MenuCtx, type MenuItem } from './menu'

// Клавиши меню — из реестра (074): без него подписи пусты.
beforeEach(() => useCatalog())
afterEach(() => dropCatalog())

const base: MenuCtx = {
  spreads: 3,
  hasSpread: true,
  canUndo: true,
  canRedo: true,
  zone: 'canvas',
  libraryPhotos: 10,
  anySelection: true,
  deletable: true,
  flippable: true,
  varyable: true,
  layerable: true,
  missing: 2,
  big: false,
  details: false,
  safe: false,
  rulers: true,
  snap: true,
}
const ctx = (o: Partial<MenuCtx> = {}): MenuCtx => ({ ...base, ...o })

const flat = (items: MenuItem[]): MenuItem[] => items.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const find = (menus: Menu[], id: string): MenuItem | undefined => menus.flatMap((m) => flat(m.items)).find((i) => i.id === id)
// Меню нет — ошибка: иначе проверка «пункта нет в меню» прошла бы и для чужого, и для несуществующего (0.26-124-20).
const ids = (menus: Menu[], menu: string) => {
  const m = menus.find((x) => x.id === menu)
  if (!m) throw new Error(`меню «${menu}» нет в строке`)
  return flat(m.items).map((i) => i.id)
}

describe('buildMenu', () => {
  it('пять меню по порядку', () => {
    expect(buildMenu(ctx()).map((m) => m.label)).toEqual(['Файл', 'Правка', 'Разворот', 'Вид', 'Справка'])
  })

  it('состав «Файл» и клавиши берутся из реестра', () => {
    const m = buildMenu(ctx())
    expect(ids(m, 'file')).toEqual([
      'new',
      'newSheet',
      'open',
      'recent',
      'save',
      'saveAs',
      'collect',
      'backups',
      'packs',
      'looks',
      'saveLook',
      'tpl',
      'tplExport',
      'tplImport',
      'showData',
      'addFolder',
      'relink',
      'export',
      'showExport',
      'settings',
    ])
    expect(find(m, 'new')?.keys).toBe('Ctrl+N')
    expect(find(m, 'saveAs')?.keys).toBe('Ctrl+Shift+S')
    expect(find(m, 'export')?.keys).toBe('Ctrl+E')
    expect(find(m, 'redo')?.keys).toBe('Ctrl+Y')
  })

  it('клавиша каждого пункта — первая клавиша его команды в реестре; без команды в реестре пункт без клавиши', () => {
    for (const it of buildMenu(ctx()).flatMap((m) => flat(m.items))) {
      const id = it.cmd && keyId(it.cmd, it.arg)
      expect(it.keys ?? '', it.id).toBe((id && keymap.hint(id)) || '')
    }
  })

  it('своя клавиша видна в меню сразу, команда без клавиши — без подписи', () => {
    useCatalog({ toggleBig: ['KeyB'], save: [], alignLeft: ['Mod+Shift+KeyL'] })
    const m = buildMenu(ctx({ alignCount: 3, level: 'full', can: (f) => can(f, 'full') }))
    expect(find(m, 'big')?.keys).toBe('B')
    expect(find(m, 'save')?.keys).toBeUndefined()
    expect(find(m, 'align-left')?.keys).toBe('Ctrl+Shift+L')
    expect(find(m, 'align-right')?.keys).toBeUndefined()
  })

  it('«Последние»: пункты по списку, пропавший серый, путь — в arg; пусто — пункт серый', () => {
    const recent = [
      { path: 'D:\\a\\Один.salbum', name: 'Один', exists: true, modified: 1 },
      { path: 'D:\\a\\Два.salbum', name: 'Два', exists: false, modified: 0 },
    ]
    const m = buildMenu(ctx({ recent }))
    const top = find(m, 'recent')!
    expect(top.disabled).toBeFalsy()
    expect(top.sub?.map((i) => i.id)).toEqual(['recent-0', 'recent-1', 'clearRecent'])
    expect(find(m, 'recent-0')).toMatchObject({ label: 'Один', cmd: 'openRecent', arg: 'D:\\a\\Один.salbum' })
    expect(find(m, 'recent-0')?.disabled).toBeFalsy()
    expect(find(m, 'recent-1')?.disabled).toBe(true)
    expect(find(m, 'clearRecent')).toMatchObject({ cmd: 'clearRecent', sep: true })
    const empty = find(buildMenu(ctx()), 'recent')!
    expect(empty.disabled).toBe(true)
    expect(empty.sub).toEqual([])
  })

  it('«Справка ▸ Стартовое окно» есть на любом уровне', () => {
    expect(find(buildMenu(ctx({ can: () => false })), 'startWindow')?.cmd).toBe('startWindow')
  })

  it('«Найти пропавшие» серый без пропавших', () => {
    expect(find(buildMenu(ctx({ missing: 0 })), 'relink')?.disabled).toBe(true)
    expect(find(buildMenu(ctx()), 'relink')?.disabled).toBeFalsy()
  })

  it('экспорт серый при пустом альбоме; пока идёт выгрузка пункт доступен — открывает окно хода (073)', () => {
    expect(find(buildMenu(ctx({ spreads: 0 })), 'export')?.disabled).toBe(true)
    expect(find(buildMenu(ctx()), 'export')?.disabled).toBeFalsy()
    expect(find(buildMenu(ctx({ spreads: 0, exporting: true })), 'export')?.disabled).toBeFalsy()
    expect(find(buildMenu(ctx({ spreads: 0, exporting: false })), 'export')?.disabled).toBe(true)
  })

  it('«Собрать проект в папку…» — после «Сохранить как…»; серое только пока сбор идёт (в пустом альбоме активно)', () => {
    const file = ids(buildMenu(ctx()), 'file')
    expect(file.indexOf('collect')).toBe(file.indexOf('saveAs') + 1)
    expect(find(buildMenu(ctx()), 'collect')?.cmd).toBe('collect')
    expect(find(buildMenu(ctx()), 'collect')?.disabled).toBeFalsy()
    expect(find(buildMenu(ctx({ spreads: 0 })), 'collect')?.disabled).toBeFalsy()
    expect(find(buildMenu(ctx({ collecting: true })), 'collect')?.disabled).toBe(true)
  })

  it('«Отменить / Повторить» следуют за историей', () => {
    const m = buildMenu(ctx({ canUndo: false, canRedo: false }))
    expect(find(m, 'undo')?.disabled).toBe(true)
    expect(find(m, 'redo')?.disabled).toBe(true)
  })

  it('«Выделить всё» по зоне: холст — разворот, библиотека — лента', () => {
    expect(find(buildMenu(ctx({ zone: 'canvas' })), 'selectAll')?.cmd).toBe('selectSpread')
    const lib = buildMenu(ctx({ zone: 'library', libraryPhotos: 0 }))
    expect(find(lib, 'selectAll')?.cmd).toBe('selectAll')
    expect(find(lib, 'selectAll')?.disabled).toBe(true)
  })

  it('выделение и удаление серые, когда нечего', () => {
    const m = buildMenu(ctx({ anySelection: false, deletable: false, flippable: false }))
    expect(find(m, 'clearSelection')?.disabled).toBe(true)
    expect(find(m, 'removeCell')?.disabled).toBe(true)
    expect(find(m, 'flip')?.disabled).toBe(true)
  })

  it('«Зеркало» — подменю из двух команд', () => {
    const flip = find(buildMenu(ctx()), 'flip')
    expect(flip?.cmd).toBeUndefined()
    expect(flip?.sub?.map((i) => i.cmd)).toEqual(['flipH', 'flipV'])
  })

  it('режим F в «Виде» называется «Во весь экран», не «Крупный просмотр» (128-11, Т7)', () => {
    const m = buildMenu(ctx())
    expect(find(m, 'big')?.label).toBe('Во весь экран')
    expect(find(m, 'big')?.cmd).toBe('toggleBig')
    expect(JSON.stringify(m)).not.toContain('Крупный просмотр')
  })

  it('переключатели вида показывают состояние', () => {
    const m = buildMenu(ctx({ big: true, safe: true, details: false, snap: false }))
    expect(find(m, 'big')?.checked).toBe(true)
    expect(find(m, 'safe')?.checked).toBe(true)
    expect(find(m, 'details')?.checked).toBe(false)
    expect(find(m, 'snap')?.checked).toBe(false)
    expect(find(m, 'fit')?.checked).toBeUndefined()
  })

  it('«Вид ▸ Силуэт под лицо» (130): после «Привязки», галочка по состоянию, без клавиши', () => {
    const on = buildMenu(ctx({ faceGuide: true }))
    expect(find(on, 'faceGuide')).toMatchObject({ label: 'Силуэт под лицо', cmd: 'toggleFaceGuide', checked: true })
    expect(find(on, 'faceGuide')?.keys).toBeUndefined()
    expect(find(buildMenu(ctx({ faceGuide: false })), 'faceGuide')?.checked).toBe(false)
    const view = ids(on, 'view')
    expect(view.indexOf('faceGuide')).toBe(view.indexOf('snap') + 1)
  })

  it('«Вид ▸ Разметка типографии» (066): сразу после «Безопасной зоны», галочка по состоянию, без клавиши', () => {
    const on = buildMenu(ctx({ markup: true }))
    expect(find(on, 'markup')).toMatchObject({ label: 'Разметка типографии', cmd: 'toggleMarkup', checked: true })
    expect(find(on, 'markup')?.keys).toBeUndefined()
    expect(find(buildMenu(ctx({ markup: false })), 'markup')?.checked).toBe(false)
    expect(find(buildMenu(ctx()), 'markup')?.checked).toBe(false) // поле не передали — выключено
    const view = ids(on, 'view')
    expect(view.indexOf('markup')).toBe(view.indexOf('safe') + 1)
  })

  it('без разворота масштаб серый', () => {
    const m = buildMenu(ctx({ hasSpread: false }))
    for (const id of ['fit', 'zoomIn', 'zoomOut', 'addText']) expect(find(m, id)?.disabled, id).toBe(true)
  })

  it('«Имена из списка…» (030): в «Правке», серое без разворота', () => {
    expect(find(buildMenu(ctx()), 'pasteNames')).toMatchObject({ label: 'Имена из списка…', cmd: 'pasteNames' })
    expect(find(buildMenu(ctx()), 'pasteNames')?.disabled).toBeFalsy()
    expect(find(buildMenu(ctx({ hasSpread: false })), 'pasteNames')?.disabled).toBe(true)
  })

  it('черта никогда не стоит первой', () => {
    for (const m of buildMenu(ctx())) {
      expect(m.items.length).toBeGreaterThan(0)
      expect(m.items[0].sep).toBeFalsy()
    }
  })
})

describe('уровень: точка расширения can()', () => {
  const only = (...allowed: string[]) => ctx({ can: (f) => allowed.includes(f) })

  it('пункт с недоступной функцией скрыт, остальные на месте', () => {
    const m = buildMenu(only())
    for (const id of ['selectAll', 'addText', 'flip', 'snap', 'zoomIn', 'zoomOut', 'allSpreads', 'big'])
      expect(find(m, id), id).toBeUndefined()
    for (const id of ['new', 'undo', 'removeCell', 'fit', 'help']) expect(find(m, id), id).toBeDefined()
  })

  it('функция разрешена — пункт возвращается', () => {
    const m = buildMenu(only('text', 'flip', 'zoom'))
    for (const id of ['addText', 'flip', 'flipH', 'zoomIn', 'zoomOut']) expect(find(m, id), id).toBeDefined()
    expect(find(m, 'snap')).toBeUndefined()
  })

  it('когда скрыта вся группа, её черты нет', () => {
    const m = buildMenu(only())
    const edit = m.find((x) => x.id === 'edit')!
    // Группа «текст/зеркало» скрыта целиком, «Выделить всё» — из средней группы; черта одна.
    expect(edit.items.map((i) => i.id)).toEqual(['undo', 'redo', 'clearSelection', 'removeCell'])
    expect(edit.items.filter((i) => i.sep)).toHaveLength(1)
    const view = m.find((x) => x.id === 'view')!
    // Группы «Уровень», «Во весь экран…» и «Вписать» — две черты.
    expect(view.items.filter((i) => i.sep)).toHaveLength(2)
  })
})

describe('уровень в меню (спека 010 U1)', () => {
  const at = (level: Level) => ctx({ level, can: (f) => can(f, level) })
  const viewIds = (level: Level) => ids(buildMenu(at(level)), 'view')

  it('«Вид ▸ Уровень» — четыре уровня, текущий с галочкой, и «Уровни и функции…»', () => {
    const m = buildMenu(at('standard'))
    const sub = find(m, 'level')!.sub!
    expect(sub.map((i) => i.cmd)).toEqual(['levelSimple', 'levelStandard', 'levelFull', 'levelCustom'])
    expect(sub.map((i) => i.checked)).toEqual([false, true, false, false])
    expect(find(m, 'compareLevels')).toMatchObject({ cmd: 'compareLevels', label: 'Уровни и функции…' })
  })
  it('«Новый проект: виньетка-лист…» — со «Стандартного» (решение 80), рядом с «Новый альбом»', () => {
    expect(ids(buildMenu(at('simple')), 'file')).not.toContain('newSheet')
    const file = ids(buildMenu(at('standard')), 'file')
    expect(file.indexOf('newSheet')).toBe(file.indexOf('new') + 1)
    expect(find(buildMenu(at('standard')), 'newSheet')).toMatchObject({
      label: 'Новый проект: виньетка-лист…',
      cmd: 'newSheet',
      hint: 'Отдельный плакат класса 20×30 или 30×40 — без альбома',
    })
  })
  it('«Файл ▸ Наборы библиотеки…» (067 Ф2) — сразу после «Резервных копий…», со «Стандартного»', () => {
    expect(ids(buildMenu(at('simple')), 'file')).not.toContain('packs')
    const std = buildMenu(at('standard'))
    expect(find(std, 'packs')).toMatchObject({ label: 'Наборы библиотеки…', cmd: 'packs' })
    expect(find(std, 'packs')?.disabled).toBeUndefined()
    const file = ids(std, 'file')
    expect(file.indexOf('packs')).toBe(file.indexOf('backups') + 1)
  })
  it('«Файл ▸ Мои шаблоны ▸» (086) — два пункта, виден со «Стандартного» (виньетка) и на «Полном», на «Простом» скрыт', () => {
    expect(ids(buildMenu(at('simple')), 'file')).not.toContain('tpl')
    for (const l of ['standard', 'full'] as const) {
      const m = buildMenu(at(l))
      const tpl = find(m, 'tpl')
      expect(tpl, l).toMatchObject({ label: 'Мои шаблоны' })
      expect(tpl?.cmd).toBeUndefined()
      expect(tpl?.sub?.map((i) => [i.label, i.cmd])).toEqual([
        ['Сохранить в файл…', 'tplExport'],
        ['Загрузить из файла…', 'tplImport'],
      ])
      const file = ids(m, 'file')
      expect(file.indexOf('tpl')).toBe(file.indexOf('saveLook') + 1)
    }
    // «Свой»: шаблоны сеток без виньетки тоже дают подменю; ни того ни другого — нет
    const only = (on: FeatureId[]): Custom => ({ base: 'simple', on, off: [] })
    const own = (c: Custom) => ids(buildMenu(ctx({ level: 'custom', can: (f) => features('custom', c).has(f) })), 'file')
    expect(own(only(['templates', 'manual']))).toContain('tpl') // templates требует manual
    expect(own(only(['vignette']))).toContain('tpl')
    expect(own(only([]))).not.toContain('tpl')
  })
  it('«Файл ▸ Макеты альбома…» и «Сохранить альбом как макет…» (131) — на всех уровнях, сразу после «Наборов библиотеки»', () => {
    for (const l of ['simple', 'standard', 'full'] as const) {
      const m = buildMenu(at(l))
      expect(find(m, 'looks'), l).toMatchObject({ label: 'Макеты альбома…', cmd: 'looks' })
      expect(find(m, 'saveLook'), l).toMatchObject({ label: 'Сохранить альбом как макет…', cmd: 'saveLook' })
      const file = ids(m, 'file')
      // «Наборы библиотеки» требуют «Фоны и стикеры» (на «Простом» их нет) — тогда макеты идут после «Резервных копий»
      expect(file.indexOf('looks'), l).toBe(file.indexOf(file.includes('packs') ? 'packs' : 'backups') + 1)
      expect(file.indexOf('saveLook'), l).toBe(file.indexOf('looks') + 1)
    }
  })
  it('проект-лист 031 (design §8): «Макеты альбома…» серый, «Сохранить альбом как макет…» серый всегда; у альбома «Макеты» живые', () => {
    for (const l of ['simple', 'standard', 'full'] as const) {
      const sheet = buildMenu({ ...at(l), sheetDoc: true })
      expect(find(sheet, 'looks')?.disabled, l).toBe(true)
      expect(find(sheet, 'saveLook')?.disabled, l).toBe(true)
      const album = buildMenu(at(l))
      expect(find(album, 'looks')?.disabled, l).toBeFalsy()
      expect(enabledCmds(sheet).has('looks'), l).toBe(false) // и клавиша молчит
    }
  })
  it('«Сохранить альбом как макет…» (131 U3): живой с разворотами; серый без разворотов и в проекте-листе', () => {
    const on = (o: object) => find(buildMenu({ ...at('simple'), ...o }), 'saveLook')?.disabled
    expect(on({ spreads: 3 })).toBeFalsy()
    expect(on({ spreads: 0 })).toBe(true)
    expect(on({ spreads: 3, sheetDoc: true })).toBe(true)
    expect(find(buildMenu({ ...at('simple'), spreads: 0 }), 'looks')?.disabled).toBeFalsy() // «Макеты альбома…» не зависит
  })
  it('«Файл ▸ Открыть папку данных» (086) — после «Мои шаблоны», на всех уровнях', () => {
    for (const l of ['simple', 'standard', 'full'] as const) {
      const m = buildMenu(at(l))
      expect(find(m, 'showData'), l).toMatchObject({ label: 'Открыть папку данных', cmd: 'showData' })
      expect(find(m, 'showData')?.disabled).toBeUndefined()
    }
    const full = ids(buildMenu(at('full')), 'file')
    expect(full.indexOf('showData')).toBe(full.indexOf('tplImport') + 1)
    const simple = ids(buildMenu(at('simple')), 'file')
    expect(simple.indexOf('showData')).toBe(simple.indexOf('saveLook') + 1) // нет ни наборов, ни шаблонов
  })
  it('«Правка ▸ Зеркало» (048): включено при выделенном стикере и без фото', () => {
    const base = { ...at('full'), flippable: false }
    expect(find(buildMenu(base), 'flip')?.disabled).toBe(true)
    expect(find(buildMenu({ ...base, sticker: true }), 'flip')?.disabled).toBeFalsy()
    expect(find(buildMenu({ ...base, flippable: true }), 'flip')?.disabled).toBeFalsy()
  })
  it('«Простой»: без зума, «Все развороты», привязки; «Вписать» остаётся', () => {
    const v = viewIds('simple')
    for (const id of ['zoomIn', 'zoomOut', 'allSpreads', 'snap']) expect(v, id).not.toContain(id)
    for (const id of ['big', 'details', 'safe', 'fit', 'level']) expect(v, id).toContain(id)
  })
  it('«Стандартный»: зум и «Все развороты» есть, привязки нет', () => {
    const v = viewIds('standard')
    for (const id of ['zoomIn', 'zoomOut', 'allSpreads']) expect(v, id).toContain(id)
    expect(v).not.toContain('snap')
  })
  it('«Показать все фото» (083): пункт «Вид» со «Стандартного»; на «Простом» его нет', () => {
    expect(viewIds('standard')).toContain('binAll')
    expect(viewIds('simple')).not.toContain('binAll')
    expect(find(buildMenu(at('standard')), 'binAll')?.cmd).toBe('binAll')
  })
  it('«Показать все фото» серое, пока подборка не показана (083-30)', () => {
    expect(find(buildMenu(at('standard')), 'binAll')?.disabled).toBe(true)
    expect(find(buildMenu({ ...at('standard'), binShown: false }), 'binAll')?.disabled).toBe(true)
    expect(find(buildMenu({ ...at('standard'), binShown: true }), 'binAll')?.disabled).toBeFalsy()
  })
  it('«Полный»: привязка и линейки на месте, у линеек галочка и Ctrl+R', () => {
    expect(viewIds('full')).toContain('snap')
    const m = buildMenu(at('full'))
    expect(find(m, 'rulers')).toMatchObject({ cmd: 'toggleRulers', keys: 'Ctrl+R', checked: true })
    expect(find(buildMenu({ ...at('full'), rulers: false }), 'rulers')?.checked).toBe(false)
  })
  it('линеек нет в «Простом» и «Стандартном»', () => {
    for (const l of ['simple', 'standard'] as const) expect(viewIds(l), l).not.toContain('rulers')
  })
  it('«Правка»: «Простой» без зеркала и выделения всего на холсте; «Стандартный» с ними', () => {
    const simple = ids(buildMenu(at('simple')), 'edit')
    for (const id of ['flip', 'selectAll']) expect(simple, id).not.toContain(id)
    const std = ids(buildMenu(at('standard')), 'edit')
    for (const id of ['flip', 'selectAll']) expect(std, id).toContain(id)
  })
  it('«Разворот»: «Простой» без текста, стикера и виньетки; «Стандартный» с ними', () => {
    const simple = ids(buildMenu(at('simple')), 'spread')
    for (const id of ['addText', 'addSticker', 'makeVignette']) expect(simple, id).not.toContain(id)
    const std = ids(buildMenu(at('standard')), 'spread')
    for (const id of ['addText', 'addSticker', 'makeVignette']) expect(std, id).toContain(id)
  })
  it('«Выделить всё» в ленте есть и в «Простом»', () => {
    expect(find(buildMenu(ctx({ ...at('simple'), zone: 'library' })), 'selectAll')?.cmd).toBe('selectAll')
  })
  it('без level галочка на «Полном»', () => {
    const sub = find(buildMenu(ctx()), 'level')!.sub!
    expect(sub.map((i) => i.checked)).toEqual([false, false, true, false])
  })
})

describe('«Свой» в меню (спека 011 U2)', () => {
  const at = (custom: Custom) => ctx({ level: 'custom', can: (f) => features('custom', custom).has(f) })
  const simpleOff: Custom = { base: 'simple', on: [], off: ['timeline', 'turn'] }

  it('галочка на «Своём»', () => {
    const sub = find(buildMenu(at(simpleOff)), 'level')!.sub!
    expect(sub.map((i) => i.checked)).toEqual([false, false, false, true])
  })
  it('без «Разреза и склейки» нет «Во весь экран», на «Простом» он есть', () => {
    expect(find(buildMenu(at(simpleOff)), 'big')).toBeUndefined()
    expect(find(buildMenu(ctx({ level: 'simple', can: (f) => can(f, 'simple') })), 'big')).toBeDefined()
  })
  it('без «Поворота фото» нет пункта «Повернуть», панель деталей и уровень остаются', () => {
    const m = buildMenu(at(simpleOff))
    expect(find(m, 'turn')).toBeUndefined()
    for (const id of ['details', 'level', 'compareLevels']) expect(find(m, id), id).toBeDefined()
  })
  it('«Правка» без функций не пустеет: отмена и «Удалить» всегда', () => {
    const none: Custom = { base: 'simple', on: [], off: ['variants', 'timeline', 'captions', 'turn'] }
    expect(ids(buildMenu(at(none)), 'edit')).toEqual(expect.arrayContaining(['undo', 'redo', 'removeCell']))
  })
  it('«Справка»: «Справочник функций…» первым, с клавишей F1; «Горячие клавиши» — «?»', () => {
    const help = buildMenu(ctx()).find((x) => x.id === 'help')!.items
    expect(help[0]).toMatchObject({ id: 'guide', label: 'Справочник функций…', cmd: 'guide' })
    expect(keysOf('guide')).toBe('F1')
    expect(find(buildMenu(ctx()), 'help')?.cmd).toBe('help')
    expect(keysOf('help')).toBe('?')
  })
  it('«Справка ▸ Что нового» — команда whatsNew', () => {
    expect(find(buildMenu(ctx()), 'whatsNew')?.cmd).toBe('whatsNew')
  })
})

describe('keysOf', () => {
  it('команда без клавиши — undefined', () => {
    expect(keysOf('addFolder')).toBeUndefined()
    expect(keysOf('zoomIn')).toBe('Ctrl+=')
  })
  it('align с параметром — отдельные команды каталога; команды с путём и опорой клавиши не имеют', () => {
    expect(keyId('align', 'hcenter')).toBe('alignHcenter')
    expect(keyId('align', 'distV')).toBe('alignDistV')
    expect(keyId('align')).toBeUndefined()
    for (const c of ['alignTo', 'openRecent', 'clearRecent'] as const) expect(keyId(c, 'x'), c).toBeUndefined()
    expect(keyId('undo')).toBe('undo')
  })
})

describe('enabledCmds: пункт виден и не серый (074-8)', () => {
  it('серый пункт и пункт под серым родителем не считаются', () => {
    const on = enabledCmds(buildMenu(ctx({ flippable: true, alignCount: 3, level: 'full', can: (f) => can(f, 'full') })))
    expect(on.has('flipH')).toBe(true)
    expect(on.has('alignLeft')).toBe(true)
    const off = enabledCmds(buildMenu(ctx({ flippable: false, alignCount: 0, level: 'full', can: (f) => can(f, 'full') })))
    expect(off.has('flipH')).toBe(false)
    expect(off.has('gray')).toBe(false)
    expect(off.has('alignLeft')).toBe(false)
    expect(off.has('save')).toBe(true)
  })
  it('пункт, скрытый уровнем, не считается', () => {
    const simple = enabledCmds(buildMenu(ctx({ level: 'simple', can: (f) => can(f, 'simple') })))
    expect(simple.has('toggleRulers')).toBe(false)
    expect(simple.has('startWindow')).toBe(true)
  })
  it('команды без клавиш (openRecent, alignTo) в набор не попадают', () => {
    const on = enabledCmds(buildMenu(ctx({ alignCount: 3, recent: [{ path: 'a.salbum', name: 'a', exists: true, modified: 1 }] })))
    for (const c of ['openRecent', 'alignTo', 'clearRecent', 'align']) expect(on.has(c), c).toBe(false)
  })
})

describe('«Правка ▸ Выровнять» (спека 010 U7)', () => {
  const full = (o: Partial<MenuCtx> = {}) => ctx({ level: 'full', can: (f) => can(f, 'full'), ...o })
  const sub = (o: Partial<MenuCtx> = {}) => find(buildMenu(full(o)), 'align')!.sub!
  const off = (o: Partial<MenuCtx>) =>
    sub(o)
      .filter((i) => i.cmd === 'align' && i.disabled)
      .map((i) => i.arg)

  it('только «Полный»', () => {
    for (const l of ['simple', 'standard'] as const)
      expect(find(buildMenu(ctx({ level: l, can: (f) => can(f, l) })), 'align'), l).toBeUndefined()
    expect(find(buildMenu(full({ alignCount: 2 })), 'align')).toBeDefined()
  })
  it('8 действий, черта перед распределением и перед опорой; у опоры галочка', () => {
    const s = sub({ alignCount: 3, alignTo: 'page' })
    expect(s.map((i) => i.arg)).toEqual(['left', 'hcenter', 'right', 'top', 'vcenter', 'bottom', 'distH', 'distV', 'sel', 'page', 'spread'])
    expect(s.filter((i) => i.sep).map((i) => i.arg)).toEqual(['distH', 'sel'])
    expect(s.filter((i) => i.checked).map((i) => i.arg)).toEqual(['page'])
    expect(s.every((i) => i.cmd === 'align' || i.cmd === 'alignTo')).toBe(true)
  })
  it('ничего не выделено — пункт серый', () => {
    expect(find(buildMenu(full({ alignCount: 0 })), 'align')?.disabled).toBe(true)
    expect(find(buildMenu(full()), 'align')?.disabled).toBe(true)
  })
  it('пункты по числу выделенных: к выделению 2 и 3', () => {
    const all = ['left', 'hcenter', 'right', 'top', 'vcenter', 'bottom', 'distH', 'distV']
    expect(off({ alignCount: 1, alignTo: 'sel' })).toEqual(all)
    expect(off({ alignCount: 2, alignTo: 'sel' })).toEqual(['distH', 'distV'])
    expect(off({ alignCount: 3, alignTo: 'sel' })).toEqual([])
  })
  it('к странице и развороту: одно — выравнивание, два — и распределение', () => {
    for (const alignTo of ['page', 'spread'] as const) {
      expect(off({ alignCount: 1, alignTo }), alignTo).toEqual(['distH', 'distV'])
      expect(off({ alignCount: 2, alignTo }), alignTo).toEqual([])
    }
  })
  it('Относительно выделения серое у одного (спека 100 п. 4), страницы и разворота — нет', () => {
    const to = (alignCount: number) => sub({ alignCount, alignTo: 'page' }).filter((i) => i.cmd === 'alignTo')
    expect(to(1).map((i) => [i.arg, !!i.disabled])).toEqual([
      ['sel', true],
      ['page', false],
      ['spread', false],
    ])
    expect(to(1)[0].hint).toContain('от 2 объектов')
    expect(to(2).every((i) => !i.disabled)).toBe(true)
  })
})

describe('«Повернуть» (010 U5)', () => {
  it('подменю из ↻ и ↺, серое без выделенного фото, есть на любом уровне', () => {
    const turn = find(buildMenu(ctx()), 'turn')
    expect(turn?.cmd).toBeUndefined()
    expect(turn?.sub?.map((i) => i.cmd)).toEqual(['turnCw', 'turnCcw', 'straighten'])
    expect(find(buildMenu(ctx({ flippable: false })), 'turn')?.disabled).toBe(true)
    for (const l of ['simple', 'standard', 'full'] as const)
      expect(find(buildMenu(ctx({ can: (f) => can(f, l) })), 'turn'), l).toBeDefined()
  })
  it('«Выпрямить по линии» (045) — через линию, только при функции «Наклон фото»', () => {
    const sub = (l: Level) => find(buildMenu(ctx({ can: (f) => can(f, l) })), 'turn')?.sub
    expect(sub('simple')?.map((i) => i.cmd)).toEqual(['turnCw', 'turnCcw'])
    expect(sub('standard')?.map((i) => i.cmd)).toEqual(['turnCw', 'turnCcw', 'straighten'])
    expect(sub('standard')?.find((i) => i.cmd === 'straighten')?.sep).toBe(true)
  })
  it('стоит перед «Зеркало»', () => {
    const edit = ids(buildMenu(ctx()), 'edit')
    expect(edit.indexOf('turn')).toBeLessThan(edit.indexOf('flip'))
  })
})

describe('«Слой» (010 U6)', () => {
  it('подменю из четырёх действий с клавишами из таблицы', () => {
    const layer = find(buildMenu(ctx()), 'layer')
    expect(layer?.cmd).toBeUndefined()
    expect(layer?.sub?.map((i) => i.cmd)).toEqual(['layerFront', 'layerUp', 'layerDown', 'layerBack'])
    expect(layer?.sub?.map((i) => i.keys)).toEqual(['Ctrl+Shift+]', 'Ctrl+]', 'Ctrl+[', 'Ctrl+Shift+['])
  })
  it('серое, когда двигать нечего (не «Ручная» или ничего не выделено)', () => {
    expect(find(buildMenu(ctx({ layerable: false })), 'layer')?.disabled).toBe(true)
    expect(find(buildMenu(ctx({ layerable: undefined })), 'layer')?.disabled).toBe(true)
    expect(find(buildMenu(ctx()), 'layer')?.disabled).toBeUndefined()
  })
  it('есть с уровня, где есть слои, и стоит в «Правка» перед «Повернуть»', () => {
    for (const l of ['simple', 'standard', 'full'] as const)
      expect(!!find(buildMenu(ctx({ can: (f) => can(f, l) })), 'layer'), l).toBe(can('layers', l))
    const edit = ids(buildMenu(ctx()), 'edit')
    expect(edit.indexOf('layer')).toBeLessThan(edit.indexOf('turn'))
  })
})

describe('«Сделать ч/б / цветным» (068)', () => {
  it('в «Правка» сразу после «Зеркало», команда gray', () => {
    const edit = ids(buildMenu(ctx()), 'edit')
    expect(edit.indexOf('gray')).toBeGreaterThan(edit.indexOf('flip'))
    expect(find(buildMenu(ctx()), 'gray')?.cmd).toBe('gray')
  })
  it('подпись по состоянию выделенных', () => {
    expect(find(buildMenu(ctx()), 'gray')?.label).toBe('Сделать ч/б')
    expect(find(buildMenu(ctx({ allGray: false })), 'gray')?.label).toBe('Сделать ч/б')
    expect(find(buildMenu(ctx({ allGray: true })), 'gray')?.label).toBe('Сделать цветным')
  })
  it('серое, когда фото не выделено', () => {
    expect(find(buildMenu(ctx({ flippable: false })), 'gray')?.disabled).toBe(true)
    expect(find(buildMenu(ctx()), 'gray')?.disabled).toBeFalsy()
  })
  it('со «Стандартного»: «Простой» пункта не имеет, «Свой» с включённой функцией — имеет', () => {
    const at = (l: Level) => ids(buildMenu(ctx({ can: (f) => can(f, l) })), 'edit')
    expect(at('simple')).not.toContain('gray')
    expect(at('standard')).toContain('gray')
    const custom: Custom = { base: 'simple', on: ['adjust'], off: [] }
    expect(ids(buildMenu(ctx({ can: (f) => features('custom', custom).has(f) })), 'edit')).toContain('gray')
  })
})
