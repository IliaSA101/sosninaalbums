// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Окно «Отправить по ссылке» (146) на заглушке службы: все состояния design §3.4, переходы между ними, ход по событиям share:*.
// Решения в Go; тут проверяется, что окно показывает нужное по ShareView, зовёт нужный метод и не придумывает ничего своего.
const ev = vi.hoisted(() => ({ on: new Map<string, (d: unknown) => void>() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (d: unknown) => void) => {
    ev.on.set(name, cb)
    return () => ev.on.delete(name)
  },
}))
const api = vi.hoisted(() => ({ SaveProject: vi.fn(async () => ({})) }))
vi.mock('../../../wailsjs/go/app/App', () => api)

const { default: ShareDialog } = await import('./ShareDialog.svelte')
const { share } = await import('./share.svelte')
const { StubShare } = await import('./sharestub')
const { store } = await import('../../base/core/store.svelte')

type Kind = ConstructorParameters<typeof StubShare>[0]
const btn = (name: string | RegExp) => screen.getByRole('button', { name })
const noBtn = (name: string | RegExp) => screen.queryByRole('button', { name })
const field = (name: string) => screen.getByRole('textbox', { name }) as HTMLInputElement
const notify = vi.spyOn(store, 'notify')
const writeText = vi.fn(async (_t: string) => {})
let stub: InstanceType<typeof StubShare>

// Открыть окно на сценарии; события Go — через ev.on.
async function open(kind: Kind) {
  stub = new StubShare(kind)
  share.bridge = stub
  render(ShareDialog)
  await share.show()
  await waitFor(() => expect(screen.getByRole('dialog')).toBeTruthy())
}
const emit = (name: string, d: unknown) => ev.on.get(name)?.(d)
const flush = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  localStorage.clear()
  notify.mockReset()
  writeText.mockReset()
  api.SaveProject.mockClear()
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  share.close()
  share.view = null
  share.progress = null
  share.stopping = false
  share.working = false
  share.waiting = false
  share.error = ''
})
afterEach(() => {
  cleanup()
  share.close()
})

describe('не подключено', () => {
  it('Адрес и Токен без запоминания, токен точками, строка про данные «этот сервер», Подключить серая до ввода', async () => {
    await open('unplugged')
    const url = field('Адрес')
    const token = screen.getByLabelText('Токен') as HTMLInputElement
    for (const el of [url, token]) expect(el.getAttribute('autocomplete')).toBe('off')
    expect(token.type).toBe('password')
    expect(screen.getByText(/Адрес и токен даёт владелец сервиса листалок/)).toBeTruthy()
    expect(screen.getByText(/уходят на этот сервер\. Их увидит любой, у кого есть ссылка \(и пароль, если задан\)/)).toBeTruthy()
    expect((btn('Подключить') as HTMLButtonElement).disabled).toBe(true)
    await fireEvent.input(url, { target: { value: 'https://albums.example.org' } })
    expect((btn('Подключить') as HTMLButtonElement).disabled).toBe(true) // токена нет
    await fireEvent.input(token, { target: { value: 'secret' } })
    expect((btn('Подключить') as HTMLButtonElement).disabled).toBe(false)
  })

  it('«Подключить» → форма; в вызов уходит адрес без пробелов, журнал заглушки токена не хранит', async () => {
    await open('unplugged')
    await fireEvent.input(field('Адрес'), { target: { value: ' https://albums.example.org ' } })
    await fireEvent.input(screen.getByLabelText('Токен'), { target: { value: 'secret' } })
    await fireEvent.click(btn('Подключить'))
    await waitFor(() => expect(field('Название').value).toBe('Свадьба И. и А.'))
    expect(stub.calls.find((c) => c.startsWith('ShareConnect'))).toBe('ShareConnect ["https://albums.example.org","***"]')
    expect(screen.getByText('Сервис: albums.example.org')).toBeTruthy()
  })

  it('отказ Go — под полем словами Go, окно остаётся на подключении; повтор исправляет', async () => {
    await open('unplugged')
    await fireEvent.input(field('Адрес'), { target: { value: 'https://albums.example.org' } })
    await fireEvent.input(screen.getByLabelText('Токен'), { target: { value: 'bad' } })
    await fireEvent.click(btn('Подключить'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Токен не подошёл'))
    expect(btn('Подключить')).toBeTruthy()
    await fireEvent.input(screen.getByLabelText('Токен'), { target: { value: 'good' } })
    await fireEvent.click(btn('Подключить'))
    await waitFor(() => expect(field('Название')).toBeTruthy())
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('ссылка, выгруженная раньше, видна и без подключения: копируется, но не правится', async () => {
    await open('published')
    stub.view.service.token = false
    stub.view.service.host = ''
    await share.refresh()
    await waitFor(() => expect(screen.getByTestId('share-url').textContent).toContain('/a/k3Jx7Qw2/'))
    expect(btn('Скопировать для клиента')).toBeTruthy()
    expect(noBtn('Изменить')).toBeNull()
    expect(noBtn('Продлить')).toBeNull()
    expect(noBtn('Отозвать ссылку')).toBeNull()
    expect(field('Адрес')).toBeTruthy()
    // Главная кнопка экрана одна — «Скопировать для клиента»; «Подключить» обычная
    const primary = [...document.querySelectorAll('button.primary')]
    expect(primary.map((b) => b.textContent?.trim())).toEqual(['Скопировать для клиента'])
    expect(btn('Подключить').classList.contains('primary')).toBe(false)
  })

  it('без ссылки «Подключить» — главная кнопка с автофокусом', async () => {
    await open('unplugged')
    expect([...document.querySelectorAll('button.primary')].map((b) => b.textContent?.trim())).toEqual(['Подключить'])
    expect(btn('Подключить').hasAttribute('data-autofocus')).toBe(true)
  })
})

describe('форма и ход', () => {
  it('поля формы: название из альбома, пароль без запоминания, «Придумать», срок помнится; сводка', async () => {
    await open('plugged')
    expect(field('Название').value).toBe('Свадьба И. и А.')
    expect(field('Пароль').getAttribute('autocomplete')).toBe('off')
    expect(field('Название').getAttribute('autocomplete')).toBe('off')
    expect(screen.getByText('62 страницы до 1600 px, ≈ 24 МБ.')).toBeTruthy()
    expect(screen.getByText('Сервис: albums.sosnina-films.ru')).toBeTruthy()
    expect(screen.getByText(/уходят на сервер albums\.sosnina-films\.ru\./)).toBeTruthy()
    await fireEvent.click(btn('Придумать'))
    await waitFor(() => expect(field('Пароль').value).toBe('лето-4827'))
    await fireEvent.click(btn('Год'))
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: 'лето-4827', days: 365 }))
    expect(JSON.parse(localStorage.getItem('shareDays')!)).toBe(365)
  })

  it('пустое название — «Выгрузить» серая; Enter в поле запускает выгрузку', async () => {
    await open('plugged')
    await fireEvent.input(field('Название'), { target: { value: '  ' } })
    expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(true)
    await fireEvent.input(field('Название'), { target: { value: 'Мой альбом' } })
    await fireEvent.keyDown(field('Название'), { key: 'Enter' })
    await waitFor(() => expect(stub.lastOptions?.title).toBe('Мой альбом'))
    expect(stub.lastOptions?.password).toBe('')
  })

  it('«Отмена» закрывает окно, ничего не отправив', async () => {
    await open('plugged')
    await fireEvent.click(btn('Отмена'))
    expect(share.open).toBe(false)
    expect(stub.lastOptions).toBeNull()
  })

  it('ход: выгрузка → «Готовим развороты», «Загружено … МБ», «Публикуем…», «Остановить»; итог — тост и ссылка', async () => {
    await open('plugged')
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(screen.getByLabelText('Ход выгрузки')).toBeTruthy())
    expect(screen.getByText('Готовим…')).toBeTruthy()
    emit('share:progress', { phase: 'pages', done: 12, total: 62, bytes: 0, totalBytes: 0 })
    await waitFor(() => expect(screen.getByText('Готовим развороты: 12 из 62')).toBeTruthy())
    emit('share:progress', { phase: 'upload', done: 18, total: 40, bytes: 7e6, totalBytes: 16e6 })
    await waitFor(() => expect(screen.getByText('Загружено 18 из 40 · 7 из 16 МБ')).toBeTruthy())
    expect((screen.getByLabelText('Ход выгрузки') as HTMLProgressElement).value).toBe(44)
    emit('share:progress', { phase: 'publish', done: 0, total: 0, bytes: 0, totalBytes: 0 })
    await waitFor(() => expect(screen.getByText('Публикуем…')).toBeTruthy())
    expect(screen.getByText('Окно можно закрыть — выгрузка продолжится.')).toBeTruthy()
    // Go опубликовал: состояние теперь с ссылкой
    stub.view = { ...stub.view, busy: false, link: (await new StubShare('published').ShareState()).link }
    emit('share:done', { err: '', stopped: false })
    await waitFor(() => expect(screen.getByTestId('share-url')).toBeTruthy())
    expect(notify).toHaveBeenCalledWith('Ссылка готова', 'info')
    expect(screen.getByText('Выгружено 29 октября 2026 как «Свадьба И. и А.»')).toBeTruthy()
  })

  it('«Остановить»: CancelShare один раз, кнопка «Останавливаем…»; share:done со stopped — красный тост', async () => {
    await open('running')
    await fireEvent.click(btn('Остановить'))
    await fireEvent.click(screen.getByRole('button', { name: 'Останавливаем…' }))
    expect(stub.calls.filter((c) => c === 'CancelShare')).toHaveLength(1)
    stub.view.busy = false
    emit('share:done', { err: '', stopped: true })
    await waitFor(() => expect(noBtn('Остановить')).toBeNull())
    expect(notify).toHaveBeenCalledWith('Выгрузка остановлена', 'error')
    await waitFor(() => expect(share.stopping).toBe(false))
  })

  it('окно закрыто во время хода: событие всё равно даёт тост; при открытии виден ход', async () => {
    await open('plugged')
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(screen.getByLabelText('Ход выгрузки')).toBeTruthy())
    share.close()
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    emit('share:progress', { phase: 'upload', done: 2, total: 40, bytes: 1e6, totalBytes: 16e6 })
    await share.show()
    await waitFor(() => expect(screen.getByText('Загружено 2 из 40 · 1 из 16 МБ')).toBeTruthy())
    share.close()
    emit('share:done', { err: 'сеть недоступна', stopped: false })
    expect(notify).toHaveBeenCalledWith('Выгрузка не удалась: сеть недоступна', 'error')
  })

  it('ошибка запуска (идёт экспорт, пропавшие фото…): тост словами Go, окно возвращается к форме', async () => {
    await open('plugged')
    stub.fail.Share = 'сейчас идёт экспорт — дождитесь его конца'
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Сейчас идёт экспорт — дождитесь его конца', 'error'))
    expect(btn('Выгрузить')).toBeTruthy()
    expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(false)
  })

  it('401 посреди хода: сообщение в тосте, токен убран у Go — окно просит подключиться заново', async () => {
    await open('running')
    stub.view = {
      ...stub.view,
      busy: false,
      service: { host: '', url: 'https://albums.sosnina-films.ru', token: false, store: '', days: 0 },
    }
    emit('share:done', { err: 'токен не подошёл — подключите сервис заново', stopped: false })
    await waitFor(() => expect(btn('Подключить')).toBeTruthy())
    expect(notify).toHaveBeenCalledWith('Выгрузка не удалась: токен не подошёл — подключите сервис заново', 'error')
    expect(field('Адрес').value).toBe('https://albums.sosnina-films.ru')
  })

  it('«Отключить»: сервис забыт, адрес остаётся в поле', async () => {
    await open('plugged')
    await fireEvent.click(btn('Отключить'))
    await waitFor(() => expect(btn('Подключить')).toBeTruthy())
    expect(field('Адрес').value).toBe('https://albums.sosnina-films.ru')
  })
})

describe('ссылка', () => {
  it('ok: ссылка текстом, дата из Go, пароль и срок, без блока «изменён»', async () => {
    await open('published')
    expect(screen.getByTestId('share-url').textContent).toBe('https://albums.sosnina-films.ru/a/k3Jx7Qw2/')
    expect(screen.getByText('Выгружено 29 октября 2026 как «Свадьба И. и А.»')).toBeTruthy()
    expect(screen.getByText('Пароль: лето-4827')).toBeTruthy()
    expect(screen.getByText('Срок: до 29 ноября 2026')).toBeTruthy()
    expect(screen.queryByText('Альбом изменён после выгрузки')).toBeNull()
    expect(btn('Открыть')).toBeTruthy()
    expect(btn('Отозвать ссылку')).toBeTruthy()
  })

  it('«Скопировать для клиента»: текст Go в буфере, тост; отказ буфера — свой текст', async () => {
    await open('published')
    await fireEvent.click(btn('Скопировать для клиента'))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(stub.text))
    expect(notify).toHaveBeenCalledWith('Текст для клиента скопирован', 'info')
    writeText.mockRejectedValueOnce(new Error('NotAllowedError: Write permission denied.'))
    await fireEvent.click(btn('Скопировать для клиента'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Не удалось скопировать в буфер обмена', 'error'))
  })

  it('«Скопировать» при отказе Go (ShareText) — слова Go, буфер не тронут', async () => {
    await open('published')
    stub.fail.ShareText = 'ссылки нет'
    await fireEvent.click(btn('Скопировать для клиента'))
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Ссылки нет', 'error'))
    expect(writeText).not.toHaveBeenCalled()
  })

  it('«Открыть» зовёт OpenShare', async () => {
    await open('published')
    await fireEvent.click(btn('Открыть'))
    await waitFor(() => expect(stub.calls).toContain('OpenShare'))
  })

  it('пароль: Изменить → поле без запоминания → Сохранить; пусто снимает; Отмена ничего не шлёт', async () => {
    await open('published')
    await fireEvent.click(btn('Изменить'))
    const f = field('Пароль')
    expect(f.getAttribute('autocomplete')).toBe('off')
    expect(f.value).toBe('лето-4827')
    await fireEvent.click(btn('Отмена'))
    expect(stub.lastChange).toBeNull()
    await fireEvent.click(btn('Изменить'))
    await fireEvent.input(field('Пароль'), { target: { value: '' } })
    await fireEvent.click(btn('Сохранить'))
    await waitFor(() => expect(screen.getByText('Без пароля')).toBeTruthy())
    expect(stub.lastChange).toEqual({ days: 0, setPassword: true, password: '' })
  })

  it('пароль: «Придумать» подставляет значение Go', async () => {
    await open('published')
    await fireEvent.click(btn('Изменить'))
    await fireEvent.input(field('Пароль'), { target: { value: 'x' } })
    await fireEvent.click(btn('Придумать'))
    await waitFor(() => expect(field('Пароль').value).toBe('лето-4827'))
  })

  it('срок: Продлить → Месяц · 3 месяца · Год → один вызов ShareChange со сроком, пароль не трогается', async () => {
    await open('published')
    await fireEvent.click(btn('Продлить'))
    expect(screen.getByText('Срок считается от сегодня.')).toBeTruthy()
    await fireEvent.click(btn('3 месяца'))
    await waitFor(() => expect(stub.lastChange).toEqual({ days: 90, setPassword: false, password: '' }))
    await waitFor(() => expect(screen.getByText('Срок: до 29 декабря 2026')).toBeTruthy())
    await waitFor(() => expect(noBtn('3 месяца')).toBeNull())
  })

  it('changed: «Обновить по ссылке» шлёт только days 0 — название, пароль и срок служба берёт из записи; кнопка обычная, «Закрыть» без ghost', async () => {
    await open('changed')
    expect(screen.getByText('Альбом изменён после выгрузки')).toBeTruthy()
    expect(btn('Обновить по ссылке').classList.contains('primary')).toBe(false)
    expect(screen.getByText('Закрыть', { selector: 'button:not(.x)' }).classList.contains('ghost')).toBe(false)
    await fireEvent.click(btn('Обновить по ссылке'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ days: 0 }))
    expect(stub.calls).toContain('Share {"days":0}')
  })

  it('отказ Go при смене пароля или срока: редактор остаётся открытым с введённым', async () => {
    await open('published')
    await fireEvent.click(btn('Изменить'))
    await fireEvent.input(field('Пароль'), { target: { value: 'новый' } })
    stub.fail.ShareChange = 'пароль слишком простой'
    await fireEvent.click(btn('Сохранить'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Пароль слишком простой'))
    expect(notify).not.toHaveBeenCalledWith('Пароль слишком простой', 'error') // строкой под полем, не тостом
    expect(field('Пароль').getAttribute('aria-invalid')).toBe('true')
    expect(field('Пароль').value).toBe('новый')
    await fireEvent.click(btn('Отмена'))
    await fireEvent.click(btn('Продлить'))
    stub.fail.ShareChange = 'сервис недоступен'
    await fireEvent.click(btn('Год'))
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Сервис недоступен'))
    expect(notify).not.toHaveBeenCalledWith('Сервис недоступен', 'error')
    expect(btn('Год')).toBeTruthy() // выбор срока не закрылся
    await fireEvent.click(btn('Отмена'))
    expect(screen.queryByRole('alert')).toBeNull() // «Отмена» убирает строку
  })

  it('правка альбома при открытом окне перечитывает состояние: появляется «изменён»', async () => {
    await open('published')
    expect(screen.queryByText('Альбом изменён после выгрузки')).toBeNull()
    stub.view.link!.state = 'changed'
    store.view = { ...store.view, rev: store.view.rev + 1 }
    await waitFor(() => expect(screen.getByText('Альбом изменён после выгрузки')).toBeTruthy())
  })

  it('«Отозвать ссылку»: вопрос у Go; «Нет» — вид прежний, «Да» — форма и тост', async () => {
    await open('published')
    const real = stub.ShareRevoke.bind(stub)
    stub.ShareRevoke = async () => (stub.calls.push('ShareRevoke'), structuredClone(stub.view)) // «Нет» на вопрос Go
    await fireEvent.click(btn('Отозвать ссылку'))
    await waitFor(() => expect(stub.calls).toContain('ShareRevoke'))
    await flush()
    expect(screen.getByTestId('share-url')).toBeTruthy()
    expect(notify).not.toHaveBeenCalledWith('Ссылка отозвана', 'info')
    stub.ShareRevoke = real // «Да»
    await fireEvent.click(btn('Отозвать ссылку'))
    await waitFor(() => expect(field('Название')).toBeTruthy())
    expect(notify).toHaveBeenCalledWith('Ссылка отозвана', 'info')
  })

  it('«Сохранённый в системе» токен — без подсказки, токен только на сессию — «Токен не сохранится…» на подключённых экранах', async () => {
    await open('plugged')
    expect(screen.queryByText(/Токен не сохранится/)).toBeNull()
    stub.view.service.store = 'session'
    await share.refresh()
    await waitFor(() =>
      expect(screen.getByText('Токен не сохранится — в системе нет хранилища паролей, после перезапуска введите снова')).toBeTruthy(),
    )
    stub.view = new StubShare('published').view
    stub.view.service.store = 'session'
    await share.refresh()
    await waitFor(() => expect(screen.getByTestId('share-url')).toBeTruthy())
    expect(screen.getByText(/Токен не сохранится/)).toBeTruthy()
  })

  it('причина Go при готовой ссылке (не «сохранить»): ссылка остаётся, причина — строкой над ней; «Обновить» не пропадает', async () => {
    await open('changed')
    stub.view.album.block = 'Лист виньетки нельзя отправить по ссылке'
    await share.refresh()
    await waitFor(() => expect(screen.getByText('Лист виньетки нельзя отправить по ссылке')).toBeTruthy())
    expect(screen.getByTestId('share-url')).toBeTruthy()
    expect(btn('Отозвать ссылку')).toBeTruthy()
  })

  it('не сохранён при готовой ссылке — «Сохранить…» первым (ссылка записана в файл, который ещё не сохранён)', async () => {
    await open('published')
    stub.view.album.block = 'Сначала сохраните альбом — ссылка хранится у файла альбома'
    stub.view.album.save = true
    await share.refresh()
    await waitFor(() => expect(btn('Сохранить…')).toBeTruthy())
    expect(screen.queryByTestId('share-url')).toBeNull()
  })

  it('other: ссылка видна и копируется, правок нет, подсказка «подключите …»', async () => {
    await open('other')
    expect(screen.getByTestId('share-url')).toBeTruthy()
    expect(btn('Скопировать для клиента')).toBeTruthy()
    expect(noBtn('Изменить')).toBeNull()
    expect(noBtn('Продлить')).toBeNull()
    expect(noBtn('Отозвать ссылку')).toBeNull()
    expect(noBtn('Обновить по ссылке')).toBeNull()
    expect(screen.getByText('Ссылка выгружена на albums.example.org. Чтобы править её, подключите albums.example.org.')).toBeTruthy()
  })

  it('other → «Выгрузить заново»: форма с названием и паролем записи, выгрузка на текущий сервис; «Отмена» — назад к ссылке', async () => {
    await open('other')
    await fireEvent.click(btn('Выгрузить заново'))
    expect(field('Название').value).toBe('Свадьба И. и А.')
    expect(field('Пароль').value).toBe('лето-4827')
    expect(screen.queryByTestId('share-url')).toBeNull()
    await fireEvent.click(btn('Отмена'))
    expect(screen.getByTestId('share-url')).toBeTruthy()
    expect(share.open).toBe(true)
    await fireEvent.click(btn('Выгрузить заново'))
    await fireEvent.input(field('Пароль'), { target: { value: '' } })
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: '', days: 90 }))
    await waitFor(() => expect(screen.getByLabelText('Ход выгрузки')).toBeTruthy())
  })

  it('other без адреса (недоделанная запись): блока ссылки и «Скопировать» нет, «Выгрузить заново» есть', async () => {
    await open('other')
    stub.view.link!.url = ''
    await share.refresh()
    await waitFor(() => expect(screen.queryByTestId('share-url')).toBeNull())
    expect(noBtn('Скопировать для клиента')).toBeNull()
    expect(btn('Выгрузить заново')).toBeTruthy()
  })
})

describe('вместо формы', () => {
  it('incomplete: «Выгрузка не закончена» и «Продолжить» шлёт только days 0 — ни названия, ни пароля из записи (T5-6)', async () => {
    await open('incomplete')
    expect(screen.getByText('Выгрузка не закончена')).toBeTruthy()
    await fireEvent.click(btn('Продолжить'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ days: 0 }))
    expect(stub.calls).toContain('Share {"days":0}')
    expect(stub.calls.join()).not.toContain('лето-4827')
  })

  it('expired: «Срок истёк — альбом удалён с сервера» и «Выгрузить заново» с последним выбором срока', async () => {
    localStorage.setItem('shareDays', '30')
    await open('expired')
    expect(screen.getByText('Срок истёк — альбом удалён с сервера')).toBeTruthy()
    await fireEvent.click(btn('Выгрузить заново'))
    await waitFor(() => expect(stub.lastOptions).toEqual({ title: 'Свадьба И. и А.', password: 'лето-4827', days: 30 }))
  })

  it('gone: свой текст', async () => {
    await open('gone')
    expect(screen.getByText('Ссылку отозвали или она истекла — альбом удалён с сервера')).toBeTruthy()
    expect(btn('Выгрузить заново')).toBeTruthy()
  })

  it('не сохранён: причина Go и «Сохранить…» (SaveProject), после сохранения окно перечитывает состояние', async () => {
    await open('unsaved')
    expect(screen.getByText('Сначала сохраните альбом — ссылка хранится у файла альбома')).toBeTruthy()
    expect(noBtn('Выгрузить')).toBeNull()
    api.SaveProject.mockImplementationOnce(async () => {
      stub.view = new StubShare('plugged').view
      return {}
    })
    await fireEvent.click(btn('Сохранить…'))
    await waitFor(() => expect(field('Название')).toBeTruthy())
    expect(api.SaveProject).toHaveBeenCalledTimes(1)
  })

  it('причина без «Сохранить…» (например, лист виньетки): только «Закрыть»', async () => {
    await open('unsaved')
    stub.view.album.save = false
    stub.view.album.block = 'Лист виньетки нельзя отправить по ссылке'
    await share.refresh()
    await waitFor(() => expect(screen.getByText('Лист виньетки нельзя отправить по ссылке')).toBeTruthy())
    expect(noBtn('Сохранить…')).toBeNull()
    await fireEvent.click(screen.getByText('Закрыть', { selector: 'button:not(.x)' }))
    expect(share.open).toBe(false)
  })

  it('busyOther: «Идёт выгрузка другого альбома — дождитесь её конца», формы и чужой ссылки нет', async () => {
    await open('busyOther')
    expect(screen.getByText('Идёт выгрузка другого альбома — дождитесь её конца')).toBeTruthy()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(noBtn('Выгрузить')).toBeNull()
    expect(noBtn('Остановить')).toBeNull()
    expect(screen.queryByTestId('share-url')).toBeNull()
  })

  it('закрытое окно не рисуется; повторное открытие после закрытия показывает форму заново', async () => {
    await open('plugged')
    await fireEvent.input(field('Название'), { target: { value: 'Правка' } })
    share.close()
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await share.show()
    await waitFor(() => expect(field('Название').value).toBe('Свадьба И. и А.'))
  })
})

describe('доработка по QA стенда (T5b)', () => {
  it('«Остановить» → share:done: между «Останавливаем…» и ссылкой «Готовим… / Остановить» не мелькает', async () => {
    await open('running')
    emit('share:progress', { phase: 'publish', done: 0, total: 0, bytes: 0, totalBytes: 0 })
    await waitFor(() => expect(screen.getByText('Публикуем…')).toBeTruthy())
    await fireEvent.click(btn('Остановить'))
    const real = stub.ShareState.bind(stub)
    let release!: () => void
    stub.ShareState = () => new Promise((r) => (release = () => r(real())))
    stub.view = { ...stub.view, busy: false }
    emit('share:done', { err: '', stopped: true })
    await flush()
    expect(screen.getByRole('button', { name: 'Останавливаем…' })).toBeTruthy() // ответа Go ещё нет — ход прежний
    expect(noBtn('Остановить')).toBeNull()
    expect(screen.queryByText('Готовим…')).toBeNull()
    release()
    await waitFor(() => expect(screen.queryByLabelText('Ход выгрузки')).toBeNull())
    expect(share.stopping).toBe(false)
  })

  it('смена альбома при открытой странице: серые кнопки отпускает, форма берёт название нового альбома', async () => {
    await open('plugged')
    const key = (n: number) => ({ openSeq: n, view: { ...store.view, path: `/a${n}.salbum` } })
    Object.assign(store, key(1))
    await flush()
    await waitFor(() => expect(field('Название').value).toBe('Свадьба И. и А.'))
    stub.Share = () => new Promise(() => {}) // вызов повис: кнопки серые
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect((btn('Придумать') as HTMLButtonElement).disabled).toBe(true))
    stub.view.album.title = 'Другой альбом'
    stub.Share = StubShare.prototype.Share.bind(stub)
    Object.assign(store, key(2)) // OpenRecent
    await flush()
    await waitFor(() => expect(field('Название').value).toBe('Другой альбом'))
    expect((btn('Придумать') as HTMLButtonElement).disabled).toBe(false)
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(stub.lastOptions?.title).toBe('Другой альбом'))
  })

  it('отказ пароля на экране ссылки: красная строка под редактором, поле помечено, тоста нет', async () => {
    await open('published')
    await fireEvent.click(btn('Изменить'))
    await fireEvent.input(field('Пароль'), { target: { value: 'ab' } })
    stub.fail.ShareChange = 'пароль короче 4 знаков'
    await fireEvent.click(btn('Сохранить'))
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toBe('Пароль короче 4 знаков')
    expect(alert.classList.contains('err')).toBe(true)
    expect(alert.previousElementSibling?.getAttribute('data-part')).toBe('password')
    expect(notify).not.toHaveBeenCalled()
  })

  it('редактор пароля в одной линии: поле, «Придумать», «Отмена» слева от «Сохранить»', async () => {
    await open('published')
    await fireEvent.click(btn('Изменить'))
    const line = document.querySelector('[data-part="password"]') as HTMLElement
    expect(line.classList.contains('edit')).toBe(true) // класс edit держит flex-wrap: nowrap
    const names = [...line.querySelectorAll('button')].map((b) => b.textContent)
    expect(names).toEqual(['Придумать', 'Отмена', 'Сохранить'])
    expect(btn('Сохранить').classList.contains('primary')).toBe(true)
    expect(btn('Отмена').classList.contains('ghost')).toBe(true)
  })

  it('ширина окна одна на всех экранах: форма, ссылка, подключение, ход', async () => {
    const widths: string[] = []
    for (const kind of ['plugged', 'published', 'unplugged', 'running'] as const) {
      await open(kind)
      widths.push((screen.getByRole('dialog') as HTMLElement).style.width)
      share.close()
      cleanup()
    }
    expect(new Set(widths)).toEqual(new Set(['560px']))
  })

  it('«Отключить» — обычная кнопка в строке состояния справа, как «Выключить» в окне агентов (не ссылка .mini)', async () => {
    await open('plugged')
    const b = btn('Отключить')
    expect(b.classList.contains('mini')).toBe(false)
    expect(b.parentElement?.classList.contains('svc')).toBe(true)
    expect(b.parentElement?.textContent).toContain('Сервис: albums.sosnina-films.ru')
  })

  it('пока Go спрашивает (Share не отвечает): кнопки серые, строка «ответьте»; отказ в вопросе — форма активна, строки нет', async () => {
    await open('plugged')
    let answer!: (v: boolean) => void
    stub.Share = () => new Promise<boolean>((r) => (answer = r))
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain('Программа задала вопрос в отдельном окне'))
    expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(true)
    answer(false) // «Нет» на вопрос
    await waitFor(() => expect(screen.queryByText(/Программа задала вопрос/)).toBeNull())
    expect((btn('Выгрузить') as HTMLButtonElement).disabled).toBe(false)
    expect(notify).not.toHaveBeenCalled()
  })

  it('вопросы пройдены — строка исчезает, окно показывает ход', async () => {
    await open('plugged')
    let answer!: (v: boolean) => void
    stub.Share = () => new Promise<boolean>((r) => (answer = r))
    await fireEvent.click(btn('Выгрузить'))
    await waitFor(() => expect(screen.getByText(/Программа задала вопрос/)).toBeTruthy())
    stub.view.busy = true
    answer(true)
    await waitFor(() => expect(btn('Остановить')).toBeTruthy())
    expect(screen.queryByText(/Программа задала вопрос/)).toBeNull()
  })
})
