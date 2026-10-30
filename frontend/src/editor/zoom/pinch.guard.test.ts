import { describe, expect, it } from 'vitest'

// Страж 134 «все места масштаба»: где Ctrl/⌘+колесо меняет масштаб, там должен быть и щипок Mac (pinchOn).
// Новое место (плитки окна, другая лупа) без щипка здесь краснеет, а не ждёт жалобы пользователя Mac.
const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

// Исключения — с причиной: файл лишь определяет предикат.
const EXCEPT: Record<string, string> = {
  '/src/panels/library/thumbzoom.ts': 'определение isThumbWheel (над ним isZoomWheel), без подписки на события',
}

const strip = (src: string): string =>
  src
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\/.*$/gm, '')
const calls = (src: string, name: string): boolean => new RegExp(`\\b${name}\\(`).test(src)

// Файл меняет масштаб колесом, но щипка не знает.
const missingPinch = (path: string, src: string): boolean => {
  if (/\.test\.ts$/.test(path) || path in EXCEPT) return false
  const c = strip(src)
  return (calls(c, 'isZoomWheel') || calls(c, 'isThumbWheel')) && !calls(c, 'pinchOn')
}

// Лупа: файл, который зовёт loupe.wheel( (колесо), должен звать и loupe.bind( (колесо и щипок вместе).
const loupeWithoutBind = (path: string, src: string): boolean => {
  if (/\.test\.ts$/.test(path)) return false
  const c = strip(src)
  return /\bloupe\.wheel\(/.test(c) && !/\bloupe\.bind\(/.test(c)
}

describe('щипок в каждом месте масштаба (134-7)', () => {
  it('файл с isZoomWheel(/isThumbWheel( зовёт и pinchOn(', () => {
    const bad = Object.entries(files)
      .filter(([p, s]) => missingPinch(p, s))
      .map(([p]) => p)
    expect(bad, 'pinchOn(node, sink) из editor/zoom/pinch или запись в EXCEPT с причиной').toEqual([])
  })

  it('исключения живые: файл есть и действительно зовёт предикат без pinchOn', () => {
    for (const p of Object.keys(EXCEPT)) {
      const c = strip(files[p] ?? '')
      expect(c, p).not.toBe('')
      expect(calls(c, 'isZoomWheel') || calls(c, 'isThumbWheel'), p).toBe(true)
      expect(calls(c, 'pinchOn'), p).toBe(false)
    }
  })

  it('самопроверка: страж видит место без щипка и пропускает комментарии', () => {
    expect(missingPinch('/src/x/A.svelte', 'if (isZoomWheel(e)) zoom()')).toBe(true)
    expect(missingPinch('/src/x/A.ts', 'if (isThumbWheel(e)) size()')).toBe(true)
    expect(missingPinch('/src/x/A.svelte', 'if (isZoomWheel(e)) zoom()\npinchOn(el, sink)')).toBe(false)
    expect(missingPinch('/src/x/A.ts', '// isZoomWheel(e) в комментарии\nconst a = 1')).toBe(false)
    expect(missingPinch('/src/x/A.test.ts', 'isZoomWheel(e)')).toBe(false)
  })

  it('лупа вешается через loupe.bind(, а не отдельным колесом', () => {
    const bad = Object.entries(files)
      .filter(([p, s]) => loupeWithoutBind(p, s))
      .map(([p]) => p)
    expect(bad, 'PreviewLoupe.bind(node) вешает и колесо, и щипок').toEqual([])
    expect(loupeWithoutBind('/src/x/A.svelte', 'node.addEventListener("wheel", (e) => loupe.wheel(e))')).toBe(true)
    expect(loupeWithoutBind('/src/x/A.svelte', '$effect(() => loupe.bind(back))')).toBe(false)
    expect(loupeWithoutBind('/src/x/A.svelte', 'loupe.wheel(e)\nloupe.bind(n)')).toBe(false)
  })

  it('страж видит места масштаба', () => {
    for (const p of [
      '/src/editor/canvas/Stage.svelte',
      '/src/panels/library/Library.svelte',
      '/src/panels/decorbrowse/tilewheel.ts',
      '/src/editor/photo/loupe.svelte.ts',
    ])
      expect(calls(strip(files[p] ?? ''), 'pinchOn'), p).toBe(true)
  })
})
