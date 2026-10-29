// Отметки «Изучил» справочника F1 (спека 140, design §1): состояние окна поверх config.json. Правда — файл у Go:
// набор меняется только по ответу MarkGuide. Альбом не трогается (ни правки, ни undo).
import { SvelteSet } from 'svelte/reactivity'
import { Api, store } from '../../base/core/store.svelte'
import { errText, capFirst } from '../../base/words/errtext'
import { ARTICLES } from '../guidetext/articles'
import { RENAMED } from '../guidetext/renamed'
import { learnedOf } from './guidelist'

// Прежние id статьи: ключи RENAMED, указывающие на неё.
export const oldIds = (id: string, renamed: Readonly<Record<string, string>> = RENAMED): string[] =>
  Object.keys(renamed).filter((k) => renamed[k] === id)

class Learned {
  // Изученные статьи — текущие id (отметки прежних id уже переведены через RENAMED).
  readonly ids = new SvelteSet<string>()
  private seq = 0

  has = (id: string): boolean => this.ids.has(id)

  private take(raw: unknown) {
    const list = Array.isArray(raw) ? (raw as string[]) : []
    const next = learnedOf(list, RENAMED, new Set(ARTICLES.map((a) => a.id)))
    for (const id of [...this.ids]) if (!next.has(id)) this.ids.delete(id)
    for (const id of next) this.ids.add(id)
  }

  // При открытии F1: одно чтение файла (второй экземпляр окна видит чужую правку).
  async load() {
    const my = ++this.seq
    try {
      const raw = await Api.GuideLearned()
      if (my === this.seq) this.take(raw)
    } catch {
      // нет связи с Go — остаётся прежнее; отметки не критичны для работы справочника
    }
  }

  // Флажок «Изучил»: одна запись на статью и её прежние id; набор — по ответу.
  async set(id: string, on: boolean) {
    const my = ++this.seq
    try {
      const raw = await Api.MarkGuide([id, ...oldIds(id)], on)
      if (my === this.seq) this.take(raw)
    } catch (e) {
      store.notify(capFirst(errText(e)), 'error')
    }
  }
}

export const learned = new Learned()
