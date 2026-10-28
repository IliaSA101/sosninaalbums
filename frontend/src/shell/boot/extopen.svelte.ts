// Открытие .salbum извне (112, design §4.3–§4.4): двойной щелчок, второй запуск, Mac OnFileOpen. Go держит в очереди
// последний файл и шлёт `open:external` (имя без пути); открывает его только OpenExternal — путь от окна не принимается.
// Порядок как у «Файл ▸ Открыть»: ждём набранный текст (edit.idle), прежний альбом Go пишет сам, без вопроса (q112-2).
import { tick } from 'svelte'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { jobs } from '../../base/core/jobs.svelte'
import { Api, store } from '../../base/core/store.svelte'
import { modalStack } from '../../base/modal/modalstack.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { edit } from '../../editor/text/edit.svelte'

class ExtOpen {
  // Имя файла, ждущего открытия; null — ничего не ждёт. Второй файл заменяет первого: в очереди Go тоже побеждает последний.
  name = $state<string | null>(null)
  // Открытия идут по одному: два подряд не должны делить replaceState.
  private chain: Promise<unknown> = Promise.resolve()

  // Подписка на событие Go и слежение за окнами. Событие, пришедшее до подписки, приходит в State.pending (store.init).
  listen(): () => void {
    const off = EventsOn('open:external', (name: string) => name && this.request(name))
    const stop = $effect.root(() => {
      $effect(() => {
        if (this.name !== null && !modalStack.any()) this.open()
      })
    })
    return () => {
      off()
      stop()
    }
  }

  // Файл пришёл. Стартовое окно создано для выбора альбома — закрываем; другое окно держит состояние текущего
  // альбома — ждём, пока его закроют (q112-9).
  request(name: string) {
    this.name = name
    if (prefs.startOpen) prefs.closeStart()
    // Стартовое окно уходит из стека на следующем кадре: без tick оно сошло бы за «чужое» окно.
    void tick().then(() => {
      if (this.name === name && modalStack.any() && !this.onlyProgress())
        store.notify(`Альбом «${name.replace(/\.salbum$/i, '')}» откроется, когда закроете окно`)
    })
  }

  // Открыто лишь окно «Открываем альбом»: закрыть его нельзя, оно уйдёт само — тост «закроете окно» ввёл бы в заблуждение.
  private onlyProgress(): boolean {
    return jobs.openShown && modalStack.count() === 1
  }

  private open() {
    this.name = null
    this.chain = this.chain.then(() => edit.idle()).then(() => store.replaceState(Api.OpenExternal()))
  }
}

export const extOpen = new ExtOpen()
