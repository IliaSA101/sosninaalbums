<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { fadePct, pctFade } from '../backdrop/fade'
  import { borderAct } from './borderact.svelte'
  import { borderlive } from './borderlive.svelte'
  import { fadeRow } from './borderrows'

  // Строка «Прозрачность» фото в карточке (079): ползунок как у фото фона и стикера — живой показ на oninput, один вызов
  // Go на change, стрелки в одном фокусе сливаются в один шаг отмены (merge-сессия). Едет с фото, как ч/б и тон:
  // «Авто» не закрепляется. У нескольких фото с разным значением — «—», первая правка ставит всем одно.
  let { photos }: { photos: string[] } = $props()

  const sp = $derived(store.spread)
  const sid = $derived(sp?.id ?? '')
  const base = $derived(fadeRow(sp, photos))
  const live = $derived(photos[0] ? borderlive.fadeOf(sid, photos[0], -1) : -1)
  const pct = $derived(live >= 0 ? fadePct(live) : base.pct)
  const mixed = $derived(base.mixed && live < 0)
  // Ключ сессии — выделенные фото: смена выделения рвёт серию.
  const key = $derived(`fade:${sid}:${photos.join(',')}`)

  const val = (e: Event) => +(e.currentTarget as HTMLInputElement).value

  const commit = (e: Event) => borderAct.fade([...photos], pctFade(val(e)), key)

  // Смена выделения или уход панели: чужой просмотр не оставляем.
  $effect(() => {
    void key
    return () => borderAct.end()
  })
</script>

<div class="row" data-guide="photo-fade" title="Чем больше, тем бледнее фото: сквозь него проступает то, что лежит ниже">
  <span class="lbl">Прозрачность</span>
  <input
    type="range"
    min="0"
    max="100"
    step="1"
    value={pct}
    aria-label="Прозрачность фото, %"
    aria-valuetext={mixed ? 'разная' : `${pct} %`}
    onfocus={() => borderAct.session.begin(key)}
    onblur={() => borderAct.end()}
    oninput={(e) => borderlive.set({ spread: sid, ids: [...photos], fade: pctFade(val(e)) })}
    onchange={commit}
    onpointerup={(e) => e.currentTarget.blur()}
  />
  <span class="num">{mixed ? '—' : `${pct} %`}</span>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 10px;
    font-size: 12px;
  }
  .lbl {
    flex: none;
    color: #cfcfd4;
  }
  input {
    flex: 1;
    min-width: 0;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }
  .num {
    flex: none;
    width: 34px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>
