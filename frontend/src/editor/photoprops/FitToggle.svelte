<script lang="ts">
  import Seg, { type SegItem } from '../../base/tpl/Seg.svelte'
  import { placesel } from '../places/placesel.svelte'

  // «Целиком / С обрезкой» выделенного (спека 008): подсвечено, только если так у всех выделенных.
  // У «Авто» «Целиком» закрепляет разворот (gridAct.placeEdit). Ничего не выделено — сегмент серый.
  const n = $derived(placesel.selected.length)
  type Fit = 'contain' | 'cover'
  const ITEMS: SegItem<Fit>[] = [
    { id: 'contain', label: 'Целиком', title: 'Вписать фото в место целиком, без обрезки' },
    { id: 'cover', label: 'С обрезкой', title: 'Заполнять место с обрезкой' },
  ]
  // Нажаты оба сразу не бывают; ни один — смешанное выделение.
  const pressed = $derived<Fit[]>(placesel.allContain ? ['contain'] : placesel.allCover ? ['cover'] : [])
</script>

<div class="fit" data-guide="fit-toggle">
  <Seg look="card" label="Фото в месте" items={ITEMS} values={pressed} off={n === 0} onpick={(f) => placesel.setFit(f === 'contain')} />
</div>

<style>
  .fit {
    margin-top: 10px;
  }
</style>
