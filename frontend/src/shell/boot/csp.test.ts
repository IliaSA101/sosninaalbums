import { describe, expect, it } from 'vitest'
import { releaseCsp } from './csp'
import html from '../../../index.html?raw'

const cspOf = (s: string) => /http-equiv="Content-Security-Policy"\s+content="([^"]*)"/.exec(s)?.[1] ?? ''

describe('CSP index.html', () => {
  it('dev: воркер HMR из blob: разрешён, ws:// тоже', () => {
    const csp = cspOf(html)
    expect(csp).toContain("worker-src 'self' blob:")
    expect(csp).toContain('ws://localhost:*')
  })

  it('выпуск: нет worker-src/blob: для воркеров и ws://, остальное прежнее', () => {
    const csp = cspOf(releaseCsp(html))
    expect(csp).not.toContain('worker-src')
    expect(csp).not.toContain('ws://')
    expect(csp).toBe(
      "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-src 'none'",
    )
  })

  it('без dev-частей html не меняется', () => {
    expect(releaseCsp('<p>x</p>')).toBe('<p>x</p>')
  })

  it('переставленный ws:// в CSP: сборка падает, а не выпускает dev-политику', () => {
    const moved = html.replace(' ws://localhost:* ws://127.0.0.1:*', ' ws://127.0.0.1:*  ws://localhost:*')
    expect(moved).not.toBe(html)
    expect(() => releaseCsp(moved)).toThrow(/ws:\/\//)
  })

  it('переставленный worker-src в CSP: ошибка', () => {
    const moved = html.replace("; worker-src 'self' blob:", ";  worker-src 'self' blob:")
    expect(moved).not.toBe(html)
    expect(() => releaseCsp(moved)).toThrow(/worker-src/)
  })
})
