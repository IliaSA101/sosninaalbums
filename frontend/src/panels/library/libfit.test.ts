import { describe, expect, it } from 'vitest'
import { BAR } from '../../editor/inspector/panels'
import { THUMB_DEF, THUMB_MAX, THUMB_MIN } from '../../base/prefs/view.svelte'
import { LIB_CHROME, LIB_DEF, PAD_Y, SPLITTER, libFloor, rowFloor } from './libwindow'

// Лента фото сжимается раньше холста (D15, 089): раскладку flex в jsdom не проверить — смотрим исходники.
const files = import.meta.glob(['/src/panels/library/Library.svelte', '/src/editor/canvas/Stage.svelte'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>
const lib = files['/src/panels/library/Library.svelte'] ?? ''
const stage = files['/src/editor/canvas/Stage.svelte'] ?? ''
const css = (s: string) => s.replace(/[\s\S]*<style>/, '')

describe('нижний предел ленты фото', () => {
  it('один полный ряд: полоса + поля сетки + плитка; растёт с размером плитки', () => {
    expect(rowFloor(THUMB_DEF)).toBe(LIB_CHROME + 2 * PAD_Y + THUMB_DEF)
    expect(rowFloor(THUMB_DEF)).toBeGreaterThanOrEqual(180)
    expect(rowFloor(THUMB_DEF)).toBeLessThanOrEqual(190)
    expect(rowFloor(THUMB_MAX) - rowFloor(THUMB_MIN)).toBe(THUMB_MAX - THUMB_MIN)
  })
  it('не выше заданной высоты: узкая лента остаётся узкой', () => {
    expect(libFloor(THUMB_DEF, 400)).toBe(rowFloor(THUMB_DEF))
    expect(libFloor(THUMB_DEF, 140)).toBe(140)
    expect(libFloor(THUMB_MAX, LIB_DEF)).toBe(LIB_DEF)
  })
  it('окно 1280×641: холст не меньше 240 при ленте в нижнем пределе', () => {
    // Высота колонки: окно − шапка 48; SpreadBar 40, лента альбома 88 (замеры D15).
    const col = 641 - 48
    const canvas = col - 40 - 88 - (rowFloor(THUMB_DEF) + SPLITTER) - BAR
    expect(canvas).toBeGreaterThanOrEqual(240)
  })
})

describe('исходники: порядок сжатия', () => {
  it('Stage: основа = BAR + LIB_DEF, растёт и сжимается', () => {
    expect(css(stage)).toContain(`flex: 1 1 ${BAR + LIB_DEF}px`)
  })
  it('Library: обёртка сжимается на порядки сильнее Stage, нижний предел и основа из разметки', () => {
    const shrink = Number(/flex-shrink:\s*(\d+)/.exec(css(lib))?.[1])
    expect(shrink).toBeGreaterThanOrEqual(1000)
    expect(lib).toContain('style:flex-basis="{collapsed ? 0 : eff + SPLITTER}px"')
    expect(lib).toContain('style:min-height="{collapsed ? 0 : floor + SPLITTER}px"')
    expect(lib).toContain('libFloor(modes.thumbSize, eff)')
  })
  it('сохранённая высота не трогается нехваткой места: eff от h и окна, не от DOM', () => {
    expect(lib).toMatch(/const eff = \$derived\(Math\.max\(MIN_H, Math\.min\(h, /)
    // запись в localStorage — только из сплиттера, значение h
    expect(lib.match(/localStorage\.setItem/g)).toHaveLength(1)
  })
  it('сплиттер стартует с фактической высоты ленты', () => {
    expect(lib).toContain('const h0 = box?.offsetHeight ?? eff')
  })
})
