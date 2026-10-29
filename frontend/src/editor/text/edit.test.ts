import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и store подменены: проверяем только порядок «набранное → ожидание цепочки».
const calls: string[] = []
vi.mock('../../base/core/store.svelte', () => ({
  store: {
    run: vi.fn(async (p: Promise<unknown>) => (await p, true)),
    spread: undefined,
    view: { spreads: [] },
    selectedCell: null,
    fonts: [],
  },
  Api: {
    SetText: vi.fn(async (id: string, text: string) => {
      calls.push(`${id}:${text}`)
    }),
    SetCaption: vi.fn(),
    SetCaptionStyle: vi.fn(),
    SetSheetCaps: vi.fn(),
    SetTextStyle: vi.fn(),
  },
}))

import { edit, SEND_DELAY } from './edit.svelte'
import { colorLive } from '../../base/color/colorlive.svelte'
import { Api, store } from '../../base/core/store.svelte'
import type { CaptionView, SheetCaps, SpreadView, TextStyle } from '../../base/core/types'

beforeEach(() => {
  vi.stubGlobal('window', globalThis)
  calls.length = 0
  edit.sel = { kind: 'text', spread: 's1', id: 't1' }
  edit.target = null
  edit.closing = false
})

describe('edit.idle', () => {
  it('досылает набранное сразу, не дожидаясь паузы', async () => {
    edit.startEdit()
    edit.input('привет')
    expect(calls).toEqual([])
    await edit.idle()
    expect(calls).toEqual(['t1:привет'])
    // Таймер погашен: через SEND_DELAY вторая отправка не придёт.
    await new Promise((r) => setTimeout(r, SEND_DELAY + 50))
    expect(calls).toEqual(['t1:привет'])
  })

  it('без набранного ничего не шлёт', async () => {
    edit.startEdit()
    await edit.idle()
    expect(calls).toEqual([])
  })
})

// Подписи с подстановками (030): общий стиль листа, отступ, вставка поля.
const style: TextStyle = { font: 'montserrat', size: 10, color: '#555555', align: 'center' }
const caption = (o: Partial<CaptionView> = {}) =>
  ({ photo: 'p1', id: '', text: 'Иванов', src: '{Фамилия}', style, over: false, pad: 4, ...o }) as CaptionView
const caps: SheetCaps = { style, pad: 4, over: false, wrap: false, text: '{Фамилия}' }
const spread = (o: Partial<SpreadView> = {}) => ({ id: 's1', captions: [caption()], texts: [], ...o }) as unknown as SpreadView

function selectCaption(sp: SpreadView) {
  ;(store as unknown as { view: { spreads: SpreadView[] } }).view.spreads = [sp]
  edit.sel = { kind: 'caption', spread: sp.id, id: 'p1' }
  vi.mocked(Api.SetSheetCaps).mockClear()
  vi.mocked(Api.SetCaptionStyle).mockClear()
}

describe('подпись без стиля листа', () => {
  it('стиль уходит в свою подпись и несёт её отступ', () => {
    selectCaption(spread())
    edit.setStyle({ size: 14 })
    expect(Api.SetCaptionStyle).toHaveBeenCalledWith('p1', { ...style, size: 14, bold: false, italic: false }, false, 4)
    expect(Api.SetSheetCaps).not.toHaveBeenCalled()
  })

  it('отступ зажимается 0…20', () => {
    selectCaption(spread())
    edit.setPad(99)
    expect(Api.SetCaptionStyle).toHaveBeenLastCalledWith('p1', style, false, 20)
    edit.setPad(NaN)
    expect(Api.SetCaptionStyle).toHaveBeenLastCalledWith('p1', style, false, 4)
  })
})

describe('подпись со стилем листа', () => {
  it('стиль, «на фото» и отступ уходят в лист, а не в подпись', () => {
    selectCaption(spread({ caps }))
    edit.setStyle({ size: 14 })
    expect(Api.SetSheetCaps).toHaveBeenLastCalledWith('s1', true, {
      ...caps,
      style: { ...style, size: 14, bold: false, italic: false },
      over: false,
      pad: 4,
    })
    edit.setOver(true)
    expect(Api.SetSheetCaps).toHaveBeenLastCalledWith('s1', true, { ...caps, over: true })
    edit.setPad(2.5)
    expect(Api.SetSheetCaps).toHaveBeenLastCalledWith('s1', true, { ...caps, pad: 2.5 })
    expect(Api.SetCaptionStyle).not.toHaveBeenCalled()
  })

  it('шаблон и «ужимать» меняют только свои поля листа', () => {
    selectCaption(spread({ caps }))
    edit.setSheetPart('s1', { text: '{Имя}' })
    expect(Api.SetSheetCaps).toHaveBeenLastCalledWith('s1', true, { ...caps, text: '{Имя}' })
    edit.setSheetPart('s1', { wrap: true })
    expect(Api.SetSheetCaps).toHaveBeenLastCalledWith('s1', true, { ...caps, wrap: true })
  })
})

describe('«Один стиль на лист»', () => {
  it('включается со стилем выделенной подписи, ужимание включено', () => {
    selectCaption(spread({ captions: [caption({ photo: 'p0', pad: 1 }), caption({ over: true, pad: 6 })] }))
    edit.setSheet('s1', true)
    expect(Api.SetSheetCaps).toHaveBeenCalledWith('s1', true, { style, pad: 6, over: true, wrap: false, text: '' })
  })

  it('выключается; на листе без подписей включить нельзя', () => {
    selectCaption(spread({ caps }))
    edit.setSheet('s1', false)
    expect(Api.SetSheetCaps).toHaveBeenCalledWith('s1', false, caps)
    vi.mocked(Api.SetSheetCaps).mockClear()
    selectCaption(spread({ captions: [] }))
    edit.setSheet('s1', true)
    expect(Api.SetSheetCaps).not.toHaveBeenCalled()
  })
})

describe('insertField вне ввода', () => {
  it('дописывает поле к шаблону (src), а не к подставленному тексту', async () => {
    selectCaption(spread())
    edit.target = null
    await edit.insertField('Имя')
    expect(Api.SetCaption).toHaveBeenLastCalledWith('p1', '{Фамилия}{Имя}', expect.any(String))
  })
})

describe('previewColor: живой показ цвета (109)', () => {
  it('блок текста — только он, без вызовов Go', () => {
    edit.sel = { kind: 'text', spread: 's1', id: 't1' }
    edit.previewColor('#aabbcc')
    expect(colorLive.color('text:t1', '#000000')).toBe('#aabbcc')
    expect(colorLive.color('text:t2', '#000000')).toBe('#000000')
    expect(Api.SetTextStyle).not.toHaveBeenCalled()
    edit.previewColor(null)
    expect(colorLive.color('text:t1', '#000000')).toBe('#000000')
  })

  it('подпись без общего стиля — только она', () => {
    selectCaption(spread({ captions: [caption(), caption({ photo: 'p2' })] }))
    edit.previewColor('#aabbcc')
    expect(colorLive.color('cap:p1', '')).toBe('#aabbcc')
    expect(colorLive.color('cap:p2', '')).toBe('')
    edit.previewColor(null)
  })

  it('previewColor при общем стиле листа — все подписи листа', () => {
    selectCaption(spread({ caps, captions: [caption(), caption({ photo: 'p2' })] }))
    edit.previewColor('#aabbcc')
    expect(colorLive.color('cap:p1', '')).toBe('#aabbcc')
    expect(colorLive.color('cap:p2', '')).toBe('#aabbcc')
    expect(Api.SetSheetCaps).not.toHaveBeenCalled()
    edit.previewColor(null)
    expect(colorLive.color('cap:p2', '')).toBe('')
  })

  it('ничего не выделено — показа нет; setStyle отдаёт промис правки для commit', async () => {
    edit.sel = null
    edit.previewColor('#aabbcc')
    expect(colorLive.cur).toBeNull()
    selectCaption(spread())
    expect(edit.setStyle({ color: '#112233' })).toBeInstanceOf(Promise)
  })
})
