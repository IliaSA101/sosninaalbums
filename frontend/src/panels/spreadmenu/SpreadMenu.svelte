<script lang="ts">
  import MenuList from '../../base/ui/MenuList.svelte'
  import type { MenuNode } from '../../base/ui/menulist'
  import { spreadMenu } from './spreadmenu'
  import { modClick } from '../timeline/timeline'
  import { isMac } from '../../base/core/platform'
  import { store } from '../../base/core/store.svelte'
  import { isSheet } from '../../base/geom/sheet'
  import { prefs } from '../../base/prefs/prefs.svelte'

  // Меню правого щелчка по группе ленты: название, разрез и склейка (094), виньетка (031) и удаление. Устройство — как у
  // PhotoMenu (108): общий MenuList (↑/↓, отметки слева, Esc и щелчок мимо закрывают), дерево пунктов — spreadmenu.ts.
  // count > 1 — выделено несколько групп.
  // label — свой текст пункта удаления (плитка обложки: «Удалить обложку»); у неё пункта виньетки нет.
  // «Вставить … перед / после» (103) — когда задан onInsert (меню ленты и «Всех разворотов»); unit — «разворот» или «лист».
  let {
    x,
    y,
    count = 1,
    label,
    kind,
    onVignette,
    onNewVignette,
    onNewLook,
    newLook,
    onVary,
    varies = true,
    onRename,
    cut,
    onCut,
    join,
    onJoin,
    onInsert,
    unit = { one: 'разворот' },
    onDelete,
    onclose,
  }: {
    x: number
    y: number
    count?: number
    label?: string
    kind?: string // вид листа под курсором (SpreadView.kind)
    onVignette?: (act: 'make' | 'clear' | 'setup') => void
    onNewVignette?: () => void // плитка «+»: новая виньетка вместо пунктов группы
    onNewLook?: () => void // плитка «+»: разворот макета альбома (131)
    newLook?: string // имя макета для пункта; нет макета (или не найден на этом ПК) — пункта нет
    onVary?: () => void // «Разнообразить раскладки» (041); нет — пункта нет (плитка обложки)
    varies?: boolean // среди целей есть разворот «Авто» с фото (anyVaries); нет — пункт серый
    onRename?: () => void // «Название…»; нет — пункта нет
    cut?: { show: boolean; ok: boolean; why: string } // разрез перед фото под курсором (cutState)
    onCut?: () => void
    join?: { ok: boolean; why: string } // склейка со следующей (joinState); у последней группы нет
    onJoin?: () => void
    onInsert?: (where: 'before' | 'after') => void
    unit?: { one: string }
    onDelete?: () => void
    onclose: () => void
  } = $props()

  const items = $derived(
    spreadMenu({
      count,
      kind,
      sheet: isSheet(store.view.settings),
      unit,
      can: (f) => prefs.can(f),
      rename: !!onRename,
      cut: onCut ? cut : undefined,
      join: onJoin ? join : undefined,
      insert: !!onInsert,
      vignette: !!onVignette,
      newVignette: !!onNewVignette,
      newLook: onNewLook ? newLook : undefined,
      vary: !!onVary,
      varies,
      del: !!onDelete,
      label,
      // Способ мышью справа в пункте — как клавиша в строке меню.
      how: modClick(isMac()),
    }),
  )

  // Действие по id пункта; меню закрывает вызывающий (как у PhotoMenu), здесь только вызов.
  const acts: Record<string, (arg?: string) => void> = {
    rename: () => onRename?.(),
    cut: () => onCut?.(),
    join: () => onJoin?.(),
    insert: (arg) => onInsert?.(arg as 'before' | 'after'),
    vignette: (arg) => onVignette?.(arg as 'make' | 'clear' | 'setup'),
    newVignette: () => onNewVignette?.(),
    newLook: () => onNewLook?.(),
    vary: () => onVary?.(),
    delete: () => onDelete?.(),
  }
  const pick = (n: MenuNode) => acts[n.id]?.(n.arg)
</script>

<MenuList {items} at={{ x, y }} onpick={pick} {onclose} />
