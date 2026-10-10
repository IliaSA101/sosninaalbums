<script lang="ts">
  import type { BackdropView } from '../../base/core/types'
  import { bgmode } from './bgmode.svelte'
  import { fitBackdrop, flipBackdrop } from './backdropact'
  import { normDeg, rotateAbout } from '../../base/geom/textbox'
  import { MAX_SCALE, MIN_SCALE, zoomFrame } from '../../base/geom/frame'
  import Icon from '../../base/ui/Icon.svelte'

  // Карточка в режиме фона (design-backdrop §6.2): масштаб фото в рамке, поворот рамки вокруг центра,
  // зеркало, «По области», «Убрать», «Готово». Поле применяется по Enter или уходу из поля.
  let { b, spread }: { b: BackdropView; spread: string } = $props()

  const num = (el: HTMLInputElement) => {
    const raw = el.value.trim().replace(',', '.')
    return raw === '' ? NaN : Number(raw)
  }
  const pct = $derived(Math.round(b.frame.s * 100))

  function commitScale(el: HTMLInputElement) {
    const n = num(el)
    if (!Number.isFinite(n)) {
      el.value = String(pct) // мусор — прежнее значение
      return
    }
    const s = Math.min(MAX_SCALE, Math.max(MIN_SCALE, n / 100))
    el.value = String(Math.round(s * 100))
    if (s !== b.frame.s) bgmode.setFrame(zoomFrame(b.frame, s))
  }

  function commitRot(el: HTMLInputElement) {
    const n = num(el)
    if (!Number.isFinite(n)) {
      el.value = String(b.box.rot)
      return
    }
    const deg = normDeg(n)
    el.value = String(deg)
    if (deg !== b.box.rot) bgmode.setBox(rotateAbout(b.box, deg))
  }
  const enter = (e: KeyboardEvent) => {
    if (e.key === 'Enter') (e.currentTarget as HTMLInputElement).blur()
  }
</script>

<div class="fields">
  <label title="Масштаб фото в рамке, 100–400 %">
    <span>Масштаб, %</span>
    <input type="number" min="100" max="400" step="1" value={pct} onchange={(e) => commitScale(e.currentTarget)} onkeydown={enter} />
  </label>
  <label title="Поворот рамки вокруг центра, градусы по часовой">
    <span>Поворот, °</span>
    <input type="number" min="-180" max="180" step="1" value={b.box.rot} onchange={(e) => commitRot(e.currentTarget)} onkeydown={enter} />
  </label>
</div>
<div class="btns">
  <button class:on={b.flipH} aria-pressed={b.flipH} title="Зеркало по горизонтали" onclick={() => flipBackdrop(spread, b.scope, 'h')}
    ><Icon name="flip-h" size={14} />↔</button
  >
  <button class:on={b.flipV} aria-pressed={b.flipV} title="Зеркало по вертикали" onclick={() => flipBackdrop(spread, b.scope, 'v')}
    ><Icon name="flip-v" size={14} />↕</button
  >
  <button title="Рамка во всю область, без поворота, кадр сброшен" onclick={() => fitBackdrop(spread, b.scope)}>По области</button>
</div>
<div class="btns">
  <button class="done" onclick={() => bgmode.exit()}>Готово</button>
  <button class="ghost" title="Убрать фото фона (Del)" onclick={() => bgmode.remove()}>Убрать</button>
</div>

<style>
  .fields {
    display: flex;
    gap: 8px;
    margin-top: 6px;
  }
  label {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 12px;
    color: #cfcfd4;
  }
  input {
    width: 100%;
    box-sizing: border-box;
    height: 24px;
    padding: 0 4px;
    font-size: 12px;
  }
  .btns {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }
  .btns button {
    flex: 1;
    height: 24px;
    padding: 0 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
    font-size: 12px;
  }
  .btns button.on {
    color: var(--accent);
  }
  .done {
    background: var(--accent);
    color: #1e1e1e;
    font-weight: 600;
  }
  .done:hover:not(:disabled) {
    background: #ffe53a;
  }
</style>
