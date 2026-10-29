import { beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «Папки» (093): вопрос перед отвязкой, подсветка и тост по событию folders:added. Go подменён; DOM в тестах нет —
// разметку FolderMenu/FolderRow проверяем по исходнику.
const calls: string[] = []
const handlers = new Map<string, (e: unknown) => void>()
const ROOT = 'E:\\Съёмки\\Весна 2026'
const FAV = ROOT + '\\Март 2026\\Избранное Март'
// Отдельные фото (097): добавление b9 — два файла, один (l1) в ленте.
const LB = {
  id: 'b9',
  at: 1_792_000_000,
  files: [
    { path: 'C:\\Загрузки\\bouquet.jpg', photo: 'l1' },
    { path: 'C:\\Загрузки\\ring.jpg', photo: 'l2' },
  ],
}
// Ответ ChooseFiles: по умолчанию — новое добавление b9; тест может подменить (отмена окна).
const added = { batch: 'b9', added: 2, inFolder: 1, folder: ROOT, dup: 0, skipped: 0, bad: 0, over: 0 }
let filesAnswer = (): object => ({ view: view(4, { loose: [LB] }), ...added })
const view = (rev: number, extra: object = {}) => ({ rev, marksRev: 1, folders: [ROOT, 'D:/b'], spreads: [], bins: [], ...extra })

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, f: (e: unknown) => void) => {
    handlers.set(name, f)
    return () => handlers.delete(name)
  },
}))
vi.mock('../../../wailsjs/go/app/App', () => ({
  Marks: async () => ({}),
  RemoveFolder: async (dir: string) => {
    calls.push('remove ' + dir)
    return view(2, { folders: ['D:/b'] })
  },
  RemoveLoose: async (id: string, file: string) => {
    calls.push(`loose ${id} ${file}`)
    return view(2, { loose: [] })
  },
  ChooseFiles: async () => {
    calls.push('files')
    return filesAnswer()
  },
  ChooseFolder: async () => {
    calls.push('choose')
    return view(3, { folders: [ROOT, 'D:/b', 'D:/new'] })
  },
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../../shell/boot/wire')
const { foldersUi } = await import('./foldersui.svelte')
const { folderfilter } = await import('./folderfilter.svelte')
const { buttonText, buttonTitle, findNode, nodeKey } = await import('./tree')
const { exportsText } = await import('./scan')
const { GROUP_KEY, batchKey, fileKey } = await import('./loose')

const ph = (id: string, dir: string) => ({ id, path: `${dir}\\${id}.jpg`, name: id, w: 100, h: 100 })
const ids = (l: { id: string }[]) => l.map((p) => p.id)
const node = (dir: string) => findNode(foldersUi.forest(), nodeKey(dir))!
wire()
const off = foldersUi.listen()

beforeEach(() => {
  calls.length = 0
  folderfilter.reset()
  foldersUi.open = false
  foldersUi.ask = null
  foldersUi.flash = null
  store.toast = null
  store.photos = [ph('s1', ROOT), ph('m1', ROOT + '\\Март 2026'), ph('f1', FAV), ph('f2', FAV), ph('b1', 'D:\\b')] as never
  store.apply(view(1) as never)
})

describe('✕ и вопрос «Убрать папку?»', () => {
  it('x only own: ✕ — только у добавленной папки, у подпапки место под него', () => {
    const files = import.meta.glob('/src/panels/folders/FolderRow.svelte', { query: '?raw', import: 'default', eager: true })
    const src = Object.values(files)[0] as string
    const body = src.replace(/<script[\s\S]*?<\/script>/, '')
    expect(body).toMatch(
      /\{#if node\.root\}\s*<button[\s\S]*?class="ghost x"[\s\S]*?\{:else\}\s*(<!--[\s\S]*?-->\s*)?<span class="xs"><\/span>/,
    )
    expect(node(ROOT).root).toBe(true)
    expect(node(FAV).root).toBe(false)
  })

  it('ask when in album: фото папки стоят в альбоме — вопрос; «Отмена» не меняет альбом', () => {
    store.apply(view(5, { spreads: [{ id: 's', photos: ['f1', 'm1', 'b1'] }] }) as never)
    foldersUi.remove(node(ROOT))
    expect(calls).toEqual([])
    expect(foldersUi.ask?.text).toBe(
      'В альбоме стоят 2 фото из «Весна 2026» и её подпапок. На листах они останутся, а из ленты пропадут все 4 фото папки.',
    )
    foldersUi.cancel()
    expect(foldersUi.ask).toBeNull()
    expect(calls).toEqual([])
    expect(store.view.rev).toBe(5)
  })

  it('«Убрать» в вопросе — RemoveFolder с путём из списка папок', async () => {
    store.apply(view(5, { backdropUsed: ['s1'] }) as never) // фото на фоне — тоже «в альбоме»
    foldersUi.remove(node(ROOT))
    expect(foldersUi.ask).not.toBeNull()
    await foldersUi.confirm()
    expect(calls).toEqual(['remove ' + ROOT])
    expect(foldersUi.ask).toBeNull()
  })

  it('no ask when free: фото папки не в альбоме — убирается сразу', () => {
    store.apply(view(5, { spreads: [{ id: 's', photos: ['f1'] }] }) as never)
    foldersUi.remove(node('D:/b'))
    expect(foldersUi.ask).toBeNull()
    expect(calls).toEqual(['remove D:/b'])
  })
})

describe('событие folders:added', () => {
  const added = (e: object) => handlers.get('folders:added')?.({ merged: 0, looseIn: 0, ...e })

  it('reveal within: подпапка уже добавленной — окно открыто, предки раскрыты, строка мигает, «Только её»', () => {
    added({ dir: FAV, within: ROOT })
    expect(foldersUi.open).toBe(true)
    expect(foldersUi.flash).toBe(nodeKey(FAV))
    expect(folderfilter.isOpen(node(ROOT + '\\Март 2026'))).toBe(true)
    expect(store.toast?.text).toBe('«Избранное Март» уже входит в «Весна 2026» — отмечена в списке')
    expect(store.toast?.action?.label).toBe('Только её')
    store.toast?.action?.run()
    expect(ids(store.visible)).toEqual(['f1', 'f2'])
    expect(store.view.rev).toBe(1) // холостая: Go проект не менял, UI тоже
  })

  it('подпапка без фото — мигает её папка, без «отмечена» и без «Только её»', () => {
    added({ dir: ROOT + '\\Пусто', within: ROOT })
    expect(foldersUi.flash).toBe(nodeKey(ROOT))
    expect(store.toast?.text).toBe('«Пусто» уже входит в «Весна 2026»')
    expect(store.toast?.action).toBeUndefined()
  })

  it('та же папка ещё раз — окно на ней, тост без действия', () => {
    added({ dir: ROOT.toLowerCase(), within: ROOT })
    expect(foldersUi.open).toBe(true)
    expect(foldersUi.flash).toBe(nodeKey(ROOT))
    expect(store.toast?.text).toBe('«Весна 2026» уже добавлена — отмечена в списке')
  })

  it('новая папка броском — окно не открывается и тоста нет; поглотила прежние — тост', () => {
    added({ dir: 'D:/new', within: '' })
    expect(foldersUi.open).toBe(false)
    expect(store.toast).toBeNull()
    added({ dir: 'E:\\Съёмки', within: '', merged: 2 })
    expect(store.toast?.text).toBe('Папка «Съёмки» добавлена — в неё вошли прежние папки: 2')
  })

  it('«+ Добавить папку…» из окна — окно открывается на новой папке', async () => {
    foldersUi.open = true
    const p = foldersUi.add()
    expect(foldersUi.open).toBe(false) // под системный выбор
    added({ dir: 'D:/new', within: '' })
    await p
    expect(calls).toEqual(['choose'])
    expect(foldersUi.open).toBe(true)
    expect(foldersUi.flash).toBe(nodeKey('D:/new'))
  })

  it('exports:skipped (121): событие → тост info; n = 0 и пустое событие — без тоста', () => {
    const fire = handlers.get('exports:skipped')!
    fire({ n: 0, dir: 'ФА25' })
    fire(undefined)
    fire(null)
    expect(store.toast).toBeNull()
    fire({ n: 3, dir: 'ФА25' })
    expect(store.toast?.text).toBe(exportsText(3, 'ФА25'))
    expect(store.toast?.kind).toBe('info')
  })

  it('отписка снимает обработчик', () => {
    off()
    expect(handlers.has('folders:added')).toBe(false)
    expect(handlers.has('exports:skipped')).toBe(false)
    foldersUi.listen()
  })
})

describe('button label', () => {
  it('«Папки 3 · фильтр» и подсказка при скрытом; «Папки 3» без', () => {
    expect(buttonText(3, true)).toBe('Папки 3 · фильтр')
    expect(buttonText(3, false)).toBe('Папки 3')
    expect(buttonTitle(162, 253, true)).toBe('В ленте 162 из 253 фото: часть папок скрыта')
    expect(buttonTitle(253, 253, false)).toBe('Папки с фото: добавить, убрать, перечитать')
  })

  it('кнопка жёлтая и ✕ «Показать все папки» — по ленте, а не по набору ключей', () => {
    const files = import.meta.glob('/src/panels/folders/FolderMenu.svelte', { query: '?raw', import: 'default', eager: true })
    const src = Object.values(files)[0] as string
    expect(src).toContain('folderfilter.active && store.inFolders.length < store.photos.length')
    expect(src).toMatch(/class:active=\{filtered\}/)
    expect(src).toMatch(/\{#if filtered\}\s*<button[\s\S]*?title="Показать все папки"/)
  })
})

describe('отдельные фото (097)', () => {
  const l1 = { id: 'l1', path: LB.files[0].path, name: 'bouquet', w: 100, h: 100 }
  const byKey = (key: string) => findNode(foldersUi.forest(), key)!
  const withLoose = (rev: number, extra: object = {}) => {
    store.photos = [...store.photos.filter((p) => p.id !== 'l1'), l1] as never
    store.apply(view(rev, { loose: [LB], ...extra }) as never)
  }

  it('add photos button: две кнопки в порядке «папку», «фото»; вторая зовёт addFiles', () => {
    const files = import.meta.glob('/src/panels/folders/FolderMenu.svelte', { query: '?raw', import: 'default', eager: true })
    const src = Object.values(files)[0] as string
    expect(src).toMatch(/foldersUi\.add\(\)[\s\S]*?Добавить папку…[\s\S]*?foldersUi\.addFiles\(\)[\s\S]*?Добавить фото…/)
  })

  it('«+ Добавить фото…»: ChooseFiles один раз, окно открыто на новом добавлении, ветка и оно раскрыты, тост с пропуском', async () => {
    foldersUi.open = true
    store.photos = [...store.photos, l1] as never
    const p = foldersUi.addFiles()
    expect(foldersUi.open).toBe(false) // под системный выбор
    await p
    expect(calls).toEqual(['files'])
    expect(store.view.rev).toBe(4)
    expect(foldersUi.open).toBe(true)
    expect(foldersUi.flash).toBe(batchKey('b9'))
    expect(folderfilter.isOpen(byKey(GROUP_KEY))).toBe(true)
    expect(folderfilter.isOpen(byKey(batchKey('b9')))).toBe(true)
    expect(store.toast?.text).toBe('Добавлено 2 фото · 1 уже есть в папке «Весна 2026» — не добавлено')
  })

  it('отмена системного окна — вид тот же, окна и тоста нет', async () => {
    const was = filesAnswer
    filesAnswer = () => ({ view: view(1), ...added, batch: '', added: 0, inFolder: 0 })
    try {
      await foldersUi.addFiles()
    } finally {
      filesAnswer = was
    }
    expect(foldersUi.open).toBe(false)
    expect(store.toast).toBeNull()
    expect(store.view.rev).toBe(1)
  })

  it('событие loose:added (бросок): окно не открывается, ветка и добавление раскрыты, тост', () => {
    withLoose(4)
    handlers.get('loose:added')?.(added)
    expect(foldersUi.open).toBe(false)
    expect(folderfilter.isOpen(byKey(batchKey('b9')))).toBe(true)
    expect(store.toast?.text).toContain('Добавлено 2 фото')
    handlers.get('loose:added')?.({ ...added, batch: '', added: 0, inFolder: 0, skipped: 3 })
    expect(store.toast?.text).toBe('Не фото: 3')
  })

  it('x on batch and file: файл не в альбоме — RemoveLoose сразу; добавление со стоящим фото — вопрос «Убрать фото?», «Убрать» — один вызов', async () => {
    withLoose(5, { spreads: [{ id: 's', photos: ['l1'] }] })
    foldersUi.remove(byKey(fileKey(LB.files[1].path)))
    expect(foldersUi.ask).toBeNull()
    expect(calls).toEqual(['loose b9 ' + LB.files[1].path])
    calls.length = 0
    withLoose(6, { spreads: [{ id: 's', photos: ['l1'] }] })
    foldersUi.remove(byKey(batchKey('b9')))
    expect(calls).toEqual([])
    expect(foldersUi.ask?.title).toBe('Убрать фото?')
    expect(foldersUi.ask?.text).toMatch(/^1 из 2 фото, добавленных .+, стоит в альбоме\. На листе оно останется/)
    await foldersUi.confirm()
    expect(calls).toEqual(['loose b9 '])
    expect(foldersUi.ask).toBeNull()
  })

  it('файл на фоне — тоже «в альбоме»: вопрос про файл; «Отмена» альбом не меняет', () => {
    withLoose(7, { backdropUsed: ['l1'] })
    foldersUi.remove(byKey(fileKey(LB.files[0].path)))
    expect(foldersUi.ask?.text).toBe('Фото «bouquet.jpg» стоит в альбоме. На листе оно останется, а из ленты пропадёт.')
    foldersUi.cancel()
    expect(calls).toEqual([])
    expect(store.view.rev).toBe(7)
  })

  it('Ctrl+Z (вид без добавления и обратно): ветка пропадает и возвращается; скрытый файл забыт вместе с добавлением', () => {
    withLoose(8)
    folderfilter.toggle(byKey(fileKey(LB.files[0].path)))
    expect(ids(store.inFolders)).not.toContain('l1')
    store.apply(view(9) as never) // отмена добавления
    expect(foldersUi.forest().some((n) => n.key === GROUP_KEY)).toBe(false)
    expect(folderfilter.active).toBe(false)
    store.apply(view(10, { loose: [LB] }) as never) // повтор
    expect(byKey(GROUP_KEY).total).toBe(1)
    expect(ids(store.inFolders)).toContain('l1')
  })

  it('«только» у добавления: в ленте только его фото', () => {
    withLoose(11)
    folderfilter.only(foldersUi.forest(), byKey(batchKey('b9')))
    expect(ids(store.inFolders)).toEqual(['l1'])
  })
})
