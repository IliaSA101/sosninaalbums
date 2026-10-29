import { untrack } from 'svelte'

// Одна общая подсказка поверх всего: лента с overflow обрезала бы обычные.
// Рисует её FixedTip.svelte; показывают её через действие use:tipped (гаснет и при удалении элемента).
// side: 'top' — над элементом по центру, 'right' — справа по середине (лента правки слева),
// 'left' — слева по середине (столбец мини-панели у правого края, спека 108).
export type TipSide = 'top' | 'right' | 'left'

class Tip {
  state = $state<{ text: string; x: number; y: number; side: TipSide } | null>(null)
  private owner: HTMLElement | null = null // чья подсказка; не reactive, нужен только destroy действия

  showAt(el: HTMLElement, text: string, side: TipSide = 'top') {
    const r = el.getBoundingClientRect()
    this.owner = el
    const mid = r.top + r.height / 2
    if (side === 'right') this.state = { text, x: r.right, y: mid, side }
    else if (side === 'left') this.state = { text, x: r.left, y: mid, side }
    else this.state = { text, x: r.left + r.width / 2, y: r.top, side }
  }

  hide() {
    this.owner = null
    this.state = null
  }

  // Погасить, только если подсказка принадлежит этому элементу (чужую не трогаем).
  hideFor(el: HTMLElement) {
    if (this.owner === el) this.hide()
  }

  // Заменить текст у уже показанной подсказки элемента (пустой — погасить).
  retext(el: HTMLElement, text: string) {
    if (this.owner !== el || !this.state) return
    if (text) this.state = { ...this.state, text }
    else this.hide()
  }
}

export const tip = new Tip()

export type TipArg = { text: string; side?: TipSide }

// use:tipped={{ text, side }} — показ и скрытие по наведению. Удалённый из DOM узел pointerleave
// не получает (панель исчезает под курсором), поэтому destroy гасит подсказку, если она его.
// Пустой text — подсказки нет (условные подсказки: меню открыто, склейка недоступна).
export function tipped(node: HTMLElement, arg: TipArg) {
  let cur = arg
  const enter = () => {
    if (cur.text) tip.showAt(node, cur.text, cur.side)
  }
  const leave = () => tip.hideFor(node)
  node.addEventListener('pointerenter', enter)
  node.addEventListener('pointerleave', leave)
  return {
    update(next: TipArg) {
      const same = next.text === cur.text && next.side === cur.side
      cur = next
      // Svelte зовёт update внутри render_effect: retext читает и пишет tip.state, без untrack — цикл.
      // Тот же текст — не трогаем вовсе: объект state остаётся прежним.
      if (!same) untrack(() => tip.retext(node, next.text))
    },
    destroy() {
      node.removeEventListener('pointerenter', enter)
      node.removeEventListener('pointerleave', leave)
      tip.hideFor(node)
    },
  }
}
