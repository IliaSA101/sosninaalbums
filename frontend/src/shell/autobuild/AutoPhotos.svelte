<script lang="ts">
  import type { AutoPlan } from '../../base/core/types'
  import type { Unit } from '../../base/geom/sheet'
  import Seg from '../../base/tpl/Seg.svelte'
  import AutoSimilarList from './AutoSimilarList.svelte'
  import type { PhotosState } from './autosimilar.svelte'
  import { GRAY_HINT, GRAY_OPTIONS, SIMILAR_OPTIONS, canShowDrops, grayText, sigsText, similarHint, similarStatus } from './autosimilar'

  // Блок «Фото» окна автосборки (078): «Убирать похожие кадры» с рядом отброшенных и «Ч/б развороты».
  // Образец — «Разнообразие раскладок» в блоке «Раскладки»: подпись, сегменты и строка-подсказка под ними.
  let { ph, plan, unit, empty, grayOn }: { ph: PhotosState; plan: AutoPlan | null; unit: Unit; empty: boolean; grayOn: boolean } = $props()

  const dropped = $derived(plan?.similar ?? 0)
  const status = $derived(
    similarStatus({ level: ph.similar, waiting: ph.waiting, sigs: ph.sigs, dropped, kept: ph.keep.size, error: ph.dropsError }),
  )
  // Список раскрывается, только когда подписи готовы и есть что показать
  const canShow = $derived(ph.similar > 0 && !ph.waiting && canShowDrops(dropped, ph.keep.size))
  const showing = $derived(ph.dropsOpen && canShow)
</script>

<div class="photos" data-guide="auto-similar">
  <div class="more">
    Убирать похожие кадры
    <Seg
      items={SIMILAR_OPTIONS.map((o) => ({ id: o.v, label: o.label, title: o.title }))}
      value={ph.similar}
      off={empty}
      onpick={(v) => ph.setSimilar(v)}
      label="Убирать похожие кадры"
    />
  </div>
  <p class="hint" role="status">{similarHint(ph.similar)}</p>
  {#if ph.similar > 0}
    <p class="status" role="status">
      {status}
      {#if canShow}
        · <button type="button" class="link" aria-expanded={showing} onclick={() => ph.setDropsOpen(!ph.dropsOpen)}>
          {showing ? 'Скрыть' : 'Показать'}
        </button>
      {/if}
    </p>
  {/if}
  {#if showing}
    <AutoSimilarList {ph} />
  {/if}

  {#if grayOn}
    <div class="more gray" data-guide="auto-gray">
      Ч/б развороты
      <Seg
        items={GRAY_OPTIONS.map((o) => ({ id: o.v, label: o.label }))}
        value={ph.gray}
        off={empty}
        onpick={(v) => ph.setGray(v)}
        label="Ч/б развороты"
      />
    </div>
    <p class="hint" role="status">{GRAY_HINT}</p>
    {#if ph.gray > 0}
      <p class="status" role="status">{ph.waiting ? sigsText(ph.sigs) : grayText(ph.gray, plan?.gray ?? 0, unit)}</p>
    {/if}
  {/if}
</div>

<style>
  .photos {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
    padding-top: 10px;
    border-top: 1px solid var(--line);
  }
  .more {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }
  .gray {
    margin-top: 4px;
  }
  .hint,
  .status {
    margin: 0;
    min-height: 16px;
    color: var(--muted);
    font-size: 12px;
  }
  .link {
    padding: 0;
    background: transparent;
    border: 0;
    color: var(--accent);
    font-size: 12px;
    cursor: pointer;
  }
</style>
