// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import type { PrintPlan } from '../../base/core/types'

// «(своя)» в списке «Печать» (123 Б3): Go пишет «Для «Альфа»», вид пометки решает интерфейс по реестру типографий.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
const { default: PrintSelect } = await import('./PrintSelect.svelte')
const { printList } = await import('../printprof/printlist.svelte')

const plan = (kind: string, title: string): PrintPlan =>
  ({
    kind,
    title,
    format: '30×20',
    off: '',
    pages: '',
    spreads: 10,
    spine: 0,
    min: 0,
    max: 0,
    folder: 'a',
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
  }) as PrintPlan

afterEach(() => {
  cleanup()
  printList.infos = []
})

describe('«Печать»: своя типография', () => {
  it('у своей — «(своя)» после названия; у встроенной — без', () => {
    printList.infos = [
      { id: 'fabrika', name: 'Фабрика Фотокниги', source: 'builtin', version: 1, hasExport: true, formats: [] },
      { id: 'own-1', name: 'Альфа', source: 'own', version: 1, hasExport: true, formats: [] },
    ]
    const plans = [plan('fabrika', 'Для Фабрики Фотокниги'), plan('own-1', 'Для «Альфа»')]
    render(PrintSelect, { plans, plan: plans[1], hasCover: true, setPref: vi.fn() })
    const texts = Array.from(document.querySelectorAll('option')).map((o) => o.textContent)
    expect(texts).toEqual(['Обычная выгрузка', 'Для Фабрики Фотокниги', 'Для «Альфа» (своя)'])
  })
})
