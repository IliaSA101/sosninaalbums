import { SvelteSet } from 'svelte/reactivity'

// Сколько микрозадач ждём привязки корня (bind:this), прежде чем сдаться и показывать всё.
const TRIES = 5

// «Рядом с видимой частью» (073, U2–U3): тяжёлое содержимое плитки монтируется, только когда плитка
// близко к окну. Один IntersectionObserver на список; id видимых — в реактивном множестве.
export class Nearby {
  #set = new SvelteSet<string>()
  #ids = new WeakMap<Element, string>()
  #io: IntersectionObserver | undefined
  // Без IntersectionObserver (тесты, старый движок) или без корня рядом всё — как до 073.
  #all = $state(typeof IntersectionObserver === 'undefined')
  readonly #root: (() => Element | null) | null
  readonly #margin: string

  // root — прокручиваемый корень (null — окно); margin — запас вокруг корня, как rootMargin.
  // Корень берётся при первом watch, а не в конструкторе: к тому моменту узел уже смонтирован.
  constructor(root: (() => Element | null) | null, margin: string) {
    this.#root = root
    this.#margin = margin
  }

  has(id: string): boolean {
    return this.#all || this.#set.has(id)
  }

  // Для `{@attach (n) => near.watch(n, id)}`; возвращает снятие.
  watch(node: Element, id: string): () => void {
    if (typeof IntersectionObserver === 'undefined') return () => {}
    this.#ids.set(node, id)
    let off = false
    const start = (left: number) => {
      if (off) return
      const io = this.#make()
      if (io) io.observe(node)
      else if (left > 0) queueMicrotask(() => start(left - 1))
      else this.#all = true // корня нет — не прячем ничего, лучше медленно, чем пусто
    }
    start(TRIES)
    return () => {
      off = true
      this.#io?.unobserve(node)
      this.#ids.delete(node)
      this.#set.delete(id)
    }
  }

  #make(): IntersectionObserver | undefined {
    if (this.#io) return this.#io
    const root = this.#root ? this.#root() : null
    if (this.#root && !root) return undefined
    this.#io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          const id = this.#ids.get(e.target)
          if (id === undefined) continue
          if (e.isIntersecting) this.#set.add(id)
          else this.#set.delete(id)
        }
      },
      { root, rootMargin: this.#margin },
    )
    return this.#io
  }
}
