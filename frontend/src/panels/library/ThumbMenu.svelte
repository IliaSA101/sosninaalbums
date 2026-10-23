<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import MenuList from '../../base/ui/MenuList.svelte'
  import type { MenuNode } from '../../base/ui/menulist'
  import { copyNames, reveal } from '../../editor/photo/fileact'
  import { goUse } from '../../editor/photoprops/gouse.svelte'
  import { lost } from './lost.svelte'
  import { lineAt, USE_ROW } from '../../editor/photoprops/usemenu'
  import { thumbMenu } from './thumbmenu'
  import { thumbmenu } from './thumbmenu.svelte'

  // Меню правой кнопки у миниатюры ленты (спека 106): «Показать в …», «Копировать имя файла», «Где стоит ▸».
  // Вид, подменю сбоку и клавиши — общий MenuList, пункты — как «Файл ▸» меню фото на холсте (fileGroup).
  // Esc, щелчок мимо, размер окна и потеря фокуса закрывают сам список; прокрутку ленты ловит Library, правку — этот $effect.
  const m = $derived(thumbmenu.menu)
  const gone = (id: string) => lost.has(id) || store.missing.includes(id)
  const items = $derived(
    m
      ? thumbMenu(
          {
            photos: m.ids.length,
            missing: m.ids.filter(gone).length,
            used: m.ids.length === 1 ? store.useCount(m.ids[0]) : 0,
            lines: thumbmenu.uses,
          },
          prefs.can,
        )
      : [],
  )

  // Любая правка альбома (rev) или пропавшие пункты — меню закрывается: строки вхождений могли устареть.
  let rev = store.view.rev
  $effect(() => {
    const r = store.view.rev
    if (r !== rev) {
      rev = r
      thumbmenu.close()
    }
  })
  $effect(() => {
    if (m && items.length === 0) thumbmenu.close()
  })

  function pick(n: MenuNode) {
    if (!m) return
    const ids = m.ids
    const line = n.id === USE_ROW ? lineAt(thumbmenu.uses, n.arg) : null
    thumbmenu.close()
    if (n.id === 'explorer') void reveal(ids[0])
    else if (n.id === 'copy') void copyNames(ids, gone)
    else if (line) goUse(line)
  }
</script>

{#if m && items.length > 0}
  <MenuList {items} at={{ x: m.x, y: m.y }} onpick={pick} onclose={() => thumbmenu.close()} />
{/if}
