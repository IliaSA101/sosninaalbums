<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { pctFade } from '../backdrop/fade'
  import TiltRow from '../photoprops/TiltRow.svelte'
  import { borderAct } from './borderact.svelte'
  import { borderlive } from './borderlive.svelte'
  import { fadeRow } from './borderrows'

  // Строка «Прозрачность» фото в карточке (079): ползунок как у фото фона и стикера (строка TiltRow с числом и ⊗) — живой показ на oninput, один вызов
  // Go на change, стрелки в одном фокусе сливаются в один шаг отмены (merge-сессия). Едет с фото, как ч/б и тон:
  // «Авто» не закрепляется. У нескольких фото с разным значением — «—», первая правка ставит всем одно.
  let { photos }: { photos: string[] } = $props()

  const sp = $derived(store.spread)
  const sid = $derived(sp?.id ?? '')
  const base = $derived(fadeRow(sp, photos))
  // Ключ сессии — выделенные фото: смена выделения рвёт серию.
  const key = $derived(`fade:${sid}:${photos.join(',')}`)

  const preview = (v: number) => borderlive.set({ spread: sid, ids: [...photos], fade: pctFade(v) })
  const commit = (v: number) => borderAct.fade([...photos], pctFade(v), key)

  // Смена выделения или уход панели: чужой просмотр не оставляем.
  $effect(() => {
    void key
    return () => borderAct.end()
  })
</script>

<!-- Строка TiltRow: ползунок, число и ⊗; сессия merge — с входа фокуса в строку до ухода из неё (onstart/onleave) -->
<TiltRow
  tilt={base.pct}
  mixed={base.mixed}
  label="Прозрачность"
  numLabel="Прозрачность фото, процент"
  rangeLabel="Прозрачность фото, %"
  rangeTitle="Чем больше, тем бледнее фото: сквозь него проступает то, что лежит ниже"
  min={0}
  max={100}
  step={1}
  guide="photo-fade"
  id="photo-fade"
  reset
  resetLabel="Сбросить: прозрачность"
  onlive={preview}
  oncommit={commit}
  onend={() => borderlive.release()}
  onstart={() => borderAct.session.begin(key)}
  onleave={() => borderAct.end()}
/>
