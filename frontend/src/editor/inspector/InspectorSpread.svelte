<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { NAME_MAX, nameChanged, normName } from '../../panels/timeline/spreadname'
  import InspectorCover from '../../panels/cover/InspectorCover.svelte'
  import { isCover, unitOf } from '../../base/geom/sheet'
  import VignettePanel from '../../panels/vignette/VignettePanel.svelte'
  import { canMakeVignette, isVignette } from '../../panels/vignette/vignette'
  import { clearVignette, makeVignette } from '../../panels/vignette/vigact'
  import { VIG_CLEAR, VIG_SAVE, vigMore } from '../../panels/vignette/vigmenu'
  import { vigOwn } from '../../panels/vignette/vigown.svelte'
  import type { MenuNode } from '../../base/ui/menulist'
  import Icon from '../../base/ui/Icon.svelte'
  import CardGroup from './CardGroup.svelte'
  import SpreadBgGroup from './SpreadBgGroup.svelte'
  import SpreadCapsGroup from './SpreadCapsGroup.svelte'

  // Карточка «Разворот N», когда ничего не выделено (спека 008, panels.md; 009 D2; 124 U5): название, группы
  // «Виньетка» → «Фон» → «Подписи» (один порядок для всех листов, решение владельца 24.10), раскладка «Авто» / «Ручная».
  // «Ручная» закрепляет места, «Авто» на сетке спрашивает (места сбросятся, gridAct.backToAuto).
  // head=false — заголовок рисует сама карточка (docked, cardTitle): сворачиваемая шапка.
  let { head = true }: { head?: boolean } = $props()
  const sp = $derived(store.spread)
  // Виньетка (031, решение 80): вместо «Авто / Ручная» — группа «Виньетка» с шаблонами и числами.
  const vsp = $derived(isVignette(sp) ? sp : null)
  const unit = $derived(unitOf(store.view.settings))
  // Обложка (032): своя карточка сверху, названия и «Авто / Ручная» нет.
  const cover = $derived(isCover(sp))

  // Enter или уход из поля — в Go; Esc возвращает прежнее название и не снимает выделение.
  function commit(el: HTMLInputElement) {
    if (!sp) return
    if (nameChanged(sp.name, el.value)) store.renameSpread(sp.id, normName(el.value))
    else el.value = sp.name ?? ''
  }
  function key(e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      el.value = sp?.name ?? ''
      el.blur()
    }
  }
  // «Сделать виньеткой» (отзыв 14.10): видимый путь вместо одного правого щелчка в ленте; тост с «Вернуть» — в makeVignette.
  const vigable = $derived(canMakeVignette(sp, prefs.can))
  let making = $state(false)
  async function toVignette() {
    if (!sp || making) return
    making = true
    try {
      await makeVignette(sp)
    } finally {
      making = false
    }
  }

  // «⋯» группы «Виньетка»: «Сохранить как шаблон…» и «Обычный разворот» (были кнопками панели).
  let vigSum = $state('')
  const vigItems = $derived(vigMore(unit.one, prefs.can('templates')))
  function vigPick(n: MenuNode) {
    if (!sp) return
    if (n.id === VIG_SAVE) vigOwn.save(sp.id)
    else if (n.id === VIG_CLEAR) void clearVignette(sp.id)
  }
</script>

{#if sp}
  {#if head}<div class="head">{cover ? 'Обложка' : `${unit.cap} ${store.current + 1}`}</div>{/if}
  <!-- Обложка (124 U6): все её группы рисует InspectorCover -->
  {#if cover}<InspectorCover />{:else}
    {#if prefs.can('names')}
      <input
        class="name"
        data-guide="spread-title"
        type="text"
        maxlength={NAME_MAX}
        placeholder="Название"
        aria-label="Название {unit.one}а"
        value={sp.name}
        onchange={(e) => commit(e.currentTarget)}
        onkeydown={key}
      />
    {/if}
    {#if vsp}
      <CardGroup id="spread.vig" title="Виньетка" summary={vigSum} more={vigItems} onmore={vigPick} moreGuide="vig-save">
        <!-- Форма берётся при создании панели: для другого листа — заново -->
        {#key vsp.id}<VignettePanel sp={vsp} onsummary={(t) => (vigSum = t)} />{/key}
      </CardGroup>
    {/if}
    <!-- Группа «Фон» живёт, пока есть цвет или фото на фоне (спека 011); без обоих — не рисуется (groupShown) -->
    <SpreadBgGroup {sp} />
    <!-- Подписи листа (030): общий стиль и шаблон «к каждому фото» -->
    <SpreadCapsGroup {sp} />
    {#if prefs.can('manual') && !vsp}
      <div class="seg" role="group" aria-label="Раскладка {unit.one}а" data-guide="mode-toggle">
        <button
          class:on={!sp.grid}
          aria-pressed={!sp.grid}
          onclick={() => sp.grid && gridAct.backToAuto(sp.id)}
          title="Фото раскладываются автоматически">Авто</button
        >
        <button
          class:on={sp.grid}
          aria-pressed={sp.grid}
          onclick={() => !sp.grid && gridAct.setGrid(true)}
          title="Сделать ручной: раскладка перестаёт меняться сама, места можно править">Ручная</button
        >
      </div>
    {/if}
    {#if vigable}
      <button
        class="ghost vig"
        data-guide="vig-make"
        disabled={making}
        title="Учителя и ученики с подписями, шапка с полями; фото {unit.of} садятся в места учеников"
        onclick={toVignette}><Icon name="vignette" size={14} />Сделать виньеткой</button
      >
    {/if}
  {/if}
{/if}

<style>
  .head {
    font-size: 13px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .name {
    width: 100%;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
  }
  .ghost {
    width: 100%;
    height: 24px;
    margin-top: 8px;
    padding: 0 6px;
    font-size: 12px;
  }
  .vig {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    width: 100%;
    height: 26px;
    margin-top: 10px;
    font-size: 12px;
    border-color: var(--line);
  }
  .seg {
    display: flex;
    margin-top: 10px;
    border-radius: 5px;
    overflow: hidden;
  }
  .seg button {
    flex: 1;
    height: 26px;
    padding: 0 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button.on {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
