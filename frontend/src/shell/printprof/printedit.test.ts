import { beforeEach, describe, expect, it, vi } from 'vitest'

// Состояние окна «Своя типография»: Go подменён, проверяем вызовы и итоги (тосты, открытие, вопросы). Диалоги файла — в Go, не вызываются.
const api = vi.hoisted(() => ({
  PrintProfiles: vi.fn(),
  PrintProfileEdit: vi.fn(),
  CheckPrintProfile: vi.fn(),
  SavePrintProfile: vi.fn(),
  ImportPrintProfile: vi.fn(),
  ConfirmImportPrintProfile: vi.fn(),
  SharePrintProfile: vi.fn(),
  DeletePrintProfile: vi.fn(),
  CoverPresets: vi.fn(),
  GetView: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.stubGlobal('window', globalThis)

const { printEdit } = await import('./printedit.svelte')
const { toEdit } = await import('./printedit.svelte')
const { printList } = await import('./printlist.svelte')
const { store } = await import('../../base/core/store.svelte')

const info = (id: string, name: string, source: string, version = 1) => ({ id, name, source, version, hasExport: true, formats: [] })
const INFOS = [
  info('fabrika', 'Фабрика Фотокниги', 'builtin'),
  info('fsfera', 'Фотосфера', 'builtin'),
  info('own-1', 'Альфа (своя)', 'own', 1),
]
const edit = (base: string, o: Record<string, unknown> = {}) => ({
  name: 'Новая (своя)',
  base,
  dpi: 300,
  layflat: true,
  safe: 10,
  bleed: 3,
  formats: [{ name: '30×20', pxW: 7181, pxH: 2398, coverW: 7866, coverH: 2835 }],
  spreads: 12,
  prefix: '1-',
  quality: 100,
  url: '',
  ...o,
})
const ok = {
  ok: true,
  errs: {},
  formats: [{ pageW: 304, pageH: 203, sideW: 310, sideH: 206, err: '', field: '', hit: false }],
  spine: 12,
  lines: [],
  names: '',
}
const bad = { ...ok, ok: false, errs: { name: 'Введите название' } }
const toast = () => store.toast as { text: string; kind?: string } | null
const flush = async () => {
  for (let i = 0; i < 10; i++) await Promise.resolve()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PrintProfiles.mockResolvedValue(INFOS)
  api.PrintProfileEdit.mockImplementation(async (b: string) => edit(b))
  api.CheckPrintProfile.mockResolvedValue(ok)
  api.CoverPresets.mockResolvedValue([])
  api.GetView.mockResolvedValue(store.view)
  printEdit.close()
  printEdit.busy = false
  printEdit.check = null
  printList.infos = []
  printList.saves = 0
  store.toast = null
})

describe('открытие', () => {
  it('show берёт заготовку на основе и открывает окно', async () => {
    await printEdit.show('fabrika')
    expect(api.PrintProfileEdit).toHaveBeenCalledWith('fabrika')
    expect(printEdit.open).toBe(true)
    expect(printEdit.e.base).toBe('fabrika')
    expect(printEdit.e.formats).toHaveLength(1)
    expect(printEdit.made).toBeNull()
  })

  it('«Общие» (без типографии) — пустая основа ""', async () => {
    await printEdit.show('')
    expect(api.PrintProfileEdit).toHaveBeenCalledWith('')
    expect(printEdit.open).toBe(true)
  })

  it('отказ Go — окно не открывается, ошибка тостом', async () => {
    api.PrintProfileEdit.mockRejectedValue('типографии "x" нет')
    await printEdit.show('x')
    expect(printEdit.open).toBe(false)
    expect(toast()).toMatchObject({ text: 'Типографии "x" нет', kind: 'error' })
  })

  it('смена основы — поля заново из Go; та же основа — без вызова', async () => {
    await printEdit.show('fabrika')
    api.PrintProfileEdit.mockClear()
    await printEdit.setBase('fabrika')
    expect(api.PrintProfileEdit).not.toHaveBeenCalled()
    await printEdit.setBase('fsfera')
    expect(api.PrintProfileEdit).toHaveBeenCalledWith('fsfera')
    expect(printEdit.e.base).toBe('fsfera')
  })

  describe('смена основы после правок (0.27-m-11)', () => {
    beforeEach(async () => {
      api.PrintProfileEdit.mockImplementation(async (b: string) => edit(b, b === 'fsfera' ? { dpi: 600, name: 'Из Фотосферы' } : {}))
      await printEdit.show('fabrika')
      api.PrintProfileEdit.mockClear()
    })

    it('поля не правили — основа меняется без вопроса', async () => {
      expect(printEdit.dirty).toBe(false)
      await printEdit.setBase('fsfera')
      expect(printEdit.ask).toBeNull()
      expect(printEdit.e).toMatchObject({ base: 'fsfera', dpi: 600 })
    })

    it('правили строку формата — вопрос, Go не зовётся, основа и поля прежние', async () => {
      printEdit.e.formats[0].pxW = 7000
      expect(printEdit.dirty).toBe(true)
      await printEdit.setBase('fsfera')
      expect(printEdit.ask).toMatchObject({ kind: 'base', id: 'fsfera', title: 'Фотосфера' })
      expect(printEdit.ask?.text).toBe('Сменить основу на «Фотосфера»? Поля окна заполнятся заново из этой типографии.')
      expect(printEdit.ask?.note).toBe('Введённые вами правки пропадут')
      expect(api.PrintProfileEdit).not.toHaveBeenCalled()
      expect(printEdit.e.base).toBe('fabrika')
      expect(printEdit.e.formats[0].pxW).toBe(7000)
    })

    it('правили название — тоже вопрос; «Без основы» называется словами списка', async () => {
      printEdit.e.name = 'Моя'
      await printEdit.setBase('')
      expect(printEdit.ask?.text).toContain('«Без основы (без обложки)»')
    })

    it('правка и возврат к прежнему значению — не правка', async () => {
      printEdit.e.dpi = 450
      printEdit.e.dpi = 300
      expect(printEdit.dirty).toBe(false)
      await printEdit.setBase('fsfera')
      expect(printEdit.ask).toBeNull()
    })

    it('отказ (dismiss) оставляет основу и правки; «Сменить» перезаполняет и снимает вопрос', async () => {
      printEdit.e.dpi = 450
      await printEdit.setBase('fsfera')
      printEdit.dismiss()
      expect(printEdit.ask).toBeNull()
      expect(printEdit.e).toMatchObject({ base: 'fabrika', dpi: 450 })
      await printEdit.setBase('fsfera')
      await printEdit.confirmBase()
      expect(api.PrintProfileEdit).toHaveBeenCalledWith('fsfera')
      expect(printEdit.ask).toBeNull()
      expect(printEdit.e).toMatchObject({ base: 'fsfera', dpi: 600, name: 'Из Фотосферы' })
      expect(printEdit.dirty).toBe(false)
    })

    it('«Сменить» без вопроса или чужой вопрос — ничего не делает', async () => {
      await printEdit.confirmBase()
      printEdit.ask = { kind: 'remove', id: 'x', title: 'x', text: '', note: '' }
      await printEdit.confirmBase()
      expect(api.PrintProfileEdit).not.toHaveBeenCalled()
      expect(printEdit.ask).not.toBeNull()
    })

    it('Go отказал после «Сменить» — вопрос закрыт, ошибка тостом, основа и правки прежние', async () => {
      printEdit.e.dpi = 450
      await printEdit.setBase('fsfera')
      api.PrintProfileEdit.mockRejectedValue('типографии "fsfera" нет')
      await printEdit.confirmBase()
      expect(printEdit.ask).toBeNull()
      expect(toast()).toMatchObject({ kind: 'error' })
      expect(printEdit.e).toMatchObject({ base: 'fabrika', dpi: 450 })
    })

    it('Esc окна сбрасывает вопрос', () => {
      printEdit.e.dpi = 450
      void printEdit.setBase('fsfera')
      printEdit.close()
      expect(printEdit.ask).toBeNull()
    })
  })

  it('Esc/«Отмена» (close): окно закрыто, вопрос и блок «добавлено» сброшены', async () => {
    await printEdit.show('fabrika')
    printEdit.ask = { kind: 'remove', id: 'x', title: 'x', text: '', note: '' }
    printEdit.close()
    expect(printEdit.open).toBe(false)
    expect(printEdit.ask).toBeNull()
  })
})

describe('живой расчёт', () => {
  it('в Go уходят числа: пустое поле (null) — 0', async () => {
    await printEdit.show('fabrika')
    const e = printEdit.e as unknown as Record<string, unknown>
    e.dpi = null
    ;(printEdit.e.formats[0] as unknown as Record<string, unknown>).coverW = null
    await printEdit.recheck()
    const sent = api.CheckPrintProfile.mock.calls.at(-1)![0]
    expect(sent.dpi).toBe(0)
    expect(sent.formats[0].coverW).toBe(0)
    expect(toEdit(printEdit.e).spreads).toBe(12)
  })

  it('ответ устаревшего запроса отбрасывается', async () => {
    await printEdit.show('fabrika')
    let slow!: (v: unknown) => void
    api.CheckPrintProfile.mockReturnValueOnce(new Promise((r) => (slow = r)))
    const a = printEdit.recheck()
    api.CheckPrintProfile.mockResolvedValueOnce(bad)
    await printEdit.recheck()
    slow(ok)
    await a
    expect(printEdit.check?.ok).toBe(false)
    expect(printEdit.check?.errs.name).toBe('Введите название')
  })

  it('строки: добавить, убрать, предел 32', async () => {
    await printEdit.show('fabrika')
    printEdit.addRow()
    expect(printEdit.e.formats).toHaveLength(2)
    printEdit.delRow(0)
    expect(printEdit.e.formats).toHaveLength(1)
    expect(printEdit.e.formats[0].name).toBe('')
    for (let i = 0; i < 40; i++) printEdit.addRow()
    expect(printEdit.e.formats).toHaveLength(32)
    expect(printEdit.canAdd).toBe(false)
  })

  it('качество 100 | 95', async () => {
    await printEdit.show('fabrika')
    printEdit.setQuality(95)
    expect(printEdit.e.quality).toBe(95)
  })
})

describe('«Сохранить»', () => {
  it('не сохраняет, пока расчёт не годен или его нет', async () => {
    await printEdit.show('fabrika')
    await printEdit.save()
    expect(api.SavePrintProfile).not.toHaveBeenCalled()
    printEdit.check = bad
    await printEdit.save()
    expect(api.SavePrintProfile).not.toHaveBeenCalled()
    expect(printEdit.canSave).toBe(false)
  })

  it('успех: SavePrintProfile(поля, apply=true), вид принят, тост, окно закрыто, список «Типография» встаёт на неё', async () => {
    await printEdit.show('fabrika')
    await printEdit.recheck()
    api.SavePrintProfile.mockResolvedValue({ id: 'own-9', view: { ...store.view, rev: store.view.rev + 1 } })
    await printEdit.save()
    expect(api.SavePrintProfile).toHaveBeenCalledTimes(1)
    const [sent, apply] = api.SavePrintProfile.mock.calls[0]
    expect(apply).toBe(true)
    expect(sent.name).toBe('Новая (своя)')
    expect(toast()?.text).toBe('Типография «Новая (своя)» сохранена и выбрана для альбома')
    expect(printEdit.open).toBe(false)
    expect(printList.saves).toBe(1)
  })

  it('отказ Go — тост красным, окно остаётся, поля целы', async () => {
    await printEdit.show('fabrika')
    await printEdit.recheck()
    api.SavePrintProfile.mockRejectedValue('такое название уже есть')
    await printEdit.save()
    expect(printEdit.open).toBe(true)
    expect(printEdit.busy).toBe(false)
    expect(toast()).toMatchObject({ text: 'Такое название уже есть', kind: 'error' })
    expect(printList.saves).toBe(0)
  })

  it('двойной щелчок: второй вызов ждёт первого', async () => {
    await printEdit.show('fabrika')
    await printEdit.recheck()
    let done!: (v: unknown) => void
    api.SavePrintProfile.mockReturnValue(new Promise((r) => (done = r)))
    const a = printEdit.save()
    await printEdit.save()
    expect(api.SavePrintProfile).toHaveBeenCalledTimes(1)
    done({ id: 'own-9', view: store.view })
    await a
  })
})

describe('«Добавить из файла…»', () => {
  const imp = (o: Record<string, unknown> = {}) => ({
    canceled: false,
    info: info('own-7', 'Бета-принт', 'own', 2),
    ask: '',
    question: '',
    token: '',
    copied: false,
    lines: ['Форматы: 20×20 — 4724 × 2398 px', 'Файлы: B-00.jpg — обложка, B-01.jpg…B-12.jpg', 'Сайт: beta-print.example'],
    ...o,
  })

  it('отмена диалога — ничего', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp({ canceled: true }))
    api.PrintProfileEdit.mockClear()
    await printEdit.importFile()
    expect(api.PrintProfileEdit).not.toHaveBeenCalled()
    expect(printEdit.made).toBeNull()
    expect(toast()).toBeNull()
  })

  it('добавлена: окно на её основе и блок «добавлена из файла» со сводкой', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp())
    await printEdit.importFile()
    expect(api.PrintProfileEdit).toHaveBeenLastCalledWith('own-7')
    expect(printEdit.made?.title).toBe('Типография «Бета-принт» добавлена из файла')
    expect(printEdit.made?.lines).toHaveLength(3)
  })

  it('добавлена копией — лишняя строка про номер', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp({ copied: true }))
    await printEdit.importFile()
    expect(printEdit.made?.lines.at(-1)).toBe('Номер из файла уже занят — добавлена копией')
  })

  it('такая уже стоит — тост, окно как было', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp({ ask: 'same' }))
    api.PrintProfileEdit.mockClear()
    await printEdit.importFile()
    expect(toast()?.text).toBe('Типография «Бета-принт» уже есть: ничего не изменилось')
    expect(api.PrintProfileEdit).not.toHaveBeenCalled()
  })

  it('вопрос о замене → «Заменить» → тост с версией и окно на ней', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(
      imp({ ask: 'replace', token: 'tok', question: 'Заменить типографию «Альфа» (версия 1) типографией из файла (версия 2)?' }),
    )
    await printEdit.importFile()
    expect(printEdit.ask).toMatchObject({ kind: 'replace', token: 'tok' })
    expect(printEdit.ask?.note).toContain('Форматы: 20×20')
    api.ConfirmImportPrintProfile.mockResolvedValue(imp())
    await printEdit.confirmReplace()
    expect(api.ConfirmImportPrintProfile).toHaveBeenCalledWith('tok')
    expect(printEdit.ask).toBeNull()
    expect(toast()?.text).toBe('Типография «Бета-принт» заменена: версия 2')
    expect(api.PrintProfileEdit).toHaveBeenLastCalledWith('own-7')
  })

  it('«Отмена» вопроса — замены нет', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp({ ask: 'replace', token: 'tok', question: '?' }))
    await printEdit.importFile()
    printEdit.dismiss()
    await printEdit.confirmReplace()
    expect(api.ConfirmImportPrintProfile).not.toHaveBeenCalled()
  })

  it('битый файл — ошибка Go как пришла', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockRejectedValue('Файл типографии не читается: в файле больше 64 КБ')
    await printEdit.importFile()
    expect(toast()).toMatchObject({ text: 'Файл типографии не читается: в файле больше 64 КБ', kind: 'error' })
    expect(printEdit.busy).toBe(false)
  })

  it('вопрос устарел — ошибка тостом', async () => {
    await printEdit.show('fabrika')
    api.ImportPrintProfile.mockResolvedValue(imp({ ask: 'replace', token: 'tok', question: '?' }))
    await printEdit.importFile()
    api.ConfirmImportPrintProfile.mockRejectedValue('вопрос устарел — добавьте файл ещё раз')
    await printEdit.confirmReplace()
    expect(toast()?.kind).toBe('error')
  })
})

describe('«Поделиться файлом…» и «Удалить»: только у своей', () => {
  it('встроенная: оба молчат', async () => {
    await printEdit.show('fabrika')
    expect(printEdit.ownBase).toBe(false)
    await printEdit.share()
    printEdit.askRemove()
    expect(api.SharePrintProfile).not.toHaveBeenCalled()
    expect(printEdit.ask).toBeNull()
  })

  it('без основы: оба молчат', async () => {
    await printEdit.show('')
    expect(printEdit.ownBase).toBe(false)
    printEdit.askRemove()
    expect(printEdit.ask).toBeNull()
  })

  it('своя: «Поделиться» зовёт Go с её id; тост с именем файла; отмена диалога — молча', async () => {
    await printEdit.show('own-1')
    expect(printEdit.ownBase).toBe(true)
    api.SharePrintProfile.mockResolvedValue({ canceled: false, name: 'Альфа.saprint' })
    await printEdit.share()
    expect(api.SharePrintProfile).toHaveBeenCalledWith('own-1')
    expect(toast()?.text).toBe('Типография записана в файл «Альфа.saprint»')
    store.toast = null
    api.SharePrintProfile.mockResolvedValue({ canceled: true, name: '' })
    await printEdit.share()
    expect(toast()).toBeNull()
  })

  it('своя: «Удалить» — сначала вопрос с id; «Отмена» ничего не удаляет', async () => {
    await printEdit.show('own-1')
    printEdit.askRemove()
    expect(printEdit.ask).toMatchObject({ kind: 'remove', id: 'own-1' })
    expect(printEdit.ask?.text).toContain('«Типография не найдена: own-1»')
    expect(printEdit.ask?.note).toBe('Вернуть её можно только файлом .saprint')
    printEdit.dismiss()
    await printEdit.confirmRemove()
    expect(api.DeletePrintProfile).not.toHaveBeenCalled()
  })

  it('своя: «Удалить» в вопросе → Go, тост, окно встаёт на первую встроенную', async () => {
    await printEdit.show('own-1')
    printEdit.askRemove()
    api.DeletePrintProfile.mockResolvedValue(store.view)
    api.PrintProfiles.mockResolvedValue(INFOS.filter((i) => i.source === 'builtin'))
    await printEdit.confirmRemove()
    await flush()
    expect(api.DeletePrintProfile).toHaveBeenCalledWith('own-1')
    expect(toast()?.text).toBe('Типография «Альфа (своя)» удалена')
    expect(api.PrintProfileEdit).toHaveBeenLastCalledWith('fabrika')
    expect(printEdit.open).toBe(true)
    expect(printEdit.ask).toBeNull()
  })

  it('удалить не удалось — тост красным, окно на месте', async () => {
    await printEdit.show('own-1')
    printEdit.askRemove()
    api.DeletePrintProfile.mockRejectedValue('это не своя типография — её нельзя удалить или отправить файлом')
    await printEdit.confirmRemove()
    expect(toast()?.kind).toBe('error')
    expect(printEdit.open).toBe(true)
  })
})
