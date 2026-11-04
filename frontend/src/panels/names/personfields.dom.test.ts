// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// «Имя на фото» строкой (спека 124, 0.26-124-3, -16, -22): Фамилия, Имя, Отчество в одном ряду в панели и в узкой карточке,
// короткие подсказки в узкой, полные слова в aria-label, title — значение; «Предмет» — строкой ниже.
const person = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    Person: person,
    PlacePerson: person,
  }
})

const { default: PersonFields } = await import('./PersonFields.svelte')
const { cardContext } = await import('../../base/prefs/cardgroups.svelte')

const view = (over: object = {}) => ({
  last: '',
  first: '',
  middle: '',
  subject: '',
  teacher: false,
  fromFile: false,
  place: false,
  ...over,
})
const docked = { context: new Map([cardContext(() => true)]) }
const floating = { context: new Map([cardContext(() => false)]) }
type Ctx = { context: Map<unknown, unknown> }
const show = (props: Record<string, unknown>, ctx: Ctx = docked) => {
  const r = render(PersonFields, { props: { photo: 'a', row: true, cap: false, ...props } as never, ...ctx })
  return r
}
const fio = () => document.querySelector('.fio') as HTMLElement
const input = (name: string) => screen.getByLabelText(name) as HTMLInputElement

beforeEach(() => person.mockResolvedValue(view()))
afterEach(() => {
  cleanup()
  person.mockReset()
})

describe('PersonFields row', () => {
  it.each([
    ['в панели деталей', docked],
    ['в узкой карточке', floating],
  ])('три поля в одном ряду %s, «Предмет» — строкой ниже', async (_n, ctx) => {
    person.mockResolvedValue(view({ teacher: true, subject: 'Математика' }))
    show({}, ctx)
    await waitFor(() => expect(input('Предмет').value).toBe('Математика'))
    const row = [...fio().querySelectorAll('input')].map((i) => i.getAttribute('aria-label'))
    expect(row).toEqual(['Фамилия', 'Имя', 'Отчество'])
    expect(fio().contains(input('Предмет'))).toBe(false)
    expect(fio().nextElementSibling).toBe(input('Предмет')) // сразу под рядом
  })

  it('в панели подсказки полные, в узкой — «Фам.», «Имя», «Отч.»; aria-label везде полный', () => {
    const { unmount } = show({}, docked)
    expect(['Фамилия', 'Имя', 'Отчество'].map((n) => input(n).placeholder)).toEqual(['Фамилия', 'Имя', 'Отчество'])
    unmount()
    show({}, floating)
    expect(['Фамилия', 'Имя', 'Отчество'].map((n) => input(n).placeholder)).toEqual(['Фам.', 'Имя', 'Отч.'])
  })

  it('title поля — его значение (и имя из файла), пустое — полное слово', async () => {
    person.mockResolvedValue(view({ last: 'Александровская-Ивановна', first: 'Анна', fromFile: true }))
    show({}, floating)
    await waitFor(() => expect(input('Фамилия').value).toBe('Александровская-Ивановна'))
    expect(input('Фамилия').title).toBe('Александровская-Ивановна')
    expect(input('Имя').title).toBe('Анна')
    expect(input('Отчество').title).toBe('Отчество') // пустое
    expect(screen.getByText('из имени файла')).toBeTruthy()
    expect(input('Фамилия').classList.contains('file')).toBe(true) // серым: имя пока из файла
  })

  it('onsummary: «нет», потом «Иванов Иван»', async () => {
    const got: string[] = []
    show({ onsummary: (t: string) => got.push(t) })
    flushSync()
    expect(got.at(-1)).toBe('нет')
    await fireEvent.input(input('Фамилия'), { target: { value: 'Иванов' } })
    await fireEvent.input(input('Имя'), { target: { value: 'Иван' } })
    expect(got.at(-1)).toBe('Иванов Иван')
  })

  it('cap=false — подписи «Имя на фото» в теле нет (её даёт группа), с cap — есть (карточка места: «Имя на месте»)', () => {
    const a = show({}, docked)
    expect(document.querySelector('.cap')).toBeNull()
    expect(document.querySelector('.person')!.classList.contains('bare')).toBe(true)
    a.unmount()
    const b = show({ cap: true, photo: '', place: { spread: 's', id: 'p' } }, docked)
    expect(screen.getByText('Имя на месте')).toBeTruthy()
    expect(document.querySelector('.person')!.classList.contains('bare')).toBe(false)
    expect(fio().querySelectorAll('input')).toHaveLength(3)
    b.unmount()
  })

  it('без row поля столбцом, как раньше (окошко имени и прочие)', () => {
    show({ row: false, cap: true }, docked)
    expect(document.querySelector('.fio')).toBeNull()
    expect(screen.getAllByRole('textbox')).toHaveLength(3)
  })
})
