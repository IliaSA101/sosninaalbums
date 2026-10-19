<script lang="ts">
  import type { BackdropScope } from '../../base/core/types'
  import type { Unit } from '../../base/geom/sheet'
  import { bgHint, bgSelShown, type BgPick } from './bgtargets'

  // Строка «на этот разворот · на выделенные в ленте (N) · на развороты: [3, 5-8] · на все развороты (N)» под «Куда»
  // в окне «Фоны» (077). Слова и поле номеров — как в «Копировать оформление». Решения и число листов — у родителя
  // (decor.svelte.ts), здесь только вид. focus растёт, когда щелчок по плитке пришёл при неверных номерах.
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
</script>

<div class="targets" role="radiogroup" aria-label="На какие {unit.many}">
  <label><input type="radio" name="bg-pick" value="this" bind:group={pick} />на этот {unit.one}</label>
  {#if bgSelShown(selN)}
    <label><input type="radio" name="bg-pick" value="sel" bind:group={pick} />на выделенные в ленте ({selN})</label>
  {/if}
  <label class="inline"
    ><input type="radio" name="bg-pick" value="nums" bind:group={pick} />на {unit.many}:
    <input
      type="text"
      class="ranges"
      class:bad
      placeholder="3, 5-8"
      aria-label="Номера {unit.plural[2]}"
      bind:this={field}
      bind:value={text}
      onfocus={() => (pick = 'nums')}
    /></label
  >
  <label><input type="radio" name="bg-pick" value="all" bind:group={pick} />на все {unit.many} ({total})</label>
  {#if pick !== 'this' && n > 0 && !(pick === 'nums' && error)}
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
    gap: 4px 14px;
    font-size: 13px;
  }
  label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
  input[type='radio'] {
    accent-color: var(--accent);
    margin: 0;
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
