<script lang="ts">
  import type { CoverView } from '../../base/core/types'
  import { modes } from '../../base/prefs/view.svelte'
  import { boxCss, labeled, zonesOf } from './cover'

  // Направляющие обложки (спека 032, п. 6): сторонки с подписями, корешок, расстав, штриховка заворота за
  // линией обреза, линия обреза и пунктир безопасной зоны (клавиша G). Только на экране: не выделяются,
  // мышь не ловят, в экспорт не идут. Проценты — от размеров листа (cover.ts).
  let { cover }: { cover: CoverView } = $props()

  const sheet = $derived({ w: cover.sheet.w, h: cover.sheet.h })
  const named = $derived(labeled(cover.zones.filter((z) => z.kind !== 'trim' && z.kind !== 'safe')))
  const trim = $derived(zonesOf(cover, 'trim'))
  const safe = $derived(modes.safe ? zonesOf(cover, 'safe') : [])
  // Узкие зоны (корешок, расстав) подписываются вдоль: слово в 14 мм по горизонтали не поместится.
  const tall = (kind: string) => kind === 'spine' || kind === 'hinge'
</script>

<div class="guides" aria-hidden="true">
  {#each named as { z, label }, i (i)}
    <div class="zone {z.kind}" style={boxCss(z.rect, sheet)}>
      {#if label}<span class="lbl" class:tall={tall(z.kind)}>{label}</span>{/if}
    </div>
  {/each}
  {#each trim as z, i (i)}
    <div class="trim" style={boxCss(z.rect, sheet)}></div>
  {/each}
  {#each safe as z, i (i)}
    <div class="safe" style={boxCss(z.rect, sheet)}></div>
  {/each}
</div>

<style>
  .guides {
    position: absolute;
    inset: 0;
    z-index: 6;
    pointer-events: none;
    overflow: hidden;
  }
  .zone,
  .trim,
  .safe {
    position: absolute;
    box-sizing: border-box;
  }
  /* Заворот — штриховка: эту полосу типография загибает на картон, она за линией обреза */
  .zone.wrap {
    background: repeating-linear-gradient(135deg, rgba(0, 0, 0, 0.28) 0 4px, rgba(0, 0, 0, 0.1) 4px 8px);
  }
  .zone.hinge {
    background: rgba(255, 255, 255, 0.14);
    border-left: 1px dashed rgba(255, 255, 255, 0.8);
    border-right: 1px dashed rgba(255, 255, 255, 0.8);
  }
  .zone.spine {
    border-left: 1.5px dashed rgba(255, 255, 255, 0.9);
    border-right: 1.5px dashed rgba(255, 255, 255, 0.9);
    box-shadow:
      -1px 0 0 rgba(0, 0, 0, 0.45),
      1px 0 0 rgba(0, 0, 0, 0.45);
  }
  .zone.back,
  .zone.front {
    display: grid;
    place-items: end start;
  }
  /* Линия обреза: двойная (белая + тёмная), видна и на фото, и на белом */
  .trim {
    border: 1px solid rgba(255, 255, 255, 0.95);
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.45),
      inset 0 0 0 1px rgba(0, 0, 0, 0.45);
  }
  .safe {
    border: 1.5px dashed var(--accent);
    box-shadow:
      0 0 0 1px rgba(0, 0, 0, 0.45),
      inset 0 0 0 1px rgba(0, 0, 0, 0.45);
  }
  .lbl {
    max-width: 100%;
    margin: 4px;
    padding: 1px 6px;
    font-size: 11px;
    line-height: 16px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: #fff;
    background: rgba(0, 0, 0, 0.55);
    border-radius: 3px;
  }
  .zone.wrap .lbl {
    position: absolute;
    left: 50%;
    top: 50%;
    margin: 0;
    transform: translate(-50%, -50%);
  }
  /* Подпись вдоль корешка (читается снизу вверх, как надпись на нём) */
  .zone.spine,
  .zone.hinge {
    display: grid;
    place-items: center;
  }
  .lbl.tall {
    margin: 0;
    padding: 6px 1px;
    writing-mode: vertical-rl;
    transform: rotate(180deg);
    max-height: 100%;
  }
</style>
