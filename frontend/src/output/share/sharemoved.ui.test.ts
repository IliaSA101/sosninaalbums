// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Одна ссылка на проект (167, design §7, §9): экран moved, поиск прежнего альбома, не своя запись. Окно на заглушке службы;
// решения в Go — здесь проверяется, что окно показывает нужное по ShareView и шлёт в Share нужный choice.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SaveProject: vi.fn(async () => ({})) }))

const { default: ShareDialog } = await import('./ShareDialog.svelte')
const { share } = await import('./share.svelte')
const { StubShare } = await import('./sharestub')
const { store } = await import('../../base/core/store.svelte')

type Kind = ConstructorParameters<typeof StubShare>[0]
const btn = (name: string | RegExp) => screen.getByRole('button', { name })
const noBtn = (name: string | RegExp) => screen.queryByRole('button', { name })
const field = (name: string) => screen.getByRole('textbox', { name }) as HTMLInputElement
const notify = vi.spyOn(store, 'notify')
let stub: InstanceType<typeof StubShare>

async function open(kind: Kind, after?: Kind) {
  stub = new StubShare(kind)
  if (after) stub.afterFind = after
  share.bridge = stub
  render(ShareDialog)
  await share.show()
  await waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
}
const finds = () => stub.calls.filter((c) => c === 'ShareFind').length
const shares = () => stub.calls.filter((c) => c.startsWith('Share ')).length

beforeEach(() => {
  localStorage.clear()
  notify.mockReset()
  share.close()
  share.view = null
  share.progress = null
  share.stopping = false
  share.working = false
  share.finding = false
  share.findError = ''
  share.waiting = false
  share.error = ''
})
afterEach(() => {
  cleanup()
  share.close()
})

describe('moved: вопрос «обновить или как новый»', () => {
  it('три обычные кнопки в порядке «Отмена», «Выгрузить как новый», «Обновить этот альбом»; фокус на «Отмена»; основной нет', async () => {
    await open('moved')
    const names = Array.from(screen.getByRole('dialog').querySelectorAll('.actions button')).map((b) => b.textContent)
    expect(names).toEqual(['Отмена', 'Выгрузить как новый', 'Обновить этот альбом'])
    expect(btn('Отмена').hasAttribute('data-autofocus')).toBe(true)
    expect(screen.getByRole('dialog').querySelector('button.primary')).toBeNull()
    expect(screen.getByRole('dialog').querySelectorAll('[data-autofocus]').length).toBe(1)
  })

  it('строки Go как пришли, ссылка текстом и «Открыть», вопрос и последствия прямо', async () => {
    await open('moved')
    expect(screen.getByTestId('share-url').textContent).toContain('/a/k3Jx7Qw2/')
    expect(screen.getByText('Выгружено 29 октября 2026 как «Иванов»')).toBeTruthy()
    expect(screen.getByText('Из папки D:\\Свадьбы, файл «Иванов.salbum»')).toBeTruthy()
    expect(screen.getByText('Этот файл: папка E:\\Работа, «Петров.salbum»')).toBeTruthy()
    expect(screen.getByText('Обновить этот альбом или выгрузить как новый?')).toBeTruthy()
    expect(screen.getByText('Обновить — страницы альбома у клиента заменятся страницами этого файла.')).toBeTruthy()
    expect(
      screen.getByText(
        'Как новый — прежняя ссылка останется у прежнего файла; если его больше нет, программа не сможет её показать и отозвать.',
      ),
    ).toBeTruthy()
    await fireEvent.click(btn('Открыть'))
    await waitFor(() => expect(stub.calls).toContain('OpenShare'))
  })

  it('moved: «Скопировать для клиента» нет, пароля нет, правок и отзыва нет', async () => {
    await open('moved')
    expect(noBtn('Скопировать для клиента')).toBeNull()
    expect(noBtn('Изменить')).toBeNull()
    expect(noBtn('Продлить')).toBeNull()
    expect(noBtn('Отозвать ссылку')).toBeNull()
    expect(screen.queryByText(/Пароль/)).toBeNull()
    expect(screen.queryByText('лето-4827')).toBeNull()
    expect(screen.queryByText('Свадьба И. и А.')).toBeNull() // название прежнего клиента — только в строке «Выгружено…»
  })

  it('«Обновить этот альбом» шлёт choice update и ничего из записи', async () => {
    await open('moved')
    await fireEvent.click(btn('Обновить этот альбом'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ days: 0, choice: 'update' }))
    await waitFor(() => expect(screen.getByLabelText('Ход выгрузки')).toBeTruthy())
  })

  it('«Выгрузить как новый» → форма без пароля прежнего клиента; «Выгрузить» шлёт choice new', async () => {
    await open('moved')
    await fireEvent.click(btn('Выгрузить как новый'))
    expect(field('Название').value).toBe('Петров') // имя этого файла (album.title из Go), не название прежнего клиента
    expect(field('Пароль').value).toBe('')
    expect(screen.queryByTestId('share-url')).toBeNull()
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Петров', password: '', days: 90, choice: 'new' }))
  })

  it('форма из moved: «Отмена» возвращает к вопросу, окно остаётся, Share не зовётся; потом можно выбрать снова', async () => {
    await open('moved')
    await fireEvent.click(btn('Выгрузить как новый'))
    await fireEvent.click(btn('Отмена'))
    expect(share.open).toBe(true)
    expect(screen.getByText('Обновить этот альбом или выгрузить как новый?')).toBeTruthy()
    expect(shares()).toBe(0)
    await fireEvent.click(btn('Обновить этот альбом'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ days: 0, choice: 'update' }))
  })

  it('«Отмена» на вопросе закрывает окно без вызовов Share', async () => {
    await open('moved')
    await fireEvent.click(btn('Отмена'))
    expect(share.open).toBe(false)
    expect(shares()).toBe(0)
  })

  it('другой компьютер: строка «На другом компьютере или из другого файла»', async () => {
    await open('movedElsewhere')
    expect(screen.getByText('На другом компьютере или из другого файла')).toBeTruthy()
    expect(screen.getByText('Выгружено как «Иванов»')).toBeTruthy() // Go даты не даёт — строка без неё, название видно
    expect(btn('Обновить этот альбом')).toBeTruthy()
  })

  it('запись без адреса (недоделанная): ссылки и «Открыть» нет, вопрос остаётся', async () => {
    await open('moved')
    stub.view.link!.url = ''
    await share.refresh()
    await waitFor(() => expect(screen.queryByTestId('share-url')).toBeNull())
    expect(noBtn('Открыть')).toBeNull()
    expect(btn('Выгрузить как новый')).toBeTruthy()
  })

  it('сбой «Обновить»: тост, кнопки снова доступны, экран прежний', async () => {
    await open('moved')
    stub.fail.Share = 'Прежнего альбома больше нет на сервере — выгрузите как новый'
    await fireEvent.click(btn('Обновить этот альбом'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Прежнего альбома больше нет на сервере — выгрузите как новый', 'error'))
    await waitFor(() => expect((btn('Обновить этот альбом') as HTMLButtonElement).disabled).toBe(false))
  })

  it('без подключения moved показывает форму подключения, не вопрос; ссылка без «Скопировать» и пароля', async () => {
    await open('moved')
    stub.view.service.token = false
    await share.refresh()
    await waitFor(() => expect(field('Адрес')).toBeTruthy())
    expect(noBtn('Скопировать для клиента')).toBeNull()
    expect(noBtn('Обновить этот альбом')).toBeNull()
    expect(screen.queryByText(/Пароль/)).toBeNull()
  })
})

describe('поиск прежнего альбома на сервисе', () => {
  it('need: строка поиска, неопределённая полоса, «Отмена»; ShareFind один раз; найден → moved', async () => {
    await open('find', 'movedElsewhere')
    await waitFor(() => expect(screen.getByText('На другом компьютере или из другого файла')).toBeTruthy())
    expect(finds()).toBe(1)
    expect(btn('Обновить этот альбом')).toBeTruthy()
  })

  it('пока ищем: «Альбом уже выгружали — ищем его на сервисе…», полоса без значения, «Отмена» закрывает, формы нет', async () => {
    stub = new StubShare('find')
    share.bridge = stub
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const orig = stub.ShareFind.bind(stub)
    stub.ShareFind = async () => (await gate, orig())
    render(ShareDialog)
    await share.show()
    await waitFor(() => expect(screen.getByText('Альбом уже выгружали — ищем его на сервисе…')).toBeTruthy())
    const bar = screen.getByRole('dialog').querySelector('progress') as HTMLProgressElement
    expect(bar.hasAttribute('value')).toBe(false)
    expect(screen.queryByRole('textbox', { name: 'Название' })).toBeNull()
    expect(btn('Отмена').hasAttribute('data-autofocus')).toBe(true)
    await fireEvent.click(btn('Отмена'))
    expect(share.open).toBe(false)
    release()
    await Promise.resolve()
  })

  it('после найденного альбома перечитывание вида второй ShareFind не зовёт', async () => {
    await open('find', 'movedElsewhere')
    await waitFor(() => expect(screen.getByText('На другом компьютере или из другого файла')).toBeTruthy())
    await share.refresh()
    expect(finds()).toBe(1)
  })

  it('не найден: форма со строкой «будет новая ссылка»; «Выгрузить» без choice', async () => {
    await open('find', 'lost')
    await waitFor(() => expect(screen.getByText('Прежний альбом на сервисе не найден — будет новая ссылка')).toBeTruthy())
    expect(field('Название').value).toBe('Свадьба И. и А.')
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: '', days: 90 }))
  })

  it('ошибка поиска: текст, «Повторить» и форма «Будет новая ссылка»; повтор находит альбом', async () => {
    await open('find', 'fail')
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Сервис не ответил — проверьте связь'))
    expect(screen.getByText('Будет новая ссылка')).toBeTruthy()
    expect(field('Название')).toBeTruthy()
    expect(finds()).toBe(1)
    stub.afterFind = 'movedElsewhere'
    await fireEvent.click(btn('Повторить'))
    await waitFor(() => expect(btn('Обновить этот альбом')).toBeTruthy())
    expect(finds()).toBe(2)
    expect(screen.queryByText('Будет новая ссылка')).toBeNull()
  })

  it('ошибка поиска: выгрузка без поиска — новая ссылка, без choice', async () => {
    await open('find', 'fail')
    await waitFor(() => expect(btn('Повторить')).toBeTruthy())
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: '', days: 90 }))
    expect(finds()).toBe(1)
  })

  it('отказ самого вызова (нет токена и т. п.): строка, «Повторить»; повтор проходит', async () => {
    stub = new StubShare('find')
    stub.fail.ShareFind = 'Подключите сервис'
    stub.afterFind = 'movedElsewhere'
    share.bridge = stub
    render(ShareDialog)
    await share.show()
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Подключите сервис'))
    expect(screen.getByText('Альбом уже выгружали — ищем его на сервисе…')).toBeTruthy()
    await waitFor(() => expect((btn('Повторить') as HTMLButtonElement).disabled).toBe(false)) // пока вид перечитывается, серая
    await fireEvent.click(btn('Повторить'))
    await waitFor(() => expect(btn('Обновить этот альбом')).toBeTruthy())
    expect(finds()).toBe(2)
  })

  it('повтор поиска на экране сбоя: «ищем…» обычной строкой, «Выгрузить» и «Повторить» серые', async () => {
    await open('find', 'fail')
    await waitFor(() => expect(btn('Повторить')).toBeTruthy())
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const orig = stub.ShareFind.bind(stub)
    stub.ShareFind = async () => (await gate, orig())
    stub.afterFind = 'movedElsewhere'
    await fireEvent.click(btn('Повторить'))
    await waitFor(() => expect(screen.getByText('Альбом уже выгружали — ищем его на сервисе…')).toBeTruthy())
    const line = screen.getByText('Альбом уже выгружали — ищем его на сервисе…')
    expect(line.classList.contains('err')).toBe(false)
    expect(line.getAttribute('role')).toBe('status')
    expect(screen.queryByRole('alert')).toBeNull()
    expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(true)
    expect((btn('Повторить') as HTMLButtonElement).disabled).toBe(true)
    release()
    await waitFor(() => expect(btn('Обновить этот альбом')).toBeTruthy())
  })

  it('отказ вызова при «Повторить» на экране сбоя: строка .err с причиной, «Выгрузить» снова доступна', async () => {
    await open('find', 'fail')
    await waitFor(() => expect(btn('Повторить')).toBeTruthy())
    stub.fail.ShareFind = 'Подключите сервис'
    await fireEvent.click(btn('Повторить'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Подключите сервис'))
    expect(screen.getByRole('alert').classList.contains('err')).toBe(true)
    await waitFor(() => expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(false))
    expect((btn('Повторить') as HTMLButtonElement).disabled).toBe(false)
  })

  it('без подключения поиска нет: форма подключения, ShareFind не зовётся', async () => {
    stub = new StubShare('find')
    stub.view.service.token = false
    share.bridge = stub
    render(ShareDialog)
    await share.show()
    await waitFor(() => expect(field('Адрес')).toBeTruthy())
    expect(finds()).toBe(0)
  })
})

describe('не своя запись: expired и other', () => {
  it('expired с другого пути: «Выгрузить заново» шлёт пустые title и password', async () => {
    await open('expiredForeign')
    expect(screen.getByText('Срок истёк — альбом удалён с сервера')).toBeTruthy()
    await fireEvent.click(btn('Выгрузить заново'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: '', password: '', days: 90 }))
    expect(stub.calls.join()).not.toContain('лето-4827')
  })

  it('expired своя запись: название и пароль записи уходят, как раньше', async () => {
    await open('expired')
    await fireEvent.click(btn('Выгрузить заново'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: 'лето-4827', days: 90 }))
  })

  it('other с другого пути: «Скопировать для клиента» и пароля нет; форма «Выгрузить заново» пуста, без старых значений', async () => {
    await open('otherForeign')
    expect(screen.getByTestId('share-url')).toBeTruthy()
    expect(btn('Открыть')).toBeTruthy()
    expect(noBtn('Скопировать для клиента')).toBeNull()
    expect(screen.queryByText(/Пароль/)).toBeNull()
    await fireEvent.click(btn('Выгрузить заново'))
    expect(field('Название').value).toBe('Петров')
    expect(field('Пароль').value).toBe('')
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Петров', password: '', days: 90 }))
  })
})

describe('пароль, заданный на другом компьютере', () => {
  it('своя запись с passwordUnknown: строка «Пароль задан на другом компьютере» и «Изменить»; пароль не показан', async () => {
    await open('published')
    stub.view.link!.password = ''
    stub.view.link!.passwordUnknown = true
    await share.refresh()
    await waitFor(() => expect(screen.getByText('Пароль задан на другом компьютере')).toBeTruthy())
    expect(screen.queryByText('Без пароля')).toBeNull()
    expect(btn('Изменить')).toBeTruthy()
    await fireEvent.click(btn('Изменить'))
    expect((screen.getByLabelText('Пароль') as HTMLInputElement).value).toBe('') // старый не известен — поле пусто
  })

  it('без passwordUnknown и без пароля — «Без пароля»', async () => {
    await open('published')
    stub.view.link!.password = ''
    await share.refresh()
    await waitFor(() => expect(screen.getByText('Без пароля')).toBeTruthy())
  })
})
