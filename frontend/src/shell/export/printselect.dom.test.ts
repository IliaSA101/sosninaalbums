// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { activePlan, buildOptions, DEFAULT_PREFS, hasFix, printHint, printQuality } from './exportopts'
import type { PrintKind, PrintPlan } from '../../base/core/types'

// Раздел «Печать» окна экспорта (081, 123): список вместо флажков. Go и хранилище подменены, сам список — в DOM.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
const { default: PrintSelect } = await import('./PrintSelect.svelte')

const files = import.meta.glob('/src/shell/export/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const src = (name: string): string => files[`/src/shell/export/${name}`] ?? ''

const plan = (o: Partial<PrintPlan>): PrintPlan => ({
  kind: 'fsfera',
  title: 'Для Фотосферы Pro',
  format: '21×30',
  off: '',
  pages: '',
  spreads: 10,
  spine: 0,
  min: 0,
  max: 0,
  folder: 'Иванов',
  coverName: '00.jpg',
  quality: 0,
  block: [],
  coverFix: false,
  fixAt: -1,
  warn: [],
  memo: [],
  done: '',
  site: '',
  siteButton: '',
  default: false,
  px: '',
  ...o,
})
const fsfera = plan({})
const fabrika = plan({ kind: 'fabrika', title: 'Для Фабрики Фотокниги', quality: 100, coverName: '1-00.jpg' })

const select = () => document.querySelector('select') as HTMLSelectElement
const options = () => Array.from(select().options).map((o) => `${o.value}|${o.textContent}`)
const mount = (plans: PrintPlan[], pref: '' | PrintKind, setPref = vi.fn(), hasCover = true) =>
  render(PrintSelect, { plans, plan: activePlan(plans, pref), hasCover, setPref })
afterEach(() => cleanup())

describe('раздел «Печать»', () => {
  it('список: «Обычная выгрузка» и типографии из планов Go, порядок Go; выбранная — в значении', () => {
    mount([fsfera, fabrika], 'fabrika')
    expect(options()).toEqual(['|Обычная выгрузка', 'fsfera|Для Фотосферы Pro', 'fabrika|Для Фабрики Фотокниги'])
    expect(select().value).toBe('fabrika')
  })
  it('выбор не запомнен или его нет среди планов — «Обычная выгрузка», подсказки нет', () => {
    mount([fsfera, fabrika], '')
    expect(select().value).toBe('')
    expect(document.querySelector('.hint')).toBeNull()
    cleanup()
    mount([fsfera], 'fabrika') // у страницы Фабрики нет: режим выключен
    expect(select().value).toBe('')
  })
  it('выбор пункта зовёт setPref с типом; «Обычная» — пустым; тот же пункт ещё раз — тот же выбор', async () => {
    const setPref = vi.fn()
    mount([fsfera, fabrika], '', setPref)
    await fireEvent.change(select(), { target: { value: 'fsfera' } })
    expect(setPref).toHaveBeenLastCalledWith('fsfera')
    await fireEvent.change(select(), { target: { value: '' } })
    expect(setPref).toHaveBeenLastCalledWith('')
    await fireEvent.change(select(), { target: { value: 'fsfera' } })
    await fireEvent.change(select(), { target: { value: 'fsfera' } })
    expect(setPref).toHaveBeenLastCalledWith('fsfera')
  })
  it('подсказка выбранного плана: папка, dpi, качество, обложка', () => {
    mount([fsfera, fabrika], 'fabrika')
    expect(document.querySelector('.hint')?.textContent).toBe(printHint(fabrika, true))
    expect(printHint(fabrika, true)).toContain('качество 100')
  })
  it('off: у плана чужого формата подсказка Go вместо «папка…», запрет под списком (120)', () => {
    const off = plan({
      kind: 'fabrika',
      format: '',
      off: 'страница 300 × 200 мм — не формат Фабрики Фотокниги',
      block: ['Страница 300 × 200 мм — не формат Фабрики Фотокниги'],
    })
    mount([off], 'fabrika')
    expect(document.querySelector('.hint')?.textContent).toBe('(страница 300 × 200 мм — не формат Фабрики Фотокниги)')
    expect(document.querySelector('p.err.blocked')?.textContent).toContain('не формат Фабрики Фотокниги')
    expect(activePlan([off], 'fabrika')).toBe(off)
  })
  it('памятка с обложкой типографии: px файла («Файлы») видны, «Папка» — нет', () => {
    const memo = [
      'Фотокнига 30×30, обложка «Фото»',
      'Страницы: Тонкие',
      'Разворотов: 20 (корешок 14 мм)',
      'Файлы: 7181×2398 px',
      'Папка: Иванов — 00.jpg и 01…20.jpg',
    ]
    mount([plan({ memo, px: '7181×2398 px' })], 'fsfera')
    const hint = Array.from(document.querySelectorAll('p.hint')).map((x) => x.textContent?.trim())
    expect(hint[0]).toBe('Фотокнига 30×30, обложка «Фото» · Страницы: Тонкие · Разворотов: 20 (корешок 14 мм) · Файлы: 7181×2398 px')
    expect(hint[0]).not.toContain('Папка')
  })
  it('нет планов — раздела нет', () => {
    mount([], '')
    expect(document.querySelector('select')).toBeNull()
    expect(src('ExportDialog.svelte')).not.toContain('filter((p) => p.format)') // чужой формат тоже показываем (120)
    expect(src('ExportDialog.svelte')).toContain('plans={plans ?? []}')
  })
  it('памятка, предупреждения и запреты выбранного плана — под списком', () => {
    const p = plan({
      memo: ['Фотокнига 30×30', 'Страницы: Тонкие', 'Папка: Иванов — 1-01…1-20.jpg'],
      warn: ['Предел близко'],
      block: ['Разворотов нет'],
    })
    mount([p], 'fsfera')
    const texts = Array.from(document.querySelectorAll('p')).map((x) => x.textContent?.trim())
    expect(texts).toEqual(['Фотокнига 30×30 · Страницы: Тонкие', 'Разворотов нет', 'Предел близко'])
  })
})

describe('«Исправить» и качество', () => {
  it('«Исправить» только у строки fixAt и только при обложке', () => {
    const p = plan({ block: ['Обложка не по шаблону', 'Разворотов нет'], coverFix: true, fixAt: 0 })
    expect(p.block.map((_, i) => hasFix(p, i))).toEqual([true, false])
    expect(src('PrintSelect.svelte')).toContain('hasFix(plan, i) && store.cover')
    expect(src('PrintSelect.svelte')).toContain('coverAct.applyPreset(store.cover!.preset)')
  })
  it('quality off: у плана с quality > 0 группа «Качество» серая и показывает 100', () => {
    expect(printQuality(fabrika, DEFAULT_PREFS.quality)).toBe(100)
    expect(printQuality(fsfera, 80)).toBe(80) // у Фотосферы выбор остаётся живым
    expect(buildOptions('all', 1, '', DEFAULT_PREFS, true, false, fabrika)).toMatchObject({ quality: 100, print: 'fabrika' })
    const q = src('ExportQuality.svelte')
    expect(q).toContain('off={qualityLocked}')
    expect(q).toContain('disabled={qualityLocked}')
    expect(src('ExportDialog.svelte')).toContain('qualityLocked={(plan?.quality ?? 0) > 0}')
  })
})
