<script module lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { unitOf } from '../../base/geom/sheet'

  // «Разворот 7» / «Лист 3»: общее имя разворота в заголовке всех трёх вкладок галереи.
  export function galleryName(spreadID: string): string {
    const n = store.view.spreads.findIndex((x) => x.id === spreadID) + 1
    return `${unitOf(store.view.settings).cap} ${n}`
  }

  // Вкладки «Варианты | Шаблоны» — только когда доступны обе (у виньетки их решает вызывающий).
  export const galleryTabs = (): boolean => prefs.can('templates') && prefs.can('variants')
</script>

<script lang="ts">
  import type { Snippet } from 'svelte'
  import { gallery } from './gallery.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import type { GalleryTab } from '../../base/core/types'

  // Шапка галереи в одну строку (050): вкладки · фильтры вкладки · заголовок · «Сохранить как шаблон…» · ×.
  // Одна на все вкладки: меняются только filters, title и save.
  let {
    tabs,
    title,
    filters,
    save,
  }: {
    tabs: boolean
    title: string
    filters?: Snippet
    save?: { title: string; onclick: () => void }
  } = $props()

  const TABS: { id: GalleryTab; label: string }[] = [
    { id: 'layouts', label: 'Варианты' },
    { id: 'grids', label: 'Шаблоны' },
  ]
  // Пока галерея уезжает после закрытия, gallery.cur уже null — держим последнюю вкладку.
  let last = $state<GalleryTab>(gallery.cur?.tab ?? 'layouts')
  $effect.pre(() => {
    if (gallery.cur) last = gallery.cur.tab
  })
  // На сетке выбор раскладки спросит «Вернуть авто?»
  const pick = (id: GalleryTab) => id !== last && gallery.open('spread', id)
</script>

<header>
  {#if tabs}
    <Seg tabs items={TABS} value={last} onpick={pick} label="Что показать" />
    <span class="bar"></span>
  {/if}
  {#if filters}{@render filters()}{/if}
  <span class="title" {title}>{title}</span>
  {#if save}
    <button type="button" class="save" title={save.title} onclick={save.onclick}>
      <span class="long">Сохранить как шаблон…</span><span class="short">Сохранить…</span>
    </button>
  {/if}
  <button type="button" class="close" onclick={() => gallery.close()} title="Закрыть (Esc)" aria-label="Закрыть галерею раскладок"
    ><Icon name="x" /></button
  >
</header>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 32px;
    font-size: 13px;
    white-space: nowrap;
  }
  .bar {
    flex: none;
    width: 1px;
    height: 20px;
    background: var(--line);
  }
  .title {
    flex: 1;
    min-width: 0;
    text-align: right;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .save {
    flex: none;
    height: 26px;
    padding: 0 10px;
    font-size: 12px;
  }
  .short {
    display: none;
  }
  /* Узкая шапка (окно 1200 px и панель деталей): короткая подпись, полное — в подсказке */
  @container (max-width: 820px) {
    .long {
      display: none;
    }
    .short {
      display: inline;
    }
  }
  .close {
    flex: none;
    width: 28px;
    height: 28px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
  }
</style>
