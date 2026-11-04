import { untrack } from 'svelte'
import { SvelteMap, SvelteSet } from 'svelte/reactivity'
import { store } from '../../base/core/store.svelte'
import { autocheck } from '../folders/autocheck'

// Плитки ленты, чья миниатюра не загрузилась (088): файл пропал до проверки папок. Состояние только UI —
// Go узнаёт о пропаже при проверке и сам убирает фото из ленты. Нарисовать «нет файла» вместо битой картинки
// браузера — работа интерфейса (decisions 088-5).
class Lost {
  private ids = new SvelteSet<string>()
  // Метка повторной загрузки в адресе миниатюры: растёт с каждым повтором, чтобы сбить кэш неудачного запроса.
  private tries = new SvelteMap<string, number>()
  // Плитки, которым повтор уже дали с последнего нового списка ленты: вторая ошибка — заглушка без нового круга.
  private spent = new SvelteSet<string>()
  // ID, у которых в прошлый раз была причина «не читается» (store.unread) — чтобы заметить её исчезновение.
  private unreadPrev = new Set<string>()

  constructor() {
    // Новый список ленты — новая попытка загрузки: заглушки сбрасываются (вернувшийся файл снова виден).
    $effect.root(() => {
      $effect(() => {
        void store.photos
        this.ids.clear()
        this.spent.clear()
      })
      // 117: причина «не читается» у фото исчезла (файл перезаписан) — миниатюру просим заново, как после новой сверки.
      $effect(() => {
        const now = new Set(Object.keys(store.unread))
        untrack(() => {
          for (const id of this.unreadPrev) {
            if (now.has(id)) continue
            this.spent.delete(id) // причина снята — новая ошибка снова вправе просить сверку
            this.again(id)
          }
        })
        this.unreadPrev = now
      })
    })
    autocheck.onDone(this.retry)
  }

  has = (id: string): boolean => this.ids.has(id)

  // Метка повтора для адреса миниатюры (0 — первая загрузка).
  retryOf = (id: string): number => this.tries.get(id) ?? 0

  // Ошибка загрузки: запомнить плитку и попросить сверку (одну на пачку ошибок — autocheck склеивает).
  // Ошибка повтора сверку не просит: иначе круг «сверка → повтор → ошибка» не кончится.
  // Ошибка плитки, которой нет в текущей ленте (снятая или от прежнего проекта), — не наша: сверку не просим.
  mark = (id: string) => {
    if (!store.byId.has(id)) return
    this.ids.add(id)
    if (!this.spent.has(id)) autocheck.soon()
  }

  // После сверки: плитка осталась серой, а Go файл пропавшим не считает (его нет в store.missing) — разовый
  // повтор загрузки. Реально пропавшие Go убирает из ленты сам, их повтор не нужен.
  retry = () => {
    for (const id of [...this.ids]) {
      if (this.spent.has(id) || store.missing.includes(id)) continue
      this.spent.add(id)
      this.again(id)
    }
  }

  // Новый адрес миниатюры (сбивает кэш прошлого сбоя) и снятая заглушка.
  private again(id: string) {
    this.tries.set(id, this.retryOf(id) + 1)
    this.ids.delete(id)
  }

  clear = () => {
    this.ids.clear()
    this.spent.clear()
  }
}

export const lost = new Lost()
