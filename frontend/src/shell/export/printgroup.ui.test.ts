import { describe, expect, it } from 'vitest'
import { activePlan, buildOptions, DEFAULT_PREFS, hasFix, printHint, printQuality, togglePrint } from './exportopts'
import type { PrintPlan } from '../../base/core/types'

// Группа «Печать» окна экспорта (081). DOM в тестах нет, поэтому поведение — через чистые помощники,
// а связь разметки с ними — по исходникам (так же, как exportwindow.guard.test.ts).
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
  ...o,
})
const fsfera = plan({})
const fabrika = plan({ kind: 'fabrika', title: 'Для Фабрики Фотокниги', quality: 100, coverName: '1-00.jpg' })

describe('группа «Печать»', () => {
  it('two plans one checked: у 21×30 два флажка, отмечен один', () => {
    const plans = [fsfera, fabrika]
    const pref = togglePrint('', 'fabrika', true)
    expect(plans.map((p) => pref === p.kind)).toEqual([false, true])
    // щелчок по включённому выключает, и режим печати уходит
    const off = togglePrint(pref, 'fabrika', false)
    expect(plans.map((p) => off === p.kind)).toEqual([false, false])
    expect(activePlan(plans, off)).toBeNull()
    expect(src('PrintGroup.svelte')).toContain('{#each plans as p (p.kind)}')
    expect(src('PrintGroup.svelte')).toContain('checked={pref === p.kind}')
  })
  it('нет планов — группы «Печать» нет', () => {
    expect(src('PrintGroup.svelte')).toMatch(/\{#if plans\.length\}/)
    expect(src('ExportDialog.svelte')).not.toContain('filter((p) => p.format)') // чужой формат тоже показываем (120)
    expect(src('ExportDialog.svelte')).toContain('plans={plans ?? []}')
  })
  it('off: у плана чужого формата подсказка Go вместо «папка…», запрет под включённым флажком (120)', () => {
    const off = plan({
      kind: 'fabrika',
      format: '',
      off: 'страница 300 × 200 мм — не формат Фабрики Фотокниги',
      block: ['Страница 300 × 200 мм — не формат Фабрики Фотокниги'],
    })
    const g = src('PrintGroup.svelte')
    expect(g).toContain('p.off ? `(${p.off})` : printHint(p, hasCover)')
    expect(g).toContain('class="err blocked"')
    expect(activePlan([off], 'fabrika')).toBe(off)
  })
  it('block with fix: «Исправить» только у строки fixAt и только при обложке', () => {
    const p = plan({ block: ['Обложка не по шаблону', 'Разворотов нет'], coverFix: true, fixAt: 0 })
    expect(p.block.map((_, i) => hasFix(p, i))).toEqual([true, false])
    const g = src('PrintGroup.svelte')
    expect(g).toContain('hasFix(plan, i) && store.cover')
    expect(g).toContain('coverAct.applyPreset(store.cover!.preset)')
  })
  it('quality off: у плана с quality > 0 группа «Качество» серая и показывает 100', () => {
    expect(printQuality(fabrika, DEFAULT_PREFS.quality)).toBe(100)
    expect(printQuality(fsfera, 80)).toBe(80) // у Фотосферы выбор остаётся живым
    expect(printHint(fabrika, true)).toContain('качество 100')
    expect(buildOptions('all', 1, '', DEFAULT_PREFS, true, false, fabrika)).toMatchObject({ quality: 100, print: 'fabrika' })
    const q = src('ExportQuality.svelte')
    expect(q).toContain('off={qualityLocked}')
    expect(q).toContain('disabled={qualityLocked}')
    expect(src('ExportDialog.svelte')).toContain('qualityLocked={(plan?.quality ?? 0) > 0}')
  })
})
