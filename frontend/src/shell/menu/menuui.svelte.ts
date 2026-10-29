// Открытое меню строки меню (спека 033): состояние вынесено из MenuBar, чтобы справочник мог раскрыть
// меню извне, как щелчком, и закрыть его, когда подсветка кончилась.
import type { MenuId } from './menu'

class MenuUi {
  // Какое из меню «Файл · Правка · Разворот · Вид · Справка» раскрыто; null — ни одно.
  openId = $state<MenuId | null>(null)
  // Запрос раскрыть меню: счётчик, чтобы повторный запрос того же меню тоже сработал. Берёт MenuBar.
  req = $state<{ id: MenuId; n: number } | null>(null)

  ask(id: MenuId) {
    this.req = { id, n: (this.req?.n ?? 0) + 1 }
  }

  close() {
    this.openId = null
  }
}

export const menuUi = new MenuUi()
