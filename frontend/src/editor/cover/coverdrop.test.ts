import { describe, expect, it } from 'vitest'
import src from './CoverDrop.svelte?raw'

// DOM в тестах нет (jsdom не подключён), поэтому разметку зон броска проверяем по исходнику компонента;
// ширину попадания и вид — глазами в QA (docs/qa/cover.md 061 п. 2).
const code = src.replace(/<!--[\s\S]*?-->/g, '').replace(/^\s*\/\/.*$/gm, '')
const markup = code.slice(code.indexOf('{#each'), code.indexOf('<style>'))
const style = code.slice(code.indexOf('<style>'))

describe('зона броска корешка (087-1)', () => {
  it('у зоны корешка полоса попадания и табличка', () => {
    // зоны идут через dropOrder (корешок последним), а не в порядке Go
    expect(markup).toMatch(/\{#each dropOrder\(cover\.drop\) as z/)
    // .grab — только у корешка, у задней/лицевой вместо неё подсказка
    expect(markup.match(/class="grab"/g)).toHaveLength(1)
    expect(markup).toMatch(/\{#if z\.kind === 'spine'\}<i class="grab"[^>]*><\/i>\{:else\}<span class="hint"/)
    // ширина полосы — из SPINE_HIT_PX, не уже зоны
    expect(markup).toMatch(/--spine-hit:\{SPINE_HIT_PX\}px/)
    expect(style).toMatch(/\.grab\s*\{[^}]*width:\s*max\(100%,\s*var\(--spine-hit\)\)/)
    expect(style).toMatch(/\.grab\s*\{[^}]*left:\s*50%/)
    // подсветка и табличка: пока тащат — обводка, при наведении — заливка и горизонтальная табличка
    expect(markup).toMatch(/class:over=\{drag\.over === `backdrop:\$\{scope\}`/)
    expect(markup).toMatch(/\{#if z\.label\}<span class="cap">/)
    expect(style).toMatch(/\.narrow\.live\s*\{[^}]*outline:\s*2px solid var\(--accent\)/)
    expect(style).toMatch(/\.narrow\.over\s*\{[^}]*rgba\(245, 217, 10, 0\.45\)/)
    expect(style).toMatch(/\.narrow\.over \.cap\s*\{[^}]*bottom:\s*100%/)
    expect(style).not.toMatch(/writing-mode/)
  })
})
