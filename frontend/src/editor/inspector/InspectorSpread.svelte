<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { NAME_MAX, nameChanged, normName } from '../../panels/timeline/spreadname'
  import ColorPicker from '../../base/ui/ColorPicker.svelte'
  import InspectorBackdrop from '../backdrop/InspectorBackdrop.svelte'

  // Карточка «Разворот N», когда ничего не выделено (спека 008, panels.md; 009 D2): название, раздел
  // «Фон» (цвет на весь разворот или по страницам), раскладка «Авто» / «Сетка». «Сетка» закрепляет
  // места, «Авто» на сетке спрашивает (места сбросятся, gridAct.backToAuto).
  const sp = $derived(store.spread)
  const sides = [
    { scope: 'left', label: 'Левая', i: 0 },
    { scope: 'right', label: 'Правая', i: 1 },
  ] as const

  // «По страницам» раскрыто вручную; если цвета страниц уже разные, раскрыто всегда (один образец
  // соврал бы). Смена разворота сворачивает.
  let splitOpen = $state(false)
  let splitFor = ''
  $effect(() => {
    if (sp && sp.id !== splitFor) {
      splitFor = sp.id
      splitOpen = false
    }
  })
  const differ = $derived(!!sp && sp.bg[0] !== sp.bg[1])
  const split = $derived(splitOpen || differ)
  const own = $derived(!!sp && (sp.bgSet[0] !== '' || sp.bgSet[1] !== ''))

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
  // ColorPicker зовёт onpick один раз на выбор — один шаг undo. Цвет '' снимает свой цвет.
  const pick = (scope: 'spread' | 'left' | 'right', color: string) => sp && store.run(Api.SetBackground(sp.id, scope, color))
</script>

{#if sp}
  <div class="head">Разворот {store.current + 1}</div>
  {#if prefs.can('names')}
    <input
      class="name"
      type="text"
      maxlength={NAME_MAX}
      placeholder="Название"
      aria-label="Название разворота"
      value={sp.name}
      onchange={(e) => commit(e.currentTarget)}
      onkeydown={key}
    />
  {/if}
  <!-- Раздел «Фон» живёт, пока есть цвет или фото на фоне (спека 011); без обоих — не рисуется -->
  {#if prefs.can('bgColor') || prefs.can('bgPhoto')}
    <div class="bg" role="group" aria-label="Фон">
      <div class="cap">Фон</div>
      {#if prefs.can('bgColor')}
        {#if !split}
          <div class="row">
            <span class="lbl">Весь разворот</span>
            <ColorPicker value={sp.bg[0]} label="Цвет фона разворота" onpick={(c) => pick('spread', c)} />
          </div>
        {/if}
        <button class="link" aria-expanded={split} disabled={differ} onclick={() => (splitOpen = !splitOpen)}>
          <svg class="chev" class:open={split} viewBox="0 0 12 12" aria-hidden="true"><path d="M4 2.5 7.5 6 4 9.5" /></svg>
          По страницам
        </button>
        {#if split}
          {#each sides as s (s.scope)}
            <div class="row">
              <span class="lbl">{s.label}</span>
              <ColorPicker value={sp.bg[s.i]} label="Цвет фона: {s.label.toLowerCase()} страница" onpick={(c) => pick(s.scope, c)} />
              <button
                class="reset"
                disabled={sp.bgSet[s.i] === ''}
                aria-label="{s.label} страница: как в альбоме"
                title="Как в альбоме"
                onclick={() => pick(s.scope, '')}>×</button
              >
            </div>
          {/each}
        {/if}
        <button class="ghost" disabled={!own} onclick={() => pick('spread', '')}>Как в альбоме</button>
      {/if}
      {#if prefs.can('bgPhoto')}<div data-slot="backdrop"><InspectorBackdrop {sp} {split} /></div>{/if}
    </div>
  {/if}
  {#if prefs.can('manual')}
    <div class="seg" role="group" aria-label="Раскладка разворота">
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
  .bg {
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
  }
  .cap {
    font-size: 12px;
    font-weight: 600;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 8px;
    font-size: 12px;
  }
  .lbl {
    flex: 1;
    color: var(--muted);
  }
  .link {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 4px;
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
    cursor: pointer;
  }
  .link:hover:not(:disabled) {
    color: var(--text);
  }
  .link:disabled {
    cursor: default;
    opacity: 0.6;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .chev {
    width: 10px;
    height: 10px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
    transition: transform var(--move);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .ghost {
    width: 100%;
    height: 24px;
    margin-top: 8px;
    padding: 0 6px;
    font-size: 12px;
  }
  .reset {
    width: 22px;
    height: 24px;
    padding: 0;
    font-size: 14px;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
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
