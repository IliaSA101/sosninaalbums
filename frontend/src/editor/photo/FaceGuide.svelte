<script lang="ts">
  import { FaceGuide } from '../../../wailsjs/go/app/App'
  import { store } from '../../base/core/store.svelte'
  import { outlineD, validGuide, veilD, type FaceGuideData } from '../../base/geom/faceguide'
  import { framelive } from './framelive.svelte'

  // Силуэт под лицо (130, silhouette.md): белая вуаль вокруг бюста, контур и жёлтая линия глаз в окне места.
  // Форму присылает Go (FaceGuide, мм окна), здесь только рисунок; на проект он не влияет и в выгрузку не идёт.
  // Лежит внутри .clip, поэтому форму места и поворот берёт у фото. show — сейчас подгоняют кадр;
  // onhave сообщает, есть ли силуэт для этого места (Cell тогда не рисует сетку третей).
  let {
    spreadId,
    photo,
    show,
    onhave,
  }: {
    spreadId: string
    photo: string
    show: boolean
    onhave: (have: boolean) => void
  } = $props()

  let g = $state<FaceGuideData | null>(null)
  let seq = 0 // номер запроса: устаревший ответ отбрасывается
  let key = ''

  // Перезапрос: другое фото или лист, правка (rev) и смена нормы листа (ответ «Подогнать по лицам»).
  $effect(() => {
    void store.view.rev
    void framelive.normRev
    const k = `${spreadId}/${photo}`
    if (k !== key) {
      key = k
      g = null // силуэт прежнего места чужой
    }
    const n = ++seq
    FaceGuide(spreadId, photo).then(
      (r) => {
        if (n === seq) g = validGuide(r) ? r : null
      },
      () => {
        if (n === seq) g = null // украшение, а не команда: тихо без силуэта
      },
    )
  })

  $effect(() => {
    onhave(g !== null)
    return () => onhave(false) // размонтирована: силуэта нет
  })
</script>

{#if show && g}
  <svg class="fg" viewBox="0 0 {g.w} {g.h}" preserveAspectRatio="none" aria-hidden="true" data-testid="face-guide">
    <path class="veil" d={veilD(g)} fill-rule="evenodd" />
    <path class="halo" d={outlineD(g)} />
    <path class="line" d={outlineD(g)} />
    <line class="halo" x1="0" x2={g.w} y1={g.eyeY} y2={g.eyeY} />
    <line class="eye" x1="0" x2={g.w} y1={g.eyeY} y2={g.eyeY} />
  </svg>
{/if}

<style>
  .fg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    overflow: hidden;
  }
  /* Вуаль: белая, 55 % (решение 0.27-130-24) */
  .veil {
    fill: rgba(255, 255, 255, 0.55);
  }
  /* Линии — как сетка третей: белые 75 % с тёмной каймой; толщина в px экрана, а не мм */
  .halo,
  .line,
  .eye {
    fill: none;
    vector-effect: non-scaling-stroke;
  }
  .halo {
    stroke: rgba(0, 0, 0, 0.45);
    stroke-width: 2.5px;
  }
  .line {
    stroke: rgba(255, 255, 255, 0.75);
    stroke-width: 1.5px;
  }
  .eye {
    stroke: var(--accent);
    stroke-width: 1.5px;
    stroke-dasharray: 6 4;
  }
</style>
