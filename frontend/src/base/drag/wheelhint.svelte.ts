// Подсказка «Колесо — листать развороты» под призраком (095 §В): долгий перенос фото — один раз за запуск
// программы; кто уже крутил колесо при переносе, её не увидит. В config не пишется: сеанс = запуск.

export const HINT_MS = 700
export const HINT_TEXT = 'Колесо — листать развороты'

export interface HintState {
  shown: boolean // уже показывали в этом запуске
  wheeled: boolean // колесо при переносе уже крутили
  photos: boolean // тащат фото (не разворот)
  since: number // начало переноса, мс
}

export const hintDue = (s: HintState, now: number): boolean => s.photos && !s.shown && !s.wheeled && now - s.since >= HINT_MS

class WheelHint {
  show = $state(false)
  private s: HintState = { shown: false, wheeled: false, photos: false, since: 0 }
  private timer: ReturnType<typeof setTimeout> | undefined

  // Перенос стал активным (после порога сдвига).
  begin(photos: boolean, now = performance.now()) {
    this.end()
    this.s.photos = photos
    this.s.since = now
    if (!hintDue({ ...this.s, since: -Infinity }, now)) return // ничего не покажем — таймер не нужен
    this.timer = setTimeout(() => this.check(), HINT_MS)
  }

  check(now = performance.now()) {
    if (!hintDue(this.s, now)) return
    this.s.shown = true
    this.show = true
  }

  // Колесо во время переноса.
  wheel() {
    this.s.wheeled = true
  }

  // Конец переноса (drag.reset): строка гаснет, «уже показывали» остаётся.
  end() {
    clearTimeout(this.timer)
    this.timer = undefined
    this.s.photos = false
    this.show = false
  }

  // Для тестов: новый запуск программы.
  fresh() {
    this.end()
    this.s = { shown: false, wheeled: false, photos: false, since: 0 }
  }
}

export const wheelHint = new WheelHint()
