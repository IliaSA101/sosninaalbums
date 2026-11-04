import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { FontFamily, FontImport } from '../../base/core/types'
import { REFUSED, resetFonts } from './fonts'
import { MISSING_TIP, hideFont, hideText, isOwnId, shownFont, uploadFonts, uploadFontsNoPick, uploadToast } from './fontupload'

const api = vi.hoisted(() => ({
  ImportFonts: vi.fn(),
  DropFonts: vi.fn(async (faces: string[]) => faces),
  HideFont: vi.fn(),
}))
vi.mock('./fontapi', () => api)

const own = (id: string, name: string, files: string[], over: Partial<FontFamily> = {}): FontFamily => ({
  id,
  name,
  css: `SA ${name}`,
  group: 'my',
  faces: files.map((file) => ({ file, bold: false, italic: false })),
  ...over,
})

const F1 = '1111111111111111.ttf'
const F2 = '2222222222222222.otf'
const added = (id: string, name: string, noCyr = false, files: string[] = []) => ({ id, name, faces: 1, noCyr, files })

// FontFace с управляемым результатом load(): файлы из bad не открываются (отказ окна).
function stubFace(bad: string[] = []) {
  vi.stubGlobal(
    'FontFace',
    class {
      constructor(
        public family: string,
        public src: string,
      ) {}
      async load() {
        if (bad.some((b) => this.src.includes(b))) throw new Error('OTS')
        return this
      }
    },
  )
}

function setup(list: FontFamily[]) {
  const notify = vi.fn()
  const pick = vi.fn()
  return { notify, pick, deps: { fonts: () => list, notify, pick } }
}

beforeEach(() => {
  resetFonts()
  vi.unstubAllGlobals()
  for (const f of Object.values(api)) f.mockClear()
  api.DropFonts.mockImplementation(async (faces: string[]) => faces)
})

describe('uploadFonts: загрузка (сц. 1)', () => {
  it('одна гарнитура — тост и она ставится тексту (onpick один раз)', async () => {
    stubFace()
    const imp: FontImport = { added: [added('my:a', 'Lato')], faces: [F1], skipped: [] }
    api.ImportFonts.mockResolvedValue(imp)
    const s = setup([own('my:a', 'Lato', [F1])])
    expect(await uploadFonts(s.deps)).toBe(true)
    expect(s.notify).toHaveBeenCalledWith('Добавлено: 1', 'info')
    expect(s.pick).toHaveBeenCalledTimes(1)
    expect(s.pick).toHaveBeenCalledWith('my:a')
    expect(api.DropFonts).not.toHaveBeenCalled()
  })

  it('две гарнитуры — только тост, шрифт не ставится', async () => {
    stubFace()
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato'), added('my:b', 'Ruslan')], faces: [F1, F2], skipped: [] })
    const s = setup([own('my:a', 'Lato', [F1]), own('my:b', 'Ruslan', [F2])])
    expect(await uploadFonts(s.deps)).toBe(false)
    expect(s.notify).toHaveBeenCalledWith('Добавлено: 2', 'info')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('одна принятая и один пропуск Go — тост с причиной, шрифт ставится', async () => {
    stubFace()
    api.ImportFonts.mockResolvedValue({
      added: [added('my:a', 'Lato')],
      faces: [F1],
      skipped: [{ name: 'Lato.woff2', why: 'веб-шрифт: нужен файл .ttf или .otf' }],
    })
    const s = setup([own('my:a', 'Lato', [F1])])
    await uploadFonts(s.deps)
    expect(s.notify).toHaveBeenCalledWith('Добавлено: 1, пропущено: 1 (Lato.woff2 — веб-шрифт: нужен файл .ttf или .otf)', 'info')
    expect(s.pick).toHaveBeenCalledWith('my:a')
  })

  it('отказ окна: единственная гарнитура откатана — пропуск в тосте, не ставится', async () => {
    stubFace([F1])
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato')], faces: [F1], skipped: [] })
    const s = setup([own('my:a', 'Lato', [F1])])
    expect(await uploadFonts(s.deps)).toBe(false)
    expect(api.DropFonts).toHaveBeenCalledWith([F1])
    expect(s.notify).toHaveBeenCalledWith('Добавлено: 0, пропущено: 1 (Lato — окно программы не смогло открыть этот шрифт)', 'error')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('из двух гарнитур окно отказало одной — тост «1, пропущено 1», оставшаяся не ставится (было две)', async () => {
    stubFace([F2])
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato'), added('my:b', 'Ruslan')], faces: [F1, F2], skipped: [] })
    const s = setup([own('my:a', 'Lato', [F1]), own('my:b', 'Ruslan', [F2])])
    await uploadFonts(s.deps)
    expect(s.notify.mock.calls[0][0]).toContain('Добавлено: 1, пропущено: 1')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('гарнитуры нет в списке (событие fonts не пришло) — не ставится', async () => {
    stubFace()
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato')], faces: [F1], skipped: [] })
    const s = setup([])
    expect(await uploadFonts(s.deps)).toBe(false)
    expect(s.pick).not.toHaveBeenCalled()
    expect(s.notify).toHaveBeenCalledTimes(1)
  })

  it('диалог закрыт без выбора — ни тоста, ни проверки, ни выбора', async () => {
    api.ImportFonts.mockResolvedValue({ added: [], faces: [], skipped: [] })
    const s = setup([])
    expect(await uploadFonts(s.deps)).toBe(false)
    expect(s.notify).not.toHaveBeenCalled()
    expect(s.pick).not.toHaveBeenCalled()
    expect(api.DropFonts).not.toHaveBeenCalled()
  })

  it('только пропуски (например, «уже есть») — красный тост, выбора нет', async () => {
    api.ImportFonts.mockResolvedValue({ added: [], faces: [], skipped: [{ name: 'Lato.ttf', why: 'уже есть в библиотеке' }] })
    const s = setup([])
    await uploadFonts(s.deps)
    expect(s.notify).toHaveBeenCalledWith('Добавлено: 0, пропущено: 1 (Lato.ttf — уже есть в библиотеке)', 'error')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('только чтение (ErrReadOnly из Go) — тост с текстом Go (заглавную ставит store), без проверки', async () => {
    api.ImportFonts.mockRejectedValue(new Error('библиотека открыта только для чтения'))
    const s = setup([])
    expect(await uploadFonts(s.deps)).toBe(false)
    expect(s.notify).toHaveBeenCalledWith('библиотека открыта только для чтения', 'error')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('гонка: событие fonts убрало откатанную гарнитуру из списка раньше ответа — тост называет её и причину', async () => {
    stubFace([F1])
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato', false, [F1])], faces: [F1], skipped: [] })
    let list = [own('my:a', 'Lato', [F1])]
    const s = setup([])
    // Список меняется, пока окно проверяет файл: DropFonts → событие fonts → в списке гарнитуры нет.
    api.DropFonts.mockImplementation(async (faces: string[]) => {
      list = []
      return faces
    })
    expect(await uploadFonts({ ...s.deps, fonts: () => list })).toBe(false)
    expect(s.notify).toHaveBeenCalledTimes(1)
    const [text, kind] = s.notify.mock.calls[0]
    expect(text).toContain('Lato')
    expect(text).toContain(REFUSED)
    expect(text).not.toContain('Шрифт')
    expect(kind).toBe('error')
    expect(s.pick).not.toHaveBeenCalled()
  })

  it('ошибка отката DropFonts — тост ошибки, шрифт не ставится', async () => {
    stubFace([F1])
    api.DropFonts.mockRejectedValue(new Error('нет метода'))
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato')], faces: [F1], skipped: [] })
    const s = setup([own('my:a', 'Lato', [F1])])
    await uploadFonts(s.deps)
    expect(s.notify).toHaveBeenCalledWith('нет метода', 'error')
    expect(s.pick).not.toHaveBeenCalled()
  })
})

describe('uploadToast: без русских букв (сц. 4)', () => {
  it('хвост про Montserrat только для гарнитур без кириллицы', () => {
    const t = uploadToast({ added: [added('my:a', 'Lato Script', true), added('my:b', 'Ruslan')], skipped: [] })
    expect(t).toEqual({ text: 'Добавлено: 2. Нет русских букв: Lato Script — они будут шрифтом Montserrat', error: false })
  })
  it('все с кириллицей — тост без хвоста; пусто — null', () => {
    expect(uploadToast({ added: [added('my:b', 'Ruslan')], skipped: [] })?.text).toBe('Добавлено: 1')
    expect(uploadToast({ added: [], skipped: [] })).toBeNull()
  })
})

describe('hideFont: × и «Убрать» (сц. 5)', () => {
  it('зовёт HideFont с id и возвращает true', async () => {
    api.HideFont.mockResolvedValue(undefined)
    const notify = vi.fn()
    expect(await hideFont('my:a', notify)).toBe(true)
    expect(api.HideFont).toHaveBeenCalledWith('my:a')
    expect(notify).not.toHaveBeenCalled()
  })
  it('ошибка Go (только чтение) — тост, false', async () => {
    api.HideFont.mockRejectedValue('Error: только чтение')
    const notify = vi.fn()
    expect(await hideFont('my:a', notify)).toBe(false)
    expect(notify).toHaveBeenCalledWith('только чтение', 'error')
  })
  it('текст вопроса — как у фонов', () => {
    expect(hideText('Lato')).toBe('«Lato» будет убран из библиотеки. Где он уже стоит, не изменится.')
  })
})

describe('shownFont: подпись кнопки (сц. 7)', () => {
  const list = [own('my:a', 'Lato', [F1]), own('montserrat', 'Montserrat', [], { group: 'sans' })]
  it('шрифт есть — его имя', () => {
    expect(shownFont('my:a', list, undefined)).toEqual({ name: 'Lato', missing: false })
  })
  it('своего нет, имя в albumFonts — «Lato (нет)»', () => {
    expect(shownFont('my:z', list, [{ id: 'my:z', name: 'Bodoni Moda' }])).toEqual({ name: 'Bodoni Moda (нет)', missing: true })
  })
  it('имя неизвестно (из шаблона) или albumFonts не пришёл — «Свой шрифт (нет)»', () => {
    expect(shownFont('my:z', list, [{ id: 'my:z', name: '' }])).toEqual({ name: 'Свой шрифт (нет)', missing: true })
    expect(shownFont('my:z', list, undefined)).toEqual({ name: 'Свой шрифт (нет)', missing: true })
  })
  it('пустая библиотека: чужой неизвестный id — как есть, без «(нет)»', () => {
    expect(shownFont('zzz', [], undefined)).toEqual({ name: 'zzz', missing: false })
  })
  it('подсказка и признак своего id', () => {
    expect(MISSING_TIP).toBe('Нет на этом компьютере — вместо него Montserrat')
    expect(isOwnId('my:0123456789abcdef')).toBe(true)
    expect(isOwnId('montserrat')).toBe(false)
  })
})

describe('uploadFontsNoPick: полоса «Нет шрифтов» (082-7)', () => {
  // Один и тот же ответ Go даёт один и тот же тост у списка «Шрифт» и у полосы; полоса шрифт не ставит.
  const cases: [string, FontImport][] = [
    ['без русских букв', { added: [added('my:a', 'Lato', true)], faces: [F1], skipped: [] }],
    ['с пропуском', { added: [added('my:a', 'Lato')], faces: [F1], skipped: [{ name: 'x.woff2', why: 'веб-шрифт' }] }],
    ['только пропуск', { added: [], faces: [], skipped: [{ name: 'x.woff2', why: 'веб-шрифт' }] }],
  ]
  it.each(cases)('одинаковый тост: %s', async (_n, imp) => {
    stubFace()
    api.ImportFonts.mockResolvedValue(imp)
    const list = [own('my:a', 'Lato', [F1])]
    const a = setup(list)
    await uploadFonts(a.deps)
    const b = setup(list)
    await uploadFontsNoPick({ fonts: b.deps.fonts, notify: b.notify })
    expect(b.notify.mock.calls).toEqual(a.notify.mock.calls)
    expect(b.notify).toHaveBeenCalledTimes(1)
  })

  it('ошибка Go — тот же errText, что у списка', async () => {
    api.ImportFonts.mockRejectedValue(new Error('альбом открыт только для чтения'))
    const a = setup([])
    await uploadFonts(a.deps)
    const b = setup([])
    expect(await uploadFontsNoPick({ fonts: b.deps.fonts, notify: b.notify })).toBe(false)
    expect(b.notify.mock.calls).toEqual(a.notify.mock.calls)
    expect(a.notify.mock.calls[0][1]).toBe('error')
  })

  it('окно закрыто без выбора — тишина', async () => {
    api.ImportFonts.mockResolvedValue({ added: [], faces: [], skipped: [] })
    const b = setup([])
    expect(await uploadFontsNoPick({ fonts: b.deps.fonts, notify: b.notify })).toBe(false)
    expect(b.notify).not.toHaveBeenCalled()
  })

  it('одна гарнитура — тост есть, текст не трогаем (pick не зовётся)', async () => {
    stubFace()
    api.ImportFonts.mockResolvedValue({ added: [added('my:a', 'Lato')], faces: [F1], skipped: [] })
    const b = setup([own('my:a', 'Lato', [F1])])
    expect(await uploadFontsNoPick({ fonts: b.deps.fonts, notify: b.notify })).toBe(true)
    expect(b.notify).toHaveBeenCalledWith('Добавлено: 1', 'info')
    expect(b.pick).not.toHaveBeenCalled()
  })
})
