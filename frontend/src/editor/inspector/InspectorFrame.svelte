<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { framelive } from '../photo/framelive.svelte'
  import { profileLine } from '../photo/color'
  import { DEFAULT_FRAME, isDefaultFrame, rotateFrame, zoomFrame } from '../../base/geom/frame'
  import type { Frame } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'
  import PlaceFields from './PlaceFields.svelte'
  import FitToggle from './FitToggle.svelte'
  import SnapToggle from './SnapToggle.svelte'
  import FlipToggle from './FlipToggle.svelte'
  import LayerRow from './LayerRow.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'

  // Карточка выделенного фото (спека 006 → 008): имя файла, рамка места X/Y/Ш/В с замком, масштаб кадра
  // 100–400 % с ⊗ сброса, «Целиком / С обрезкой», dpi и размер в px, профиль (не sRGB). Ползунок ведёт предпросмотр в Cell через framelive, SetFrame — при отпускании.
  const sp = $derived(store.spread)
  const id = $derived(store.selectedCell ?? '')
  const i = $derived(sp ? sp.photos.indexOf(id) : -1)
  const fv = $derived(i >= 0 ? sp?.frames?.[i] : undefined)
  const frame = $derived<Frame>(fv ? { s: fv.s, x: fv.x, y: fv.y, r: fv.r } : DEFAULT_FRAME)
  const cur = $derived(framelive.of(id) ?? frame)
  const contain = $derived(!!sp?.grid && sp.places?.find((p) => p.photo === id)?.fit === 'contain')
  const name = $derived(store.byId.get(id)?.name ?? 'Фото')
  const dpi = $derived(fv?.dpi ?? 0)
  const info = $derived(store.byId.get(id))
  const px = $derived(info && info.w > 0 && info.h > 0 ? `${info.w}×${info.h} px` : '')
  const minDPI = $derived(store.view.minDPI)
  const pct = $derived(Math.round(cur.s * 100))
  const profile = $derived(profileLine(store.byId.get(id)))

  const value = (e: Event) => +(e.currentTarget as HTMLInputElement).value / 100
  async function change(e: Event) {
    const f = zoomFrame(frame, value(e))
    await store.run(Api.SetFrame(id, f.s, f.x, f.y))
    framelive.clear()
  }
  // Наклон (010 U5): ползунок и поле ведут предпросмотр на ячейке, SetFrameAngle уходит на отпускании
  // и по Enter / уходу из поля. Четверти и наклон разложил Go (turn, tilt); R/90 здесь не считается.
  const MAX_TILT = 44.9
  let draft = $state<number | null>(null) // что сейчас показано в поле, пока тянут
  const tilt = $derived(draft ?? fv?.tilt ?? 0)
  // Пустое поле или «-» (valueAsNumber — NaN): ввод не закончен, поле и draft не трогаем, иначе минус не набрать.
  const tiltOf = (e: Event) => (e.currentTarget as HTMLInputElement).valueAsNumber
  const clampTilt = (v: number) => Math.min(MAX_TILT, Math.max(-MAX_TILT, Math.round(v * 10) / 10))
  function tiltLive(e: Event) {
    const v = tiltOf(e)
    if (!Number.isFinite(v) || !fv) return
    draft = clampTilt(v)
    framelive.set(id, rotateFrame(frame, (fv.turn ?? 0) * 90 + draft), true)
  }
  async function tiltCommit(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    const v = tiltOf(e)
    if (Number.isFinite(v) && fv && clampTilt(v) !== fv.tilt) await store.run(Api.SetFrameAngle(id, clampTilt(v)))
    // Недонабранное («-», пусто) откатываем к значению из Go, иначе поле осталось бы пустым.
    else if (!Number.isFinite(v)) input.value = String(fv?.tilt ?? 0)
    draft = null // после ответа Go: иначе ползунок на миг прыгнет назад
    framelive.clear()
  }
  async function turn(quarters: -1 | 1) {
    await store.run(Api.TurnPhotos([id], quarters))
  }
  async function reset() {
    await store.run(Api.ResetFrame(id))
    framelive.clear()
  }
</script>

<div class="name" title={name}>{name}</div>
{#if prefs.can('manual')}
  <PlaceFields />
  <SnapToggle />
{/if}
<div class="gap"></div>
{#if !prefs.can('frame')}
  <!-- «Простой»: кадр и «Целиком / С обрезкой» не показываются -->
{:else if contain}
  <div class="note">Вписано целиком</div>
{:else}
  <div class="row">
    <span class="lbl">Масштаб</span>
    <span class="pct">{pct} %</span>
    <button
      class="reset"
      disabled={isDefaultFrame(cur)}
      onclick={reset}
      aria-label="Сбросить кадр"
      title="Сбросить кадр: фото целиком в месте"><Icon name="reset" size={16} /></button
    >
  </div>
  <input
    type="range"
    min="100"
    max="400"
    step="1"
    value={pct}
    aria-label="Масштаб кадра"
    oninput={(e) => framelive.set(id, zoomFrame(frame, value(e)))}
    onchange={change}
    onpointerup={(e) => e.currentTarget.blur()}
  />
{/if}
{#if prefs.can('turn')}
  <div class="row turn">
    <span class="lbl">Повернуть</span>
    <button class="reset" onclick={() => turn(-1)} aria-label="Повернуть на 90° против часовой" title="↺ 90°"
      ><Icon name="turn-ccw" size={16} /></button
    >
    <button class="reset" onclick={() => turn(1)} aria-label="Повернуть на 90° по часовой" title="↻ 90°"
      ><Icon name="turn-cw" size={16} /></button
    >
  </div>
{/if}
{#if prefs.can('tilt')}
  <div class="row tilt">
    <label class="lbl" for="tilt-num">Наклон, °</label>
    <input
      id="tilt-num"
      class="num"
      type="number"
      min={-MAX_TILT}
      max={MAX_TILT}
      step="0.1"
      value={tilt}
      aria-label="Наклон, градусы"
      oninput={tiltLive}
      onchange={tiltCommit}
    />
  </div>
  <input
    type="range"
    min={-MAX_TILT}
    max={MAX_TILT}
    step="0.1"
    value={tilt}
    aria-label="Наклон"
    title="Наклон фото в рамке; двойной щелчок — выровнять"
    oninput={tiltLive}
    onchange={tiltCommit}
    ondblclick={(e) => {
      e.currentTarget.value = '0'
      tiltLive(e)
      tiltCommit(e)
    }}
    onpointerup={(e) => e.currentTarget.blur()}
  />
{/if}
{#if prefs.can('frame')}<FitToggle />{/if}
{#if prefs.can('flip')}<FlipToggle />{/if}
{#if sp?.grid && prefs.can('layers')}
  <LayerRow onpick={(op) => placesel.layer(op)} />
{/if}
{#if dpi > 0 || px}
  <div
    class="dpi"
    class:low={dpi > 0 && dpi < minDPI}
    title={dpi > 0 && dpi < minDPI ? `Нужно от ${minDPI} dpi` : 'dpi при печати и размер снимка'}
  >
    {[dpi > 0 ? `≈ ${dpi} dpi` : '', px].filter(Boolean).join(' · ')}
  </div>
{/if}
{#if profile}
  <div class="profile" class:warn={profile.level === 'warn'} title={profile.title}>{profile.text}</div>
{/if}

<style>
  .name {
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-bottom: 8px;
  }
  .note {
    font-size: 12px;
    color: var(--muted);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .lbl {
    color: var(--muted);
    flex: 1;
  }
  .pct {
    font-variant-numeric: tabular-nums;
  }
  .turn {
    margin-top: 8px;
  }
  .tilt {
    margin-top: 8px;
  }
  .num {
    width: 64px;
    height: 24px;
    padding: 0 4px;
    box-sizing: border-box;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .reset {
    width: 24px;
    height: 24px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: #cfcfd4;
    border-radius: 4px;
  }
  input[type='range'] {
    width: 100%;
    margin: 6px 0 0;
  }
  .dpi {
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
  }
  .gap {
    height: 8px;
  }
  .profile {
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .profile.warn {
    color: #e0803a;
    font-weight: 600;
  }
  .dpi.low {
    color: #ff6b5e;
    font-weight: 600;
  }
</style>
