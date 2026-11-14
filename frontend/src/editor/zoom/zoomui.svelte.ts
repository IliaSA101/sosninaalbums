// Состояние бегунка масштаба (спека 016): ядро — zoomui.ts, здесь таймер гашения подсказки.
import { PEEK_MS, zoomUiNext, type ZoomUiEvent, type ZoomUiState } from './zoomui'

class ZoomUi {
  state = $state<ZoomUiState>('closed')
  // Курсор над бейджем или бегунком: подсказка не гаснет.
  private hovered = false
  private timer: ReturnType<typeof setTimeout> | undefined

  private stop() {
    clearTimeout(this.timer)
    this.timer = undefined
  }
  // Таймер живёт только в peek и не при наведении.
  private sync() {
    this.stop()
    if (this.state === 'peek' && !this.hovered) this.timer = setTimeout(() => this.send('expire'), PEEK_MS)
  }
  private send(e: ZoomUiEvent) {
    this.state = zoomUiNext(this.state, e)
    this.sync()
  }

  pulse() {
    if (this.state !== 'open') this.send('pulse') // в open — холостой вызов
  }
  hover(on: boolean) {
    this.hovered = on
    this.sync()
  }
  grab() {
    this.send('grab')
  }
  toggle() {
    this.send('toggle')
  }
  open() {
    this.send('open')
  }
  close() {
    if (this.state !== 'closed') this.send('close')
  }
}

export const zoomui = new ZoomUi()
