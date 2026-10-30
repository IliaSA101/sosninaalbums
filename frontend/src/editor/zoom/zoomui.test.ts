import { describe, expect, it } from 'vitest'
import { zoomUiNext as next, type ZoomUiEvent, type ZoomUiState } from './zoomui'

const run = (s: ZoomUiState, ...ev: ZoomUiEvent[]) => ev.reduce(next, s)

describe('zoomui', () => {
  it('closed→peek→closed по таймауту', () => {
    expect(run('closed', 'pulse')).toBe('peek')
    expect(run('closed', 'pulse', 'expire')).toBe('closed')
  })
  it('pulse продлевает: peek остаётся peek', () => expect(run('closed', 'pulse', 'pulse', 'pulse')).toBe('peek'))
  it('expire не трогает closed и open', () => {
    expect(run('closed', 'expire')).toBe('closed')
    expect(run('open', 'expire')).toBe('open')
  })
  it('grab из peek — open, и подсказка больше не гаснет', () => expect(run('closed', 'pulse', 'grab', 'expire')).toBe('open'))
  it('pulse в open — холостой', () => expect(run('open', 'pulse')).toBe('open'))
  it('toggle: closed→open→closed, peek→open', () => {
    expect(run('closed', 'toggle')).toBe('open')
    expect(run('closed', 'toggle', 'toggle')).toBe('closed')
    expect(run('peek', 'toggle')).toBe('open')
  })
  it('open и close', () => {
    expect(run('closed', 'open')).toBe('open')
    expect(run('open', 'close')).toBe('closed')
    expect(run('peek', 'close')).toBe('closed')
  })
})
