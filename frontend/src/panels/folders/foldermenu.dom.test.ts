// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Go подменён: из окна «Папки» (093, 097) зовутся RemoveFolder, ChooseFolder, ChooseFiles, RemoveLoose и Rescan. Окно и вопрос «Убрать папку?»
// живут в разных местах приложения (FolderMenu в панели, RemoveAsk в App) — монтируем оба, как в программе.
const removeFolder = vi.hoisted(() => vi.fn())
const chooseFolder = vi.hoisted(() => vi.fn())
const chooseFiles = vi.hoisted(() => vi.fn())
const removeLoose = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  RemoveFolder: removeFolder,
  ChooseFolder: chooseFolder,
  ChooseFiles: chooseFiles,
  RemoveLoose: removeLoose,
  Rescan: vi.fn(),
}))

const { default: FolderMenu } = await import('./FolderMenu.svelte')
const { default: RemoveAsk } = await import('./RemoveAsk.svelte')
const { store } = await import('../../base/core/store.svelte')
const { folderfilter } = await import('./folderfilter.svelte')
const { foldersUi } = await import('./foldersui.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { wire } = await import('../../shell/boot/wire')

wire() // хуки верха: store.inFolders режется фильтром папок

const A = 'D:/Свадьба/Сборы'
const B = 'D:/Свадьба/Церемония'
const photo = (id: string, dir: string) => ({ id, path: `${dir}/${id}.jpg` })

// Фото по папкам: две в каждой, id — p<номер папки><a|b>; `onSheets` — id, стоящие на листах альбома.
function setPhotos(dirs: string[], folders: string[], onSheets: string[] = []) {
  store.photos = dirs.flatMap((f, i) => [photo(`p${i}a`, f), photo(`p${i}b`, f)]) as never
  const spreads = onSheets.length ? [{ id: 's1', photos: onSheets, rects: [], w: 400, h: 200, name: '', kind: '' }] : []
  store.view = { ...store.view, rev: store.view.rev + 1, folders, spreads, loose: [] } as never
}
const setFolders = (folders: string[], onSheets: string[] = []) => setPhotos(folders, folders, onSheets)

const button = () => screen.getByRole('button', { name: /^Папки/ })
const popup = () => screen.queryByRole('dialog', { name: 'Папки с фото' })
const ask = () => screen.queryByRole('dialog', { name: 'Убрать папку?' })
const box = (name: string) => screen.getByRole('checkbox', { name: new RegExp(`«${name}»`) }) as HTMLInputElement
const status = () => screen.getByRole('status').textContent

// Окно и вопрос вместе, как в программе.
function mount() {
  const r = render(FolderMenu)
  render(RemoveAsk)
  return r
}

beforeEach(() => {
  // Синглтоны переживают тест: сбрасываем всё, что меняют окно, фильтр и вопрос.
  folderfilter.reset()
  foldersUi.open = false
  foldersUi.ask = null
  foldersUi.flash = null
  modes.big = false
  setFolders([A, B])
})
afterEach(() => {
  cleanup()
  removeFolder.mockReset()
  chooseFolder.mockReset()
  chooseFiles.mockReset()
  removeLoose.mockReset()
})

describe('окно «Папки»: открытие и закрытие', () => {
  it('без папок — кнопка «+ Папка с фото» вместо окна', () => {
    setFolders([])
    mount()
    expect(screen.getByText('+ Папка с фото')).toBeTruthy()
    expect(popup()).toBeNull()
  })

  it('щелчок по кнопке открывает окно (в body), повторный — закрывает', async () => {
    mount()
    await fireEvent.click(button())
    expect(popup()).toBeTruthy()
    expect(popup()!.parentElement).toBe(document.body) // портал
    expect(button().getAttribute('aria-expanded')).toBe('true')
    await fireEvent.click(button())
    flushSync()
    expect(popup()).toBeNull()
  })

  it('Esc и щелчок снаружи закрывают; повторное открытие работает', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(popup()).toBeNull()
    await fireEvent.click(button())
    expect(popup()).toBeTruthy()
    await fireEvent.pointerDown(document.body)
    expect(popup()).toBeNull()
  })

  it('щелчок внутри окна его не закрывает', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.pointerDown(box('Сборы'))
    expect(popup()).toBeTruthy()
  })

  it('крупный просмотр (modes.big) закрывает открытое окно', async () => {
    mount()
    await fireEvent.click(button())
    modes.big = true
    flushSync()
    expect(popup()).toBeNull()
  })

  it('«Добавить папку…» закрывает окно и зовёт выбор папки один раз', async () => {
    chooseFolder.mockResolvedValue({ ...store.view, rev: store.view.rev + 1 })
    mount()
    await fireEvent.click(button())
    await fireEvent.click(screen.getByRole('button', { name: /Добавить папку/ }))
    expect(popup()).toBeNull()
    expect(chooseFolder).toHaveBeenCalledTimes(1)
  })
})

describe('окно «Папки»: флажки «показывать в ленте»', () => {
  it('снять флажок: кнопка «Папки 2 · фильтр», строка серая, счёт ленты меняется; вернуть — всё как было', async () => {
    mount()
    await fireEvent.click(button())
    expect(status()).toBe('В ленте 4 из 4 фото')
    await fireEvent.click(box('Сборы'))
    expect(box('Сборы').checked).toBe(false)
    expect(button().textContent).toContain('Папки 2 · фильтр')
    expect(button().className).toContain('active')
    expect(status()).toBe('В ленте 2 из 4 фото')
    expect(box('Сборы').closest('li')!.className).toContain('off')
    await fireEvent.click(box('Сборы'))
    expect(button().textContent).not.toContain('фильтр')
    expect(button().className).not.toContain('active')
    expect(status()).toBe('В ленте 4 из 4 фото')
  })

  it('«Показать все» серая, пока всё видно; после скрытия сбрасывает фильтр', async () => {
    mount()
    await fireEvent.click(button())
    const all = screen.getByRole('button', { name: 'Показать все' }) as HTMLButtonElement
    expect(all.disabled).toBe(true)
    await fireEvent.click(box('Церемония'))
    expect(all.disabled).toBe(false)
    await fireEvent.click(all)
    expect(box('Церемония').checked).toBe(true)
    expect(all.disabled).toBe(true)
  })

  it('крестик рядом с кнопкой сбрасывает фильтр и пропадает', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.click(box('Сборы'))
    await fireEvent.click(screen.getByRole('button', { name: 'Показать все папки' }))
    expect(folderfilter.active).toBe(false)
    expect(screen.queryByRole('button', { name: 'Показать все папки' })).toBeNull()
  })

  it('«только»: в ленте остаётся одна папка; «Показать все» возвращает остальные', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.click(screen.getByTitle('Показать в ленте только «Церемония»'))
    expect(box('Сборы').checked).toBe(false)
    expect(box('Церемония').checked).toBe(true)
    expect(status()).toBe('В ленте 2 из 4 фото')
    await fireEvent.click(screen.getByRole('button', { name: 'Показать все' }))
    expect(status()).toBe('В ленте 4 из 4 фото')
  })

  it('одна папка — флажков и «только» нет; убрали папку из вида — скрытое забыто (prune)', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.click(box('Сборы'))
    expect(folderfilter.active).toBe(true)
    store.apply({ ...store.view, rev: store.view.rev + 1, folders: [B] } as never) // настоящий путь: хук applied → prune
    await vi.waitFor(() => expect(folderfilter.active).toBe(false))
    expect(screen.queryByRole('checkbox')).toBeNull()
    expect(screen.queryByTitle(/Показать в ленте только/)).toBeNull()
  })

  it('папка без фото (идёт чтение) — флажок серый', async () => {
    setPhotos([A, B], [A, B, 'D:/Свадьба/Банкет'])
    mount()
    await fireEvent.click(button())
    expect(box('Банкет').disabled).toBe(true)
    expect(box('Сборы').disabled).toBe(false)
  })
})

describe('окно «Папки»: дерево с подпапками', () => {
  const P1 = `${A}/Часть 1`
  const P2 = `${A}/Часть 2`
  beforeEach(() => setPhotos([P1, P2, B], [A, B]))

  it('корень раскрыт, подпапки видны; ▸ сворачивает и раскрывает', async () => {
    mount()
    await fireEvent.click(button())
    expect(screen.getByRole('checkbox', { name: /«Часть 1»/ })).toBeTruthy()
    await fireEvent.click(screen.getAllByRole('button', { name: 'Свернуть' })[0])
    expect(screen.queryByRole('checkbox', { name: /«Часть 1»/ })).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Раскрыть' }))
    expect(screen.getByRole('checkbox', { name: /«Часть 1»/ })).toBeTruthy()
  })

  it('скрыли одну подпапку — у родителя «—», его флажок показывает всё; ещё щелчок — скрывает всё поддерево', async () => {
    mount()
    await fireEvent.click(button())
    await fireEvent.click(box('Часть 1'))
    expect(box('Сборы').indeterminate).toBe(true)
    expect(box('Сборы').checked).toBe(false)
    expect(status()).toBe('В ленте 4 из 6 фото')
    await fireEvent.click(box('Сборы')) // «—» → всё поддерево на месте
    expect(box('Сборы').indeterminate).toBe(false)
    expect(box('Сборы').checked).toBe(true)
    expect(status()).toBe('В ленте 6 из 6 фото')
    await fireEvent.click(box('Сборы')) // галочка → всё поддерево скрыто
    expect(box('Часть 1').checked).toBe(false)
    expect(box('Часть 2').checked).toBe(false)
    expect(status()).toBe('В ленте 2 из 6 фото')
  })
})

describe('«Убрать папку»: вопрос, если фото стоят в альбоме', () => {
  const okAnswer = async () => ({ ...store.view, rev: store.view.rev + 1, folders: [B] })
  const crossA = () => screen.getByRole('button', { name: /Убрать папку «Сборы»/ })

  it('нет фото на листах — Go зовётся сразу и один раз, вопроса нет; без папки остаётся одна, флажков нет', async () => {
    removeFolder.mockImplementation(okAnswer)
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    expect(removeFolder).toHaveBeenCalledExactlyOnceWith(A)
    expect(ask()).toBeNull()
    await vi.waitFor(() => expect(screen.queryByRole('checkbox')).toBeNull())
  })

  it('фото стоит на листе — вопрос с числом, фокус на «Отмена», Go не звали', async () => {
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await tick()
    expect(ask()).toBeTruthy()
    expect(ask()!.textContent).toContain('В альбоме стоит 1 фото из «Сборы»')
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Отмена' }))
    expect(removeFolder).not.toHaveBeenCalled()
  })

  it('«Отмена»: вопрос ушёл, окно «Папки» осталось, Go не звали; повторный ✕ спрашивает снова', async () => {
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await fireEvent.click(screen.getByRole('button', { name: 'Отмена' }))
    expect(ask()).toBeNull()
    expect(popup()).toBeTruthy()
    expect(removeFolder).not.toHaveBeenCalled()
    await fireEvent.click(crossA())
    expect(ask()).toBeTruthy()
  })

  it('«Убрать»: один вызов Go с путём папки, вопрос ушёл', async () => {
    removeFolder.mockImplementation(okAnswer)
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await fireEvent.click(screen.getByRole('button', { name: 'Убрать' }))
    expect(removeFolder).toHaveBeenCalledExactlyOnceWith(A)
    expect(ask()).toBeNull()
  })

  it('Esc закрывает только вопрос: окно «Папки» остаётся; второй Esc закрывает и его', async () => {
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await tick()
    await fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(ask()).toBeNull()
    expect(popup()).toBeTruthy()
    expect(removeFolder).not.toHaveBeenCalled()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(popup()).toBeNull()
  })

  it('щелчок внутри вопроса не закрывает окно «Папки» под ним', async () => {
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await fireEvent.pointerDown(screen.getByText(/В альбоме стоит/))
    await fireEvent.pointerDown(screen.getByRole('button', { name: 'Отмена' }))
    expect(popup()).toBeTruthy()
    expect(ask()).toBeTruthy()
  })

  // Находка ревью 093: data-barpop стоял только на теле вопроса — заголовок, ✕ и поля окна закрывали «Папки» под ним.
  it('щелчок по заголовку вопроса, его полям, ✕ и тёмному фону не закрывает окно «Папки»', async () => {
    setFolders([A, B], ['p0a'])
    mount()
    await fireEvent.click(button())
    await fireEvent.click(crossA())
    await fireEvent.pointerDown(screen.getByText('Убрать папку?')) // заголовок
    await fireEvent.pointerDown(ask()!) // поля окна (.box)
    expect(popup()).toBeTruthy()
    const x = screen.getByRole('button', { name: 'Закрыть' })
    await fireEvent.pointerDown(x)
    expect(popup()).toBeTruthy()
    await fireEvent.click(x) // ✕ закрывает только вопрос
    expect(ask()).toBeNull()
    expect(popup()).toBeTruthy()
    expect(removeFolder).not.toHaveBeenCalled()
    await fireEvent.click(crossA())
    await fireEvent.pointerDown(ask()!.parentElement!) // тёмный фон: закрывает вопрос, окно остаётся
    expect(ask()).toBeNull()
    expect(popup()).toBeTruthy()
  })
})

describe('отдельные фото (097)', () => {
  const L1 = 'C:/Загрузки/bouquet.jpg'
  const L2 = 'C:/Загрузки/ring.jpg'
  const GONE = 'D:/Телефон/gone.jpg'
  const at = Math.floor(Date.now() / 1000) - 60
  const LOOSE = [
    {
      id: 'b1',
      at,
      files: [
        { path: L1, photo: 'l1' },
        { path: L2, photo: 'l2' },
      ],
    },
    { id: 'b2', at: at - 86400 * 400, files: [{ path: GONE, photo: 'l3' }] },
  ]
  const askPhoto = () => screen.queryByRole('dialog', { name: 'Убрать фото?' })

  // Папки и отдельные фото: l1, l2 в ленте, l3 — нет на диске (чтение кончилось).
  function setLoose(folders: string[], loose: object[] = LOOSE, onSheets: string[] = []) {
    const lp = [
      { id: 'l1', path: L1 },
      { id: 'l2', path: L2 },
    ]
    store.photos = [...folders.flatMap((f, i) => [photo(`p${i}a`, f), photo(`p${i}b`, f)]), ...lp] as never
    store.scan = { active: false, walking: false, found: 0, read: 0 }
    const spreads = onSheets.length ? [{ id: 's1', photos: onSheets, rects: [], w: 400, h: 200, name: '', kind: '' }] : []
    store.view = { ...store.view, rev: store.view.rev + 1, folders, spreads, loose } as never
  }

  it('без папок, но с отдельными — кнопка «Папки» без числа; ветка без черты сверху', async () => {
    setLoose([])
    mount()
    expect(screen.queryByText('+ Папка с фото')).toBeNull()
    expect(button().textContent!.trim()).toBe('Папки')
    await fireEvent.click(button())
    const group = screen.getByRole('checkbox', { name: /«Фото отдельно»/ }).closest('li')!
    expect(group.className).not.toContain('sep')
  })

  it('с папками — ветка «Фото отдельно» последней, через черту; добавление свёрнуто, ▸ раскрывает файлы', async () => {
    setLoose([A])
    mount()
    await fireEvent.click(button())
    const items = [...popup()!.querySelectorAll('li')]
    const group = screen.getByRole('checkbox', { name: /«Фото отдельно»/ }).closest('li')!
    expect(items.indexOf(group)).toBeGreaterThan(0)
    expect(group.className).toContain('sep')
    expect(screen.queryByRole('checkbox', { name: /«bouquet\.jpg»/ })).toBeNull() // добавление свёрнуто
    await fireEvent.click(screen.getAllByRole('button', { name: 'Раскрыть' })[0])
    const file = screen.getByRole('checkbox', { name: /«bouquet\.jpg»/ }).closest('li')!
    expect(file.textContent).toContain('Загрузки') // бледная папка-источник
    expect(file.title).toBe(L1)
    expect(screen.getByRole('button', { name: 'Убрать «bouquet.jpg» из ленты' }).title).toBe('Убрать из ленты')
  })

  it('missing file: «нет на диске», курсив-класс, флажок серый', async () => {
    setLoose([A])
    mount()
    await fireEvent.click(button())
    const old = screen.getAllByRole('button', { name: 'Раскрыть' })
    await fireEvent.click(old[old.length - 1]) // давнее добавление b2
    const row = screen.getByRole('checkbox', { name: /«gone\.jpg»/ }).closest('li')!
    expect(row.className).toContain('missing')
    expect(row.textContent).toContain('нет на диске')
    expect((screen.getByRole('checkbox', { name: /«gone\.jpg»/ }) as HTMLInputElement).disabled).toBe(true)
  })

  it('unreadable file (117): «не читается» вместо «нет на диске», подсказка с причиной, ✕ на месте', async () => {
    setLoose([A])
    store.unread = { l3: 'внутри HEIC, а не JPEG. Совет из Go' }
    mount()
    await fireEvent.click(button())
    const old = screen.getAllByRole('button', { name: 'Раскрыть' })
    await fireEvent.click(old[old.length - 1])
    const row = screen.getByRole('checkbox', { name: /«gone\.jpg»/ }).closest('li')!
    expect(row.className).toContain('missing') // тот же бледный курсив
    expect(row.textContent).toContain('не читается')
    expect(row.textContent).not.toContain('нет на диске')
    expect(row.title).toBe(GONE + '\nНе читается: внутри HEIC, а не JPEG. Совет из Go')
    expect(screen.getByRole('button', { name: 'Убрать «gone.jpg» из ленты' })).toBeTruthy()
    store.unread = {}
  })

  it('«+ Добавить фото…»: окно закрывается под выбор, ChooseFiles один раз, затем окно открыто на мигающем добавлении', async () => {
    setLoose([A], [])
    chooseFiles.mockResolvedValue({
      view: { ...store.view, rev: store.view.rev + 1, loose: [LOOSE[0]] },
      batch: 'b1',
      added: 2,
      inFolder: 0,
      folder: '',
      dup: 0,
      skipped: 0,
      bad: 0,
      over: 0,
    })
    mount()
    await fireEvent.click(button())
    await fireEvent.click(screen.getByRole('button', { name: /Добавить фото/ }))
    expect(chooseFiles).toHaveBeenCalledTimes(1)
    await vi.waitFor(() => expect(popup()).toBeTruthy())
    const file = screen.getByRole('checkbox', { name: /«ring\.jpg»/ }).closest('li')! // добавление раскрыто
    const batch = file.previousElementSibling!.previousElementSibling!
    expect(batch.className).toContain('flash')
    expect(store.toast?.text).toBe('Добавлено 2 фото')
  })

  it('✕ у добавления со стоящим фото — «Убрать фото?»; щелчок по вопросу окно не закрывает; «Убрать» — RemoveLoose(id, "")', async () => {
    setLoose([A], LOOSE, ['l1'])
    removeLoose.mockResolvedValue({ ...store.view, rev: store.view.rev + 1, loose: [LOOSE[1]] })
    mount()
    await fireEvent.click(button())
    const xs = screen.getAllByRole('button', { name: /из ленты$/ })
    await fireEvent.click(xs[0]) // первое добавление
    await tick()
    expect(askPhoto()!.textContent).toMatch(/1 из 2 фото, добавленных сегодня в \d\d:\d\d, стоит в альбоме/)
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Отмена' }))
    await fireEvent.pointerDown(screen.getByText('Убрать фото?'))
    expect(popup()).toBeTruthy()
    await fireEvent.click(screen.getByRole('button', { name: 'Убрать' }))
    expect(removeLoose).toHaveBeenCalledExactlyOnceWith('b1', '')
    expect(askPhoto()).toBeNull()
  })

  it('✕ у файла не из альбома — RemoveLoose(id, путь) сразу, вопроса нет', async () => {
    setLoose([A])
    removeLoose.mockResolvedValue({ ...store.view, rev: store.view.rev + 1 })
    mount()
    await fireEvent.click(button())
    await fireEvent.click(screen.getAllByRole('button', { name: 'Раскрыть' })[0])
    await fireEvent.click(screen.getByRole('button', { name: 'Убрать «ring.jpg» из ленты' }))
    expect(removeLoose).toHaveBeenCalledExactlyOnceWith('b1', L2)
    expect(askPhoto()).toBeNull()
  })

  it('ветка «Фото отдельно» без ✕ (её никто не добавлял)', async () => {
    setLoose([A])
    mount()
    await fireEvent.click(button())
    const group = screen.getByRole('checkbox', { name: /«Фото отдельно»/ }).closest('li')!
    expect(group.querySelector('.x')).toBeNull()
    expect(group.querySelector('.xs')).toBeTruthy()
  })
})
