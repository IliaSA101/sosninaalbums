// «Свои цвета» (109-4): один список ≤ 10 в config.json у Go, общий для всех альбомов. Правила списка (новые первыми,
// повторы, мусор) — у Go; здесь только копия для окна. Это данные программы, не альбома: отмены нет.
import * as Api from '../../../wailsjs/go/app/App'
import { dropLegacyRecent, legacyRecent } from './palette'

class MyColors {
  list = $state<string[]>([])
  private loading: Promise<void> | null = null

  // Один раз за запуск. Файл пуст, а в localStorage есть прежние «Последние» — разовый перенос: Go пишет их, только
  // если в файле всё ещё пусто (второе окно не задвоит), ключ стирается после ответа.
  load(): Promise<void> {
    this.loading ??= this.fetch().catch(() => {
      this.loading = null // Go не ответил — следующее открытие окна попробует снова
    })
    return this.loading
  }

  private async fetch() {
    let list = await Api.MyColors()
    if (list.length === 0) {
      const old = legacyRecent()
      if (old.length > 0) list = await Api.ImportColors(old)
      dropLegacyRecent()
    }
    this.list = list
  }

  // Выбранный цвет и «+» — одно и то же: наверх списка.
  async remember(hex: string): Promise<void> {
    this.list = await Api.RememberColor(hex).catch(() => this.list)
  }

  add(hex: string): Promise<void> {
    return this.remember(hex)
  }

  async forget(hex: string): Promise<void> {
    this.list = await Api.ForgetColor(hex).catch(() => this.list)
  }

  // Главные цвета фото листа (≤ 8) для вкладки «С фото»; ошибка уходит окну.
  photoColors(sheet: string): Promise<string[]> {
    return Api.SpreadPhotoColors(sheet)
  }
}

export const mycolors = new MyColors()
