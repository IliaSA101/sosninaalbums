<script lang="ts">
  import type { Unit } from '../../base/geom/sheet'
  import Seg from '../../base/tpl/Seg.svelte'
  import type { AutoDialog } from './autobuild.svelte'
  import { MAX_PER, RANGE_HINT, RANGE_MODES } from './autobuild'

  // 051: фото на разворот «от … до …»; «Авто» — от среднего, ввод в поле — «Свой».
  let { ui, unit, empty }: { ui: AutoDialog; unit: Unit; empty: boolean } = $props()

  // Фокус ушёл с поля: «от» > «до» подтягивается, пустое поле возвращает показанное число.
  const leave = (e: FocusEvent & { currentTarget: HTMLInputElement }, which: 'from' | 'to') => {
    ui.settle()
    e.currentTarget.value = String((which === 'from' ? ui.from : ui.to) ?? '')
  }
</script>

<div class="more" role="group" aria-label="Фото на {unit.one}">
  Фото на {unit.one}:
  <Seg
    items={RANGE_MODES}
    value={ui.auto ? 'auto' : 'own'}
    off={empty}
    onpick={(m) => (m === 'auto' ? ui.setAuto() : ui.setOwn())}
    label="Фото на {unit.one}: режим"
  />
  от
  <input
    id="auto-from"
    type="number"
    min="1"
    max={MAX_PER}
    step="1"
    value={ui.from ?? ''}
    disabled={empty}
    aria-label="Фото на {unit.one}: от"
    onblur={(e) => leave(e, 'from')}
    oninput={(e) => e.currentTarget.value !== '' && ui.setMinPer(e.currentTarget.valueAsNumber)}
  />
  до
  <input
    id="auto-to"
    type="number"
    min="1"
    max={MAX_PER}
    step="1"
    value={ui.to ?? ''}
    disabled={empty}
    aria-label="Фото на {unit.one}: до"
    onblur={(e) => leave(e, 'to')}
    oninput={(e) => e.currentTarget.value !== '' && ui.setMaxPer(e.currentTarget.valueAsNumber)}
  />
</div>
<p class="hint">{RANGE_HINT}</p>

<style>
  .more {
    display: flex;
    flex-wrap: wrap; /* узкое окно: «от…до» уходят на вторую строку */
    align-items: center;
    gap: 4px 8px;
  }
  .more input[type='number'] {
    width: 64px;
  }
  .hint {
    margin: 4px 0 0;
    min-height: 16px;
    color: var(--muted);
    font-size: 12px;
  }
</style>
