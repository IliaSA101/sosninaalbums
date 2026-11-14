<script lang="ts">
  import type { VigChoice } from '../../base/core/types'
  import { toneOf } from './vignette'
  import { liveRadius, shapeCss } from '../../base/geom/shape'

  // Превью шаблона виньетки: места по долям листа, роли разным тоном (c.head — шапка, Go отдаёт с G3). Общее у панели в карточке
  // и галереи раскладок; размер задаёт вызывающий, подпись и выбор — тоже его.
  // form (044, 056) — форма мест людей: плитка рисует её у учеников и учителей (только рисование); групповое фото
  // всегда прямоугольное. Нет form — форма самой коробки шаблона. Портрет коробок уже тот, что получится после щелчка.
  let { c, w, h, form }: { c: VigChoice; w: number; h: number; form?: { shape?: string; round?: number } } = $props()

  // Скругление — % меньшей стороны, считаем по размеру места на плитке (пропорция листа та же, что у мм).
  type Box = VigChoice['boxes'][number]

  function radiusOf(b: Box, role: string | undefined): string | undefined {
    const g = toneOf(role) === 'group'
    const shape = g ? '' : form ? form.shape : b.shape
    const round = g ? 0 : ((form ? form.round : b.round) ?? 0)
    const bw = b.w * w
    const bh = b.h * h
    return shapeCss(shape, liveRadius(round, bw, bh), bw, bh)
  }
</script>

<span class="card" class:over={c.over} style:width="{w}px" style:height="{h}px">
  {#each c.boxes as b, i (i)}
    <span
      class="cell {toneOf(c.roles[i])}"
      style:left="{b.x * 100}%"
      style:top="{b.y * 100}%"
      style:width="{b.w * 100}%"
      style:height="{b.h * 100}%"
      style:border-radius={radiusOf(b, c.roles[i])}
    ></span>
  {/each}
  <!-- Шапка листа (049): полоса тоном заголовка, чтобы виды «шапка сверху / в ряду» отличались на плитке -->
  {#if c.head}
    <span
      class="cell head"
      style:left="{c.head.x * 100}%"
      style:top="{c.head.y * 100}%"
      style:width="{c.head.w * 100}%"
      style:height="{c.head.h * 100}%"
    ></span>
  {/if}
</span>

<style>
  .card {
    position: relative;
    display: block;
    flex: none;
    background: var(--panel2);
    border: 1px solid var(--line);
    border-radius: 3px;
  }
  .card.over {
    opacity: 0.35;
  }
  /* Роли разным тоном: учителя — светлые, ученики — серые, групповое — акцентное. */
  .cell {
    position: absolute;
    box-sizing: border-box;
    border-radius: 1px;
  }
  .cell.pupil {
    background: #8a8a92;
  }
  .cell.teacher {
    background: #e8e8ea;
  }
  .cell.group {
    background: var(--accent);
  }
  .cell.head {
    background: #5a5a62;
  }
</style>
