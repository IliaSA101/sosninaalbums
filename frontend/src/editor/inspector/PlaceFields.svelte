<script lang="ts">
  import { store, Api } from '../../base/core/store.svelte'
  import { gridAct } from '../../panels/layouts/gridact.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { fieldBox, fmtMM, sameBox, type Field } from '../../base/geom/gridedit'
  import { fieldRotated } from '../../base/geom/rotbox'
  import { normDeg } from '../../base/geom/textbox'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import Icon from '../../base/ui/Icon.svelte'

  // Поля X/Y/Ш/В образца (первое выделенное) и замок пропорций — общие для карточек «Место» и «Фото»
  // (спека 008). Без выделения — серые.
  const s = $derived(store.view.settings)
  const bounds = $derived({ w: s.pageW * 2, h: s.pageH })
  const sample = $derived(placesel.selected[0])

  // Enter или уход из поля: число уходит в SetPlaces одной правкой, мусор возвращает прежнее значение.
  // У «Авто» id — фото, и первая такая правка закрепляет разворот (gridAct.placeEdit).
  function commit(f: Field, el: HTMLInputElement) {
    const sp = store.spread
    if (!sample || !sp) return
    const v = parseFloat(el.value.replace(',', '.'))
    if (!Number.isFinite(v)) {
      el.value = fmtMM(sample[f])
      return
    }
    // Повёрнутое место зажимает Go (вылет не растёт), а не «внутрь разворота» — предпросмотр тот же.
    const next = sample.rot ? fieldRotated(sample, f, v, placesel.lock, bounds.w, bounds.h) : fieldBox(sample, f, v, placesel.lock, bounds)
    el.value = fmtMM(next[f])
    if (sameBox(next, sample)) return
    gridAct.placeEdit(sp.id, Api.SetPlaces(sp.id, [{ id: sample.id, x: next.x, y: next.y, w: next.w, h: next.h }], ''))
  }
  // «Угол, °»: вокруг центра места; у «Авто» первая такая правка закрепляет разворот.
  function commitAngle(el: HTMLInputElement) {
    const sp = store.spread
    if (!sample || !sp) return
    const v = parseFloat(el.value.replace(',', '.'))
    if (!Number.isFinite(v)) {
      el.value = fmtMM(sample.rot)
      return
    }
    const deg = normDeg(v)
    el.value = fmtMM(deg)
    if (deg !== sample.rot) gridAct.placeEdit(sp.id, Api.SetPlaceAngle(sp.id, sample.id, deg))
  }
  // Enter — применить (change придёт при уходе из поля); Esc — вернуть значение, выделение не снимать.
  function key(f: Field | 'rot', e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      if (sample) el.value = fmtMM(f === 'rot' ? sample.rot : sample[f])
      el.blur()
    }
  }
</script>

{#snippet field(f: Field, label: string, hint: string)}
  <label class="f" title={hint}>
    <span>{label}</span>
    <input
      type="number"
      step="0.1"
      disabled={!sample}
      value={sample ? fmtMM(sample[f]) : ''}
      aria-label={hint}
      onchange={(e) => commit(f, e.currentTarget)}
      onkeydown={(e) => key(f, e)}
    />
  </label>
{/snippet}

<div class="grid" class:off={!sample}>
  {@render field('x', 'X', 'X, мм от левого края разворота')}
  <span></span>
  {@render field('y', 'Y', 'Y, мм от верхнего края разворота')}
  {@render field('w', 'Ш', 'Ширина, мм')}
  <button
    class="lock"
    class:on={placesel.lock}
    onclick={() => placesel.toggleLock()}
    aria-pressed={placesel.lock}
    aria-label="Замок пропорций"
    title="Замок пропорций: ширина и высота меняются вместе"><Icon name="lock" size={14} /></button
  >
  {@render field('h', 'В', 'Высота, мм')}
  {#if prefs.can('rotate')}
    <span class="unit">Угол, °</span>
    <span></span>
    <label class="f" title="Угол поворота места вокруг центра, °">
      <input
        type="number"
        step="1"
        disabled={!sample}
        value={sample ? fmtMM(sample.rot) : ''}
        aria-label="Угол, °"
        onchange={(e) => commitAngle(e.currentTarget)}
        onkeydown={(e) => key('rot', e)}
      />
    </label>
  {/if}
</div>

<style>
  .off {
    opacity: 0.55;
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 22px 1fr;
    align-items: center;
    gap: 6px 4px;
  }
  .f {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: var(--muted);
  }
  .f span {
    width: 12px;
    flex: none;
  }
  .f input {
    width: 100%;
    min-width: 0;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .unit {
    font-size: 12px;
    color: var(--muted);
  }
  .lock {
    width: 22px;
    height: 22px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  .lock.on {
    background: var(--accent);
    color: #1e1e1e;
  }
</style>
