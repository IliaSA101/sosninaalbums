<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import type { PrintKind, PrintPlan } from '../../base/core/types'
  import { coverAct } from '../../panels/cover/coveract.svelte'
  import ExportGroup from './ExportGroup.svelte'
  import { printList } from '../printprof/printlist.svelte'
  import { hasFix, kindOf, memoLine, printHint } from './exportopts'

  // Раздел «Печать» окна «Экспорт» (047, 081, 123): один список «Обычная выгрузка» + типографии из планов Go
  // (страница чужого формата — тоже, с подсказкой off и запретом). Выбор при открытии делает Go (Plan.default),
  // дальше — выбор фотографа. Под списком — подсказка выбранной типографии, памятка, запреты Go и предупреждения.
  // Все тексты и проверки приходят из Go; здесь только рисуем.
  let {
    plans,
    plan,
    hasCover,
    setPref,
  }: {
    plans: readonly PrintPlan[]
    plan: PrintPlan | null
    hasCover: boolean
    setPref: (v: '' | PrintKind) => void
  } = $props()
</script>

{#if plans.length}
  <ExportGroup cap="Печать">
    <select aria-label="Печать" value={plan?.kind ?? ''} onchange={(e) => setPref(kindOf(e.currentTarget.value, plans))}>
      <option value="">Обычная выгрузка</option>
      {#each plans as p (p.kind)}<option value={p.kind}>{printList.planLabel(p)}</option>{/each}
    </select>
    {#if plan}<span class="hint">{plan.off ? `(${plan.off})` : printHint(plan, hasCover)}</span>{/if}
  </ExportGroup>
  {#if plan}
    {#if plan.memo.length}<p class="hint">{memoLine(plan.memo)}</p>{/if}
    {#each plan.block as line, i (line)}
      <p class="err blocked">
        {line}
        {#if hasFix(plan, i) && store.cover}
          <button class="fix" onclick={() => coverAct.applyPreset(store.cover!.preset)}>Исправить</button>
        {/if}
      </p>
    {/each}
    {#each plan.warn as line (line)}<p class="warn fs">{line}</p>{/each}
  {/if}
{/if}

<style>
  select {
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .err {
    margin: -6px 0 4px 78px;
    color: #e5645a;
    font-size: 12px;
  }
  /* Кнопка «Исправить» стоит рядом с текстом, как в карточке обложки */
  .err.blocked {
    margin-top: -2px;
  }
  .fix {
    height: 22px;
    margin-left: 6px;
    padding: 0 8px;
    font-size: 12px;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    margin: 0;
  }
  p.hint {
    margin: -6px 0 12px 78px;
  }
  /* Предупреждения плана — под списком, с тем же отступом, что у подсказок */
  p.warn.fs {
    margin: -6px 0 12px 78px;
    color: var(--text);
    font-size: 12px;
  }
</style>
