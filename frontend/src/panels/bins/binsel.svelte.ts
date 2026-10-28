// Какая подборка показана в ленте и какая «целевая» (083). Как folderfilter — настройка окна: в .salbum и undo
// не попадает, при «Новый»/«Открыть» сбрасывается, а удалённая подборка забывается (prune).
// Ленту режет store.inBin (хук bin → filterBin).
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import type { Bin } from '../../base/core/types'
import { ownMarks } from '../marks/ownmarks.svelte'
import { binIndex, binsOf, type BinHost } from './bins'

class BinSel {
  // Показанная подборка; '' — все фото.
  cur = $state('')
  // Целевая для B и броска: последняя, куда добавляли или которую создали.
  target = $state('')
  // ID брошенных на кнопку фото, когда целевой подборки нет: окно строит «+ N» и «Новая» для них, а не для выделения.
  // Закрытие окна чистит (BinMenu).
  pending = $state<string[]>([])
  // Окно «Подборки» открыто: клавиши (B без подборок, Shift+B, бросок) открывают его снаружи.
  open = $state(false)
  // Окно открыто клавишей Shift+B: сразу строка новой подборки (BinMenu снимает флаг, когда показал её).
  wantNew = $state(false)
  // Где сейчас кнопка «Подборки» (пишет LibraryBar): тост «подборок нет» называет путь через «⋯».
  host = $state<BinHost>('bar')
  // Членство по своим отметкам: пересчёт только при смене карты (а не на каждый вызов хука).
  index = $derived(binIndex(ownMarks.map))

  // ID подборки → имя (подсказка миниатюры).
  names = $derived(new Map(binsOf(store.view).map((b) => [b.id, b.name])))

  // Имена подборок фото по его членству; на «Простом» подборок нет.
  namesOf(ids: readonly string[] | undefined): string[] {
    if (!ids || ids.length === 0 || !prefs.can('bins')) return []
    return ids.flatMap((id) => this.names.get(id) ?? [])
  }

  // Показанная с учётом уровня: на «Простом» подборок нет, лента не режется, подборки в файле целы.
  get shown(): string {
    return prefs.can('bins') ? this.cur : ''
  }

  active = $derived(this.shown !== '')

  // Показанная — целевая по умолчанию (spec п. 6).
  targetId(): string {
    return this.cur || this.target
  }

  // Имя целевой для подписи броска; нет целевой — ''.
  get targetName(): string {
    return this.names.get(this.targetId()) ?? ''
  }

  show(id: string) {
    this.cur = id
  }

  showAll() {
    this.cur = ''
  }

  // Имя показанной подборки; нет такой — ''.
  curName(bins: readonly Bin[]): string {
    return bins.find((b) => b.id === this.shown)?.name ?? ''
  }

  // Список подборок изменился: удалённая (в том числе отменой создания) больше не показана и не целевая.
  prune(bins: readonly Bin[]) {
    if (this.cur === '' && this.target === '') return
    const have = new Set(bins.map((b) => b.id))
    if (this.cur !== '' && !have.has(this.cur)) this.cur = ''
    if (this.target !== '' && !have.has(this.target)) this.target = ''
  }

  // Закрыть окно и забыть всё, что открыли вместе с ним (брошенные фото, «новая строка»).
  close() {
    this.open = false
    this.pending = []
    this.wantNew = false
  }

  reset() {
    this.cur = ''
    this.target = ''
    this.close()
  }
}

export const binsel = new BinSel()
