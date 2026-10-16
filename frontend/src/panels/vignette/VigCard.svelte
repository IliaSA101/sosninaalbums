<script lang="ts">
  import type { VigChoice } from '../../base/core/types'
  import { toneOf } from './vignette'
  import { liveRadius, shapeCss } from '../../base/geom/shape'
  import { fitBox, type TileForm } from './vigown'

  // Превью шаблона виньетки: места по долям листа, роли разным тоном (c.head, c.shape — поля 049, Go отдаёт с G3). Общее у панели в карточке и галереи
  // раскладок; размер задаёт вызывающий, подпись и выбор — тоже его.
  // form (044) — форма мест людей из окна или листа: плитка рисует её у учеников и учителей (только рисование);
  // групповое фото всегда прямоугольное. Нет form — форма самой коробки шаблона (свой шаблон с формой).
  // form.ratio/from (049): коробки Go посчитаны в портрете from, а щелчок поставит ratio — места людей вписываются в ratio
  // (круг 1:1 в 3:4-коробку), без этого плитка «кругов» рисует эллипс, а щелчок даёт круг.
  let { c, w, h, form }: { c: VigChoice; w: number; h: number; form?: Partial<TileForm> } = $props()

  // Скругление — % меньшей стороны, считаем по размеру места на плитке (пропорция листа та же, что у мм).
  type Box = VigChoice['boxes'][number]
  // Коробка места на плитке: людям — с пропорцией результата, группе — как есть.
  function placed(b: Box, role: string | undefined): Box {
    if (toneOf(role) === 'group' || !form?.ratio || !form.from || form.ratio === form.from) return b
    return { ...b, ...fitBox(b, w, h, form.ratio) }
  }

  function radiusOf(b: Box, role: string | undefined): string | undefined {
    const g = toneOf(role) === 'group'
    // Каталожные «круги» (049) всегда рисуют места людей кругами: подсказка шаблона важнее формы окна
    const hint = !c.own && c.shape === 'oval'
    const shape = g ? '' : hint ? 'oval' : form ? form.shape : b.shape
    const round = g ? 0 : ((form ? form.round : b.round) ?? 0)
    const bw = b.w * w
    const bh = b.h * h
    return shapeCss(shape, liveRadius(round, bw, bh), bw, bh)
  }
</script>

<span class="card" class:over={c.over} style:width="{w}px" style:height="{h}px">
  {#each c.boxes as raw, i (i)}
    {@const b = placed(raw, c.roles[i])}
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
