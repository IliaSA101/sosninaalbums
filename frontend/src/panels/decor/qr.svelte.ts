// Окно «QR-код» (100 п. 2): ссылка, образец от Go, «Поставить». Образец — «Сохранить разворот…» (tplsave.svelte.ts):
// окно поверх окна «Стикеры», закрывается вместе с ним. Код строит и ставит Go одним шагом отмены (AddQRSticker).
import { Api } from '../../base/core/store.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { currentPage, sticker } from '../../editor/sticker/sticker.svelte'
import { decor } from './decor.svelte'
import { QR_DELAY, type QRInfo } from './qr'

class QrForm {
  open = $state(false)
  link = $state('')
  info = $state<QRInfo | null>(null)
  error = $state('') // ошибка Go под полем: пусто, управляющие знаки, длиннее предела
  busy = $state(false)
  private n = 0 // номер запроса образца: ответ на устаревший ввод отбрасывается
  private timer: ReturnType<typeof setTimeout> | undefined

  show() {
    this.reset()
    this.open = true
  }

  close() {
    this.reset()
    this.open = false
  }

  private reset() {
    clearTimeout(this.timer)
    this.n++
    this.link = ''
    this.info = null
    this.error = ''
    this.busy = false
  }

  // Ввод в поле: образец обновляется через QR_DELAY после последней буквы.
  input(v: string) {
    this.link = v
    this.error = '' // старая ошибка не держит «Поставить» серой, пока образец не пришёл: Enter сразу после правки ставит (Go проверит сам)
    clearTimeout(this.timer)
    this.timer = setTimeout(() => void this.preview(), QR_DELAY)
  }

  async preview() {
    const my = ++this.n
    try {
      const info = (await Api.QRPreview(this.link)) as QRInfo
      if (my !== this.n) return
      this.info = info
      this.error = ''
    } catch (e) {
      if (my !== this.n) return
      this.info = null
      this.error = capFirst(errText(e))
    }
  }

  // «Поставить» серая: ошибка под полем, пустое поле, библиотека только для чтения, идёт постановка.
  get canPut(): boolean {
    return !this.busy && !this.error && this.link.trim() !== '' && !decor.cat?.readOnly
  }

  // «Поставить» (Enter): код в «Мои» и стикером на текущую страницу. Успех закрывает оба окна;
  // отказ Go (например, на листе уже 64 стикера) — красный тост, окно остаётся.
  async commit(): Promise<boolean> {
    const c = decor.cur
    if (!this.open || !c || this.busy) return false
    if (this.link.trim() === '') {
      await this.preview() // Enter в пустом поле: показать причину, а не молчать
      return false
    }
    if (!this.canPut) return false
    clearTimeout(this.timer)
    this.busy = true
    try {
      if (!(await sticker.addQR(c.spread, currentPage(), this.link))) return false
    } finally {
      this.busy = false
    }
    this.close()
    decor.close()
    return true
  }
}

export const qrForm = new QrForm()
