<script lang="ts">
  import { edit } from './edit.svelte'
  import { normDeg } from '../../base/geom/textbox'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import type { TextView } from '../../base/core/types'

  // Угол и ширина блока: поле «Угол, °» и переключатель «По тексту» — строка в InspectorText.
  // Только для блока, не для подписи.
  let { t }: { t: TextView } = $props()

  // Кнопка не забирает фокус у редактора: курсор остаётся в тексте.
  const keepFocus = (e: MouseEvent) => e.preventDefault()

  function commit(el: HTMLInputElement) {
    const raw = el.value.trim().replace(',', '.')
    const n = raw === '' ? NaN : Number(raw)
    if (!Number.isFinite(n)) {
      el.value = String(t.rot) // мусор — прежнее значение
      return
    }
    const deg = normDeg(n)
    el.value = String(deg)
    if (deg !== t.rot) edit.setAngle(t.id, deg)
  }
</script>

{#if prefs.can('textRotate')}
  <label class="angle" title="Угол поворота, градусы по часовой">
    <span>Угол, °</span>
    <input
      type="number"
      min="-180"
      max="180"
      step="1"
      aria-label="Угол, °"
      value={t.rot}
      onchange={(e) => commit(e.currentTarget)}
      onkeydown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur()
      }}
    />
  </label>
{/if}
<button
  class="auto"
  class:on={t.auto}
  disabled={t.auto}
  title={t.auto ? 'Потяните боковую ручку, чтобы задать ширину' : 'Ширина снова по тексту'}
  aria-pressed={t.auto}
  onmousedown={keepFocus}
  onclick={() => edit.setAuto(t.id)}>По тексту</button
>

<style>
  .angle {
    display: flex;
    flex: none;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: #cfcfd4;
  }
  .angle input {
    width: 46px;
    height: 24px;
    padding: 0 2px;
    font-size: 12px;
    text-align: center;
  }
  .auto {
    flex: none;
    height: 24px;
    padding: 0 6px;
    font-size: 12px;
    background: #3a3a3e;
  }
  .auto.on {
    background: var(--accent);
    color: #1e1e1e;
    opacity: 1;
  }
</style>
