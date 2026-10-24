// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Окно «Своя типография»: кнопки у своей и встроенной, «Сохранить» от расчёта Go, ошибки под полями, вопросы поверх окна.
// Go подменён; нативные диалоги файла не вызываются (ImportPrintProfile отдаёт готовый итог).
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
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: Dialog } = await import('./PrintProfileDialog.svelte')
const { printEdit } = await import('./printedit.svelte')
const { printList } = await import('./printlist.svelte')
const { store } = await import('../../base/core/store.svelte')

const info = (id: string, name: string, source: string) => ({ id, name, source, version: 1, hasExport: true, formats: [] })
const INFOS = [info('fabrika', 'Фабрика Фотокниги', 'builtin'), info('own-1', 'Типография Альфа', 'own')]
const edit = (base: string) => ({
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
})
const ok = {
  ok: true,
  errs: {},
  formats: [{ pageW: 303.95, pageH: 202.95, sideW: 310, sideH: 206, err: '', field: '', hit: true }],
  spine: 12,
  lines: ['Сейчас альбом даёт: разворот 7181 × 2398 px при 300 dpi — это формат «30×20»'],
  names: '1-00.jpg — обложка, 1-01.jpg…1-12.jpg',
}
const flush = async () => {
  for (let i = 0; i < 12; i++) await Promise.resolve()
  flushSync()
}
// Подписи полей строки формата, помеченных красным (class bad).
const badLabels = () => [...document.querySelectorAll('.gr input.bad')].map((e) => e.getAttribute('aria-label'))
const btn = (name: string) => screen.getByRole('button', { name }) as HTMLButtonElement

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.PrintProfiles.mockResolvedValue(INFOS)
  api.PrintProfileEdit.mockImplementation(async (b: string) => edit(b))
  api.CheckPrintProfile.mockResolvedValue(ok)
  api.CoverPresets.mockResolvedValue([])
  printEdit.close()
  printEdit.busy = false
  printEdit.check = null
  store.toast = null
})
afterEach(() => cleanup())

async function open(base: string) {
  render(Dialog)
  await printEdit.show(base)
  await flush()
}

describe('PrintProfileDialog', () => {
  it('закрыто — пусто; открыто — окно «Своя типография» с полями и расчётом Go', async () => {
    render(Dialog)
    expect(screen.queryByRole('dialog')).toBeNull()
    await printEdit.show('fabrika')
    await flush()
    expect(screen.getByRole('dialog', { name: 'Своя типография' })).toBeTruthy()
    expect((screen.getByLabelText('Название') as HTMLInputElement).value).toBe('Новая (своя)')
    expect(document.body.textContent).toContain('= 303,95 × 202,95 мм')
    expect(document.body.textContent).toContain('= 310 × 206 мм')
    expect(document.body.textContent).toContain(', корешок 12 мм')
    expect(document.body.textContent).toContain('Файлы: 1-00.jpg — обложка')
  })

  it('порядок кнопок: слева «Добавить из файла…», «Поделиться файлом…», «Удалить»; справа «Отмена», «Сохранить»', async () => {
    await open('fabrika')
    const names = screen.getAllByRole('button', { hidden: false }).map((b) => b.textContent?.trim())
    const tail = names.slice(names.indexOf('Добавить из файла…'))
    expect(tail).toEqual(['Добавить из файла…', 'Поделиться файлом…', 'Удалить', 'Отмена', 'Сохранить'])
  })

  it('основа встроенная — «Поделиться» и «Удалить» серые с подсказкой; основа своя — рабочие', async () => {
    await open('fabrika')
    expect(btn('Поделиться файлом…').disabled).toBe(true)
    expect(btn('Удалить').disabled).toBe(true)
    expect(btn('Удалить').title).toBe('Встроенную типографию не отправить и не удалить: выберите в «Основе» свою')
    cleanup()
    printEdit.close()
    await open('own-1')
    expect(btn('Поделиться файлом…').disabled).toBe(false)
    expect(btn('Удалить').disabled).toBe(false)
    expect(btn('Поделиться файлом…').title).toBe('Записать «Типография Альфа.saprint»')
  })

  it('смена «Основы» на свою включает кнопки', async () => {
    await open('fabrika')
    const base = screen.getByLabelText('Основа') as HTMLSelectElement
    base.value = 'own-1'
    await fireEvent.change(base)
    await flush()
    expect(api.PrintProfileEdit).toHaveBeenLastCalledWith('own-1')
    expect(btn('Удалить').disabled).toBe(false)
  })

  it('«Сохранить» серая, пока Go не одобрил; ошибка названия — под полем «Название»', async () => {
    api.CheckPrintProfile.mockResolvedValue({ ...ok, ok: false, errs: { name: 'Такое название уже есть' } })
    await open('fabrika')
    expect(btn('Сохранить').disabled).toBe(true)
    expect(btn('Сохранить').title).toBe('Исправьте отмеченные поля')
    const name = screen.getByLabelText('Название')
    expect(name.className).toContain('bad')
    expect(name.parentElement!.querySelector('.errt')!.textContent).toBe('Такое название уже есть')
  })

  it('ошибки без своего места — общей строкой формы, как пришли из Go', async () => {
    api.CheckPrintProfile.mockResolvedValue({ ...ok, ok: false, errs: { form: 'Адрес на другом сайте' } })
    await open('fabrika')
    expect(document.querySelector('.errt.form')!.textContent).toBe('Адрес на другом сайте')
  })

  it('ошибка строки формата — под строкой; красным — поле по field', async () => {
    api.CheckPrintProfile.mockResolvedValue({
      ...ok,
      ok: false,
      errs: { formats: 'Исправьте строки форматов' },
      formats: [{ pageW: 0, pageH: 0, sideW: 0, sideH: 0, err: 'Подпись повторена', field: 'name', hit: false }],
    })
    await open('fabrika')
    expect(document.body.textContent).toContain('Подпись повторена')
    expect(document.body.textContent).toContain('Исправьте строки форматов')
    expect(document.body.textContent).toContain('= —')
    expect(badLabels()).toEqual(['Подпись формата'])
  })

  it.each([
    ['px', ['Ширина разворота, px', 'Высота разворота, px']],
    ['cover', ['Ширина обложки, px', 'Высота обложки, px']],
  ] as const)('field «%s» — красные только свои числа, подпись не тронута', async (field, labels) => {
    api.CheckPrintProfile.mockResolvedValue({
      ...ok,
      ok: false,
      errs: { formats: 'Исправьте строки форматов' },
      formats: [{ pageW: 0, pageH: 0, sideW: 0, sideH: 0, err: 'Подпись повторена', field, hit: false }],
    })
    await open('fabrika')
    expect(badLabels()).toEqual(labels)
  })

  it('«Сейчас альбом даёт…» зелёная по hit, не по тексту', async () => {
    await open('fabrika')
    const line = () => document.querySelector('.check .t')!
    expect(line().classList.contains('f')).toBe(true)
    api.CheckPrintProfile.mockResolvedValue({ ...ok, formats: [{ ...ok.formats[0], hit: false }] })
    await fireEvent.input(screen.getByLabelText('Плотность, dpi'), { target: { value: '600' } })
    await flush()
    // текст всё ещё кончается « — это формат «30×20»», но Go признака не дал
    expect(line().textContent).toContain('— это формат')
    expect(line().classList.contains('f')).toBe(false)
  })

  it('правка поля — новый расчёт у Go с этим значением', async () => {
    await open('fabrika')
    api.CheckPrintProfile.mockClear()
    await fireEvent.input(screen.getByLabelText('Плотность, dpi'), { target: { value: '600' } })
    await flush()
    expect(api.CheckPrintProfile.mock.calls.at(-1)![0].dpi).toBe(600)
  })

  it('качество 100 | 95: нажатая — 95; уходит в Go', async () => {
    await open('fabrika')
    expect(screen.getByRole('button', { name: '100' }).getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(screen.getByRole('button', { name: '95' }))
    await flush()
    expect(screen.getByRole('button', { name: '95' }).getAttribute('aria-pressed')).toBe('true')
    expect(api.CheckPrintProfile.mock.calls.at(-1)![0].quality).toBe(95)
  })

  it('«+ Добавить формат» и ✕ у строки', async () => {
    await open('fabrika')
    await fireEvent.click(btn('+ Добавить формат'))
    await flush()
    expect(screen.getAllByLabelText('Подпись формата')).toHaveLength(2)
    await fireEvent.click(screen.getAllByRole('button', { name: 'Убрать формат' })[0])
    await flush()
    expect(screen.getAllByLabelText('Подпись формата')).toHaveLength(1)
  })

  it('«Сохранить» при годном расчёте зовёт Go и закрывает окно', async () => {
    await open('fabrika')
    api.SavePrintProfile.mockResolvedValue({ id: 'own-9', view: { ...store.view, rev: store.view.rev + 1 } })
    expect(btn('Сохранить').disabled).toBe(false)
    await fireEvent.click(btn('Сохранить'))
    await flush()
    expect(api.SavePrintProfile).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog', { name: 'Своя типография' })).toBeNull()
  })

  it('«Отмена» закрывает без записи', async () => {
    await open('fabrika')
    await fireEvent.click(btn('Отмена'))
    expect(api.SavePrintProfile).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('«Удалить»: вопрос поверх окна, «Отмена» — назад, «Удалить» — Go', async () => {
    await open('own-1')
    await fireEvent.click(btn('Удалить'))
    await flush()
    const ask = screen.getByRole('dialog', { name: 'Удалить типографию?' })
    expect(ask.textContent).toContain('«Типография не найдена: own-1»')
    expect(ask.textContent).toContain('Вернуть её можно только файлом .saprint')
    await fireEvent.click(Array.from(ask.querySelectorAll('button')).find((b) => b.textContent === 'Отмена')!)
    await flush()
    expect(screen.queryByRole('dialog', { name: 'Удалить типографию?' })).toBeNull()
    expect(api.DeletePrintProfile).not.toHaveBeenCalled()
    await fireEvent.click(btn('Удалить'))
    await flush()
    api.DeletePrintProfile.mockResolvedValue(store.view)
    api.PrintProfiles.mockResolvedValue([INFOS[0]])
    const again = screen.getByRole('dialog', { name: 'Удалить типографию?' })
    await fireEvent.click(Array.from(again.querySelectorAll('button')).find((b) => b.textContent === 'Удалить')!)
    await flush()
    expect(api.DeletePrintProfile).toHaveBeenCalledWith('own-1')
  })

  it('добавлено из файла — блок со сводкой над полями', async () => {
    await open('fabrika')
    api.ImportPrintProfile.mockResolvedValue({
      canceled: false,
      info: info('own-7', 'Бета-принт', 'own'),
      ask: '',
      question: '',
      token: '',
      copied: false,
      lines: ['Форматы: 20×20 — 4724 × 2398 px', 'Сайт: beta-print.example'],
    })
    await fireEvent.click(btn('Добавить из файла…'))
    await flush()
    const made = document.querySelector('.made')!
    expect(made.textContent).toContain('Типография «Бета-принт» добавлена из файла')
    expect(made.textContent).toContain('Сайт: beta-print.example')
  })

  it('вопрос о замене: «Отмена» слева, «Заменить» справа', async () => {
    await open('fabrika')
    api.ImportPrintProfile.mockResolvedValue({
      canceled: false,
      info: info('own-1', 'Типография Альфа', 'own'),
      ask: 'replace',
      question: 'Заменить типографию «Типография Альфа» (версия 1) типографией из файла (версия 2)?',
      token: 't',
      copied: false,
      lines: ['Форматы: 20×20 — 4724 × 2398 px'],
    })
    await fireEvent.click(btn('Добавить из файла…'))
    await flush()
    const ask = screen.getByRole('dialog', { name: 'Заменить типографию?' })
    expect(ask.textContent).toContain('(версия 2)?')
    expect(Array.from(ask.querySelectorAll('button')).map((b) => b.textContent?.trim() || b.getAttribute('aria-label'))).toEqual([
      'Закрыть',
      'Отмена',
      'Заменить',
    ])
  })

  it('список основы: встроенные и свои, «Без основы» последним', async () => {
    await open('fabrika')
    const base = screen.getByLabelText('Основа') as HTMLSelectElement
    expect(Array.from(base.querySelectorAll('optgroup')).map((g) => g.label)).toEqual(['Встроенные', 'Свои'])
    expect(base.options[base.options.length - 1].textContent).toBe('Без основы (без обложки)')
    expect(printList.infos).toHaveLength(2)
  })
})
