<script lang="ts">
  import { placesel } from '../places/placesel.svelte'
  import { setGroupField } from '../places/groupgesture'
  import type { FrameField } from '../../base/groupgeom/groupframe'
  import { fmtMM } from '../../base/geom/gridedit'
  import { prefs } from '../../base/prefs/prefs.svelte'

  // Поля рамки группы (спека 018, заменяет 014): X/Y — левый верхний угол неповёрнутой рамки (то, что обводит
  // пунктир GroupFrame), Ш/В — её стороны в осях рамки, «Угол» — угол рамки. Замка нет: Ш тянет только ширину от
  // левой стороны, В — только высоту от верхней (пропорционально — угловыми ручками). X/Y — перенос, Ш/В —
  // ScaleGroupAxes, угол — RotateGroup. Зажимы делает Go, а после правки в поле встаёт то, что получилось.
  const frame = $derived(placesel.frame)
  const scaleOn = $derived(prefs.can('groupXf'))
  // Угол правится, если разрешены масштаб группы и поворот того, что в группе есть; иначе только виден.
  const rotShown = $derived(prefs.can('rotate') || prefs.can('textRotate'))
  const rotOn = $derived(
    scaleOn && (placesel.selected.length === 0 || prefs.can('rotate')) && (placesel.texts.length === 0 || prefs.can('textRotate')),
  )

  const valueOf = (f: FrameField | 'a') => (f === 'a' ? fmtMM(placesel.phi) : frame ? fmtMM(frame[f]) : '')

  // Enter или уход из поля: число — одна правка в очереди (у «Авто» с фото она же закрепляет разворот); мусор и то же
  // значение возвращают прежнее. Итог берём из свежей рамки: Go мог зажать до прежнего, и тогда реактивное
  // значение не сменится, а в поле останется введённое. Пока в очереди есть ещё правки, поле не трогаем.
  let pending = 0
  async function commit(f: FrameField | 'a', el: HTMLInputElement) {
    pending++
    await setGroupField(f, parseFloat(el.value.replace(',', '.')))
    if (--pending === 0) el.value = valueOf(f)
  }
  // Enter — применить (change придёт при уходе из поля); Esc — вернуть значение, выделение не снимать.
  function key(f: FrameField | 'a', e: KeyboardEvent) {
    const el = e.currentTarget as HTMLInputElement
    if (e.key === 'Enter') el.blur()
    else if (e.key === 'Escape') {
      e.stopPropagation()
      el.value = valueOf(f)
      el.blur()
    }
  }
</script>

{#snippet field(f: FrameField, label: string, hint: string, off = false)}
  <label class="f" title={off ? 'Масштаб и поворот группы выключены в уровне' : hint}>
    <span>{label}</span>
    <input
      type="number"
      step="0.1"
      disabled={!frame || off}
      value={valueOf(f)}
      aria-label={hint}
      onchange={(e) => commit(f, e.currentTarget)}
      onkeydown={(e) => key(f, e)}
    />
  </label>
{/snippet}

<div class="grid">
  {@render field('x', 'X', 'X рамки группы, мм от левого края разворота')}
  {@render field('y', 'Y', 'Y рамки группы, мм от верхнего края разворота')}
  {@render field('w', 'Ш', 'Ширина рамки группы, мм: тянет от левой стороны', !scaleOn)}
  {@render field('h', 'В', 'Высота рамки группы, мм: тянет от верхней стороны', !scaleOn)}
  {#if rotShown}
    <span class="unit">Угол, °</span>
    <label class="f" title={placesel.mixed ? 'Угол рамки группы; у объектов разные углы' : 'Угол рамки группы'}>
      <input
        type="number"
        step="any"
        disabled={!frame || !rotOn}
        value={valueOf('a')}
        aria-label="Угол рамки группы, °"
        onchange={(e) => commit('a', e.currentTarget)}
        onkeydown={(e) => key('a', e)}
      />
    </label>
  {/if}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    align-items: center;
    gap: 6px 8px;
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
</style>
