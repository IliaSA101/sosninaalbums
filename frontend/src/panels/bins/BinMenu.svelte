<script lang="ts">
  import { onDestroy } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { drag } from '../../base/drag/drag.svelte'
  import { modes } from '../../base/prefs/view.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import Popover from '../../base/ui/Popover.svelte'
  import { gallery } from '../gallery/gallery.svelte'
  import { remove, setFor } from './binact'
  import { BinMenuState, NEW } from './binstate.svelte'
  import BinName from './BinName.svelte'
  import BinRow from './BinRow.svelte'
  import { binAction, binButton, binButtonTip, binCounts, binsOf, dropLabel, freeName, newText } from './bins'
  import { binsel } from './binsel.svelte'

  // Подборки фото одной кнопкой (083), образец — «Папки» (FolderMenu, 054): число и ▾; показана подборка —
  // жёлтая кнопка «Имя 24 / 30 ▾» и × «Показать все фото». Окно остаётся открытым при показе и правках.
  const bins = $derived(binsOf(store.view))
  const shown = $derived(bins.find((b) => b.id === binsel.shown))
  // Числа показанной подборки на кнопке: один проход по ленте, только пока подборка показана.
  const shownN = $derived(
    shown ? { name: shown.name, n: binCounts([shown], store.photos, binsel.index, store.used, store.backdropUsed).get(shown.id)! } : null,
  )
  // Брошенные на кнопку фото (целевой нет) важнее выделения: «+ N» и «Новая» строятся для них.
  const ids = $derived(binsel.pending.length > 0 ? binsel.pending : store.selectedIds)
  const dropped = $derived(binsel.pending.length > 0)
  // Во время переноса фото из ленты кнопка называет, куда оно попадёт (083-19).
  const over = $derived(drag.over === 'bin' && drag.payload?.kind === 'photos')
  const targetName = $derived(binsel.targetName)

  // Окно открывают и снаружи (B без подборок, Shift+B, бросок): флаг живёт в binsel.
  let btn = $state<HTMLButtonElement>()
  // Какая строка в режиме имени и текст ошибки под полем.
  const menu = new BinMenuState()

  // Кнопка ушла в «⋯» или «⋯» закрыли: окно без кнопки не остаётся висеть флагом до следующего открытия.
  onDestroy(() => binsel.close())

  // Лента свёрнута (крупный просмотр, галерея) — окну не к чему быть привязанным.
  $effect(() => {
    if (modes.big || gallery.cur) binsel.open = false
  })
  $effect(() => {
    if (!binsel.open) {
      menu.stop()
      binsel.pending = []
      binsel.wantNew = false
    }
  })
  // Shift+B: окно открыто, показываем строку новой подборки и снимаем флаг.
  $effect(() => {
    if (binsel.open && binsel.wantNew) {
      binsel.wantNew = false
      menu.startEdit(NEW)
    }
  })

  // Бросок на строку или «Новая» закончил дело — окно закрывается; обычное выделение — окно остаётся (как «Папки»).
  async function setRow(id: string) {
    const was = dropped
    const ok = await setFor(id, ids, binAction(ids, id, binsel.index) === 'add')
    if (was && ok) binsel.open = false
  }
  async function submitNew(name: string) {
    const was = dropped
    await menu.submitNew(name, [...ids])
    if (was && menu.editing === '') binsel.open = false
  }

  const stop = (e: PointerEvent) => e.stopPropagation()
</script>

<span class="bins" class:over data-drop="bin">
  <button
    bind:this={btn}
    type="button"
    class="fbtn"
    class:active={!!shownN}
    data-guide="bins"
    title={binButtonTip(shownN)}
    aria-haspopup="dialog"
    aria-expanded={binsel.open}
    onclick={() => (binsel.open = !binsel.open)}
  >
    {over ? dropLabel(targetName) : binButton(bins.length, shownN)}
    <Icon name="caret-down" size={12} />
  </button>
  {#if shownN}
    <button class="ghost reset" type="button" title="Показать все фото" aria-label="Показать все фото" onclick={() => binsel.showAll()}
      ><Icon name="x" size={12} /></button
    >
  {/if}
</span>

<Popover anchor={btn} bind:open={binsel.open} label="Подборки фото" kind="bins">
  <!-- Числа считаются только при открытом окне (Popover монтирует содержимое открытым) -->
  {@const counts = binCounts(bins, store.photos, binsel.index, store.used, store.backdropUsed)}
  <div class="box">
    <ul class:scroll={bins.length > 10}>
      <li class="all" class:cur={!binsel.shown}>
        <span class="dot" aria-hidden="true">{binsel.shown ? '' : '●'}</span>
        <button type="button" class="name" aria-current={!binsel.shown} onpointerdown={stop} onclick={() => binsel.showAll()}
          >Все фото</button
        >
        <span class="n">{store.photos.length}</span>
      </li>
      {#each bins as b (b.id)}
        <BinRow
          bin={b}
          n={counts.get(b.id) ?? { all: 0, inAlbum: 0 }}
          current={b.id === binsel.shown}
          target={b.id === binsel.targetId()}
          sel={ids.length}
          action={binAction(ids, b.id, binsel.index)}
          editing={menu.editing === b.id}
          err={menu.editing === b.id ? menu.err : ''}
          onshow={() => binsel.show(b.id)}
          onset={() => setRow(b.id)}
          onedit={() => menu.startEdit(b.id)}
          onrename={(name) => menu.submitRename(b.id, name)}
          oncancel={() => menu.stop()}
          onremove={() => remove(b.id)}
        />
      {/each}
      {#if menu.editing === NEW}
        <li class="new">
          <span class="dot"></span>
          <BinName value={freeName(bins)} label="Имя новой подборки" err={menu.err} onsubmit={submitNew} oncancel={() => menu.stop()} />
        </li>
      {/if}
    </ul>
    <button type="button" class="add" onpointerdown={stop} onclick={() => menu.startEdit(NEW)}>
      <Icon name="plus" size={14} />
      {newText(ids.length, dropped)}
    </button>
  </div>
</Popover>

<style>
  .bins {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .bins.over .fbtn {
    outline: 2px dashed var(--accent);
    outline-offset: 2px;
  }
  .fbtn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .fbtn.active {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .fbtn.active:hover:not(:disabled) {
    background: #ffe53a;
  }
  .reset {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 6px;
    width: 320px;
  }
  .box > button {
    height: 26px;
    font-size: 12px;
    padding: 0 8px;
  }
  .add {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    justify-content: flex-start;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
  }
  ul.scroll {
    max-height: 260px;
    overflow-y: auto;
  }
  li.all,
  li.new {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 0 2px 4px;
  }
  li.all.cur {
    background: rgba(255, 255, 255, 0.05);
  }
  .dot {
    width: 10px;
    color: var(--accent);
    font-size: 10px;
  }
  .all .name {
    flex: 1;
    min-width: 0;
    text-align: left;
    background: transparent;
    height: 22px;
    font-size: 12px;
    padding: 0 4px;
  }
  .all .n {
    color: var(--muted);
    padding-right: 4px;
  }
</style>
