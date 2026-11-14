<script lang="ts">
  import { tick } from 'svelte'
  import type { BackdropScope } from '../../base/core/types'
  import type { Unit } from '../../base/geom/sheet'
  import Seg from '../../base/tpl/Seg.svelte'
  import { BG_ALBUM_HINT, bgHint, bgPickItems, type BgPick } from './bgtargets'

  // Строка «На: Этот разворот · Все развороты (N) · Весь альбом · Выделенные (N) · Номера [3, 5-8]» под «Куда» в окне «Фоны» (077, 100-7).
  // Seg, как «Куда» в том же окне. Решения и число листов — у родителя (decor.svelte.ts), здесь только вид.
  // focus растёт, когда щелчок по плитке пришёл при неверных номерах.
  let {
    pick = $bindable(),
    text = $bindable(),
    selN,
    total,
    unit,
    scope,
    n,
    error,
    focus,
  }: {
    pick: BgPick
    text: string
    selN: number
    total: number
    unit: Unit
    scope: BackdropScope
    n: number // сколько листов затронет выбор
    error: string | null // ошибка номеров (только при pick = 'nums')
    focus: number
  } = $props()

  let field = $state<HTMLInputElement>()
  $effect(() => {
    if (focus > 0) field?.focus()
  })
  const bad = $derived(pick === 'nums' && !!error && (text.trim() !== '' || focus > 0))
  // «Выделенные» могли пропасть (выделение снято): показываем то, что реально сработает
  const shown = $derived<BgPick>(pick === 'sel' && selN < 2 ? 'this' : pick)

  function choose(p: BgPick) {
    pick = p
    if (p === 'nums') void tick().then(() => field?.focus())
  }
</script>

<div class="targets">
  <span class="lbl">На</span>
  <Seg items={bgPickItems(selN, total, unit)} value={shown} onpick={choose} label="На какие {unit.many}" />
  {#if pick === 'nums'}
    <input
      type="text"
      class="ranges"
      class:bad
      placeholder="3, 5-8"
      aria-label="Номера {unit.plural[2]}"
      bind:this={field}
      bind:value={text}
    />
  {/if}
  {#if shown === 'album'}
    <span class="hint">{BG_ALBUM_HINT}</span>
  {:else if shown !== 'this' && n > 0 && !(pick === 'nums' && error)}
    <span class="hint">{bgHint(n, scope, unit)}</span>
  {/if}
  {#if bad}<span class="err">{error}</span>{/if}
</div>

<style>
  .targets {
    flex: none;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px 8px;
    font-size: 13px;
  }
  .lbl {
    color: var(--muted);
    font-size: 12px;
  }
  .ranges {
    width: 110px;
    height: 26px;
    padding: 0 8px;
    font: inherit;
    background: #3a3a3e;
    color: var(--text);
    border: 1px solid var(--line);
    border-radius: 6px;
    user-select: text;
  }
  .ranges.bad {
    border-color: #c0392b;
  }
  .err {
    flex-basis: 100%;
    color: #e5645a;
    font-size: 12px;
  }
  .hint {
    margin-left: auto;
    color: var(--muted);
    font-size: 12px;
  }
</style>
