<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import type { PrintKind, PrintPlan } from '../../base/core/types'
  import { coverAct } from '../../panels/cover/coveract.svelte'
  import ExportGroup from './ExportGroup.svelte'
  import { hasFix, memoLine, printHint, togglePrint } from './exportopts'

  // Группа «Печать» окна «Экспорт» (047, 081; вынесено из ExportDialog): по флажку на каждую типографию из планов Go (страница чужого
  // формата — тоже, с подсказкой off и запретом под включённым флажком); включён не больше одного. Под флажками — памятка, запреты Go и предупреждения включённой типографии.
  // Все тексты и проверки приходят из Go; здесь только рисуем.
  let {
    plans,
    pref,
    plan,
    hasCover,
    setPref,
  }: {
    plans: readonly PrintPlan[]
    pref: '' | PrintKind
    plan: PrintPlan | null
    hasCover: boolean
    setPref: (v: '' | PrintKind) => void
  } = $props()
</script>

{#if plans.length}
  <ExportGroup cap="Печать">
    {#each plans as p (p.kind)}
      <label
        ><input
          type="checkbox"
          checked={pref === p.kind}
          onchange={(e) => setPref(togglePrint(pref, p.kind, e.currentTarget.checked))}
        />{p.title}
        <span class="hint">{p.off ? `(${p.off})` : printHint(p, hasCover)}</span></label
      >
    {/each}
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
  label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
  input[type='checkbox'] {
    accent-color: var(--accent);
    margin: 0;
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
  /* Предупреждения плана — под флажком, с тем же отступом, что у подсказок */
  p.warn.fs {
    margin: -6px 0 12px 78px;
    color: var(--text);
    font-size: 12px;
  }
</style>
