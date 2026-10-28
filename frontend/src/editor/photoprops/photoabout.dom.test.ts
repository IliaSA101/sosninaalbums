// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'

// «О снимке» (084, 106): ссылки «Использовано N раз: …» ведут через общий goUse, по виду вхождения.
const about = vi.hoisted(() => vi.fn())
const goUse = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ PhotoAbout: about }))
vi.mock('./gouse.svelte', () => ({ goUse }))

const { default: PhotoAbout } = await import('./PhotoAbout.svelte')

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

const data = {
  id: 'f1',
  file: '',
  shot: [],
  note: '',
  used: 'Использовано 2 раза',
  uses: [
    { sheet: 2, label: 'Разворот 3 — место', kind: 'place', spread: 's3', page: 'Разворот 3', what: 'место', pick: 'f1' },
    { sheet: -1, label: 'Фон альбома', kind: 'albumBg', spread: '', page: 'Фон альбома', what: '' },
  ],
}

describe('PhotoAbout: ссылки вхождений', () => {
  it('ссылка зовёт goUse со строкой вхождения; «Фон альбома» — вид albumBg', async () => {
    about.mockResolvedValue(data)
    render(PhotoAbout, { id: 'f1' })
    await fireEvent.click(await screen.findByText('Разворот 3', { selector: 'button' }))
    expect(goUse).toHaveBeenLastCalledWith(expect.objectContaining({ kind: 'place', spread: 's3', pick: 'f1' }))
    await fireEvent.click(screen.getByText('Фон альбома', { selector: 'button' }))
    expect(goUse).toHaveBeenLastCalledWith(expect.objectContaining({ kind: 'albumBg', spread: '' }))
  })
})
