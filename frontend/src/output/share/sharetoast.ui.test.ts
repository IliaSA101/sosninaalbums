// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Тосты окна «Отправить по ссылке» после «Изменить» (пароль) и «Продлить» (159-Б). Вынесены из sharedialog.ui.test.ts:
// тот файл упёрся в предел eslint 600 строк. Чистая логика текстов — в share.test.ts, здесь — что окно их показывает.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SaveProject: vi.fn(async () => ({})) }))

const { default: ShareDialog } = await import('./ShareDialog.svelte')
const { share } = await import('./share.svelte')
const { StubShare } = await import('./sharestub')
const { store } = await import('../../base/core/store.svelte')

const btn = (name: string | RegExp) => screen.getByRole('button', { name })
const field = (name: string) => screen.getByRole('textbox', { name }) as HTMLInputElement
const notify = vi.spyOn(store, 'notify')
const flush = () => new Promise((r) => setTimeout(r, 0))
let stub: InstanceType<typeof StubShare>

async function open() {
  stub = new StubShare('published')
  share.bridge = stub
  render(ShareDialog)
  await share.show()
  await waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
}

beforeEach(() => {
  localStorage.clear()
  notify.mockReset()
  share.close()
  share.view = null
})
afterEach(() => {
  cleanup()
  share.close()
})

describe('тосты после правки ссылки', () => {
  it('«Сохранить» пароля: тост об изменении; тот же пароль снова — молчит', async () => {
    await open()
    await fireEvent.click(btn('Изменить'))
    await fireEvent.input(field('Пароль'), { target: { value: 'новый-пароль' } })
    await fireEvent.click(btn('Сохранить'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Пароль изменён — прежние входы клиента сброшены', 'info'))
    notify.mockClear()
    await fireEvent.click(btn('Изменить'))
    await fireEvent.click(btn('Сохранить')) // в поле тот же пароль
    await waitFor(() => expect(stub.calls.filter((c) => c.startsWith('ShareChange')).length).toBe(2))
    await flush()
    expect(notify).not.toHaveBeenCalled()
  })

  it('«Продлить»: тост «Срок ссылки — …» со строкой Go', async () => {
    await open()
    await fireEvent.click(btn('Продлить'))
    await fireEvent.click(btn('3 месяца'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Срок ссылки — до 29 декабря 2026', 'info'))
  })
})
