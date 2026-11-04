// Режим «Взять цвет с фото» (109, U2/U4): общий для окна цвета и слоя на холсте. Здесь — только владелец режима
// и обратный вызов; слой, лупа и вызов Go (PickColorAt) — editor/eyedrop (U4).
class Eyedrop {
  // Кто запустил режим (экземпляр окна). null — режима нет.
  owner = $state.raw<object | null>(null) // raw: без прокси, иначе сравнение с владельцем не сойдётся
  // Лист, с которого берём цвет (ID разворота); слой встаёт только над ним (109-u4-1).
  sheet = $state('')
  private done: ((hex: string) => void) | null = null
  // Нажатие погашено — щелчок того же жеста гасим тоже: иначе он снял бы выделение или закрыл окно сменой карточки.
  private mute = false
  // «Своё» окно режима: щелчок в нём (и по его кнопке) — не «мимо».
  private inside: ((t: Element) => boolean) | null = null

  get on(): boolean {
    return this.owner !== null
  }

  // Новый запуск отменяет прежний: режим один на программу.
  start(owner: object, sheet: string, onpick: (hex: string) => void, inside?: (t: Element) => boolean): void {
    this.owner = owner
    this.sheet = sheet
    this.done = onpick
    this.inside = inside ?? null
  }

  // Цвет взят: режим кончается, окно-владелец делает из цвета одну правку.
  finish(hex: string): void {
    const done = this.done
    this.cancel()
    done?.(hex)
  }

  // owner — снять режим, только если он ещё этого окна (окно закрылось, а режим уже чужой — не трогаем).
  cancel(owner?: object): void {
    if (owner && this.owner !== owner) return
    this.owner = null
    this.sheet = ''
    this.done = null
    this.inside = null
  }

  // Нажатие в режиме (109-10). Слушатель стоит на window в захвате и ставится при загрузке модуля — раньше любых
  // слушателей компонентов (TextLayer снимает выделение текста на том же window/capture, а порядок там — по
  // регистрации), поэтому глушим целиком: ни снятия выделения, ни рамки, ни закрытия окна цвета. Возвращает true,
  // если нажатие погашено.
  press(e: Event): boolean {
    this.mute = false // щелчка не было (увели курсор с нажатой кнопкой) — глушилка живёт только до следующего нажатия
    if (!this.owner) return false
    const t = e.target instanceof Element ? e.target : null
    if (!t || t.closest('[data-eyedrop]') || this.inside?.(t)) return false // слой пипетки и окно владельца — не «мимо»
    e.stopImmediatePropagation()
    e.preventDefault()
    this.cancel()
    this.mute = true
    return true
  }

  // Щелчок после погашенного нажатия. Возвращает true, если погашен.
  click(e: Event): boolean {
    if (!this.mute) return false
    this.mute = false
    e.stopImmediatePropagation()
    e.preventDefault()
    return true
  }

  // Текущий разворот сменился (или листа нет) — режим снимается: слой над другим листом не стоит (109-u4-1).
  check(current: string | undefined): void {
    if (this.owner && this.sheet !== current) this.cancel()
  }
}

export const eyedrop = new Eyedrop()

// Первым в очереди window/capture: модуль грузится до монтирования любого компонента.
if (typeof window !== 'undefined') {
  window.addEventListener('pointerdown', (e) => eyedrop.press(e), true)
  window.addEventListener('click', (e) => eyedrop.click(e), true)
}
