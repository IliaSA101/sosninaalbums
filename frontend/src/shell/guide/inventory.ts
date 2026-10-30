// Опись функций программы (спека 140, design §2): одна на стражи полноты справочника (136) и на число в шапке F1 и в
// «О программе». Функция — то, что вызывают одним действием интерфейса: команда, действие меню фото без своей команды
// или жест. Окна отдельно не считаются: их открывает команда. Повторы схлопываются по id.
import { keymap } from '../../base/keys/keymap.svelte'
import { plural } from '../../base/words/plural'
import { ACTS, type ActId } from '../../editor/photomenu/photoacts'
import type { Menu, MenuItem } from '../menu/menu'
import { MOUSE_HELP } from '../menu/mousehelp'
import { menus } from './show'

// Действия-подменю: сами ничего не делают, а раскрывают список («Ещё», «Слой», «Целиком / с обрезкой»).
export const ACT_GROUPS: readonly ActId[] = ['more', 'layer', 'fit']

export interface Inventory {
  commands: Set<string> // id команд: пункты меню всех уровней и каталог клавиш
  acts: Set<ActId> // действия меню фото без своей команды, без групп
  gestures: string[] // жесты и местные клавиши: MOUSE_HELP.show без повторов
}

// Команды пунктов меню на всех уровнях вложенности.
function menuCommands(list: readonly MenuItem[], out: Set<string>): void {
  for (const it of list) {
    if (it.cmd) out.add(it.cmd)
    if (it.sub) menuCommands(it.sub, out)
  }
}

export function inventory(menu: readonly Menu[], commands: readonly { id: string }[]): Inventory {
  const cmds = new Set<string>()
  for (const m of menu) menuCommands(m.items, cmds)
  for (const c of commands) cmds.add(c.id)
  const acts = new Set<ActId>()
  for (const id of Object.keys(ACTS) as ActId[]) if (!ACTS[id].cmd && !ACT_GROUPS.includes(id)) acts.add(id)
  return { commands: cmds, acts, gestures: [...new Set(MOUSE_HELP.map((r) => r.show))] }
}

export const functionCount = (i: Inventory): number => i.commands.size + i.acts.size + i.gestures.length

// Число функций программы сейчас: полное меню (контекст FULL в show.ts) и каталог клавиш из Go.
export const guideCount = (): number => functionCount(inventory(menus(), keymap.view.commands))

// Слова про число функций в одном месте: шапка F1 и «О программе» читают их отсюда.
export const functionsText = (n: number): string => `${n} ${plural(n, ['функция', 'функции', 'функций'])}`
export const ABOUT_FUNCTIONS_TAIL = ' — описаны в '
export const ABOUT_FUNCTIONS_LINK = 'справочнике F1' // падеж по фразе; спека: «справочник F1»
