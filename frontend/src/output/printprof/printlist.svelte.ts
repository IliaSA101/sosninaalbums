import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { Api } from '../../base/core/store.svelte'
import type { PrintInfo } from '../../base/core/types'
import { coverAct } from '../../panels/cover/coveract.svelte'
import { listLabel, planLabel } from './printwords'
import type { PrintPlan } from '../../base/core/types'

// Типографии реестра для списков (123 Б3): «Типография» в настройках альбома и в карточке обложки, «Печать» в окне
// «Экспорт», «Основа» в окне «Своя типография». Реестр меняется событием `print:changed` (сохранена, удалена, добавлена
// файлом своя; набор библиотеки) — тогда список, кнопки форматов и пресеты обложки читаются заново.
class PrintList {
  infos = $state<PrintInfo[]>([])
  // Растёт на каждую смену реестра: кто держит свою копию PagePresets, перечитывает её по этому числу.
  rev = $state(0)
  // Растёт, когда окно сохранило типографию и поставило её альбому: список «Типография» встаёт на неё.
  saves = $state(0)
  private req = 0

  async load() {
    const my = ++this.req
    try {
      const list = ((await Api.PrintProfiles()) ?? []) as unknown as PrintInfo[]
      if (my === this.req) this.infos = list
    } catch {
      // список — удобство: без него названия показываются без пометки
    }
  }

  // print:changed — реестр другой.
  async changed() {
    this.rev++
    await Promise.all([this.load(), coverAct.reloadPresets()])
  }

  // Подписка на события Go; снимается возвратом.
  watch(): () => void {
    void this.load()
    const off = EventsOn('print:changed', () => void this.changed())
    return () => off?.()
  }

  label(name: string): string {
    return listLabel(name, this.infos)
  }

  planLabel(p: Pick<PrintPlan, 'kind' | 'title'>): string {
    return planLabel(p, this.infos)
  }

  // Типография, чьё название стоит в списке «Типография» (группа пресета = название профиля).
  byName(name: string): PrintInfo | undefined {
    return this.infos.find((i) => i.name === name)
  }
}

export const printList = new PrintList()
