<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { framelive } from '../photo/framelive.svelte'
  import { profileLine } from '../photo/color'
  import { DEFAULT_FRAME, isDefaultFrame, zoomFrame } from '../../base/geom/frame'
  import { liveTilt, photoTilt } from '../tilt/tiltlive'
  import type { Frame } from '../../base/core/types'
  import Icon from '../../base/ui/Icon.svelte'
  import { ownLabel } from './ownlabel'
  import PlaceFields from './PlaceFields.svelte'
  import FitToggle from '../photoprops/FitToggle.svelte'
  import ShapeToggle from '../photoprops/ShapeToggle.svelte'
  import SnapToggle from '../photoprops/SnapToggle.svelte'
  import TurnFlipRow from '../photoprops/TurnFlipRow.svelte'
  import AdjustRows from '../photoprops/AdjustRows.svelte'
  import PhotoFade from '../border/PhotoFade.svelte'
  import InspectorBorder from '../border/InspectorBorder.svelte'
  import LayerRow from './LayerRow.svelte'
  import TiltRow from '../photoprops/TiltRow.svelte'
  import PhotoAbout from '../photoprops/PhotoAbout.svelte'
  import { straighten } from '../tilt/straighten.svelte'
  import { placesel } from '../places/placesel.svelte'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import PersonFields from '../../panels/names/PersonFields.svelte'
  import CardGroup from './CardGroup.svelte'
  import { aboutSum, frameSum, placeSum, shapeSum, toneSum } from './groupsum'
  import { toneOf } from '../photoprops/adjustrows'
  import { borderRows, fadeRow, targetsOf } from '../border/borderrows'
  import { decor } from '../../panels/decor/decor.svelte'
  import { shapeKey } from '../../base/geom/shape'

  // Карточка выделенного фото (спека 006 → 008 → 124): имя файла и справа dpi, дальше сворачиваемые группы — «Кадр»
  // (масштаб 100–400 % с ⊗ сброса, наклон, «Целиком / С обрезкой», «Повернуть · Зеркало»), «Цвет и тон», «Форма и рамочка»,
  // «Имя на фото», «Место» (X/Y/Ш/В с замком), «О снимке» (px, профиль, съёмка). Ползунок ведёт предпросмотр в Cell через
  // framelive, SetFrame — при отпускании. Что видно на уровне, решает groupShown (groupsum.ts) теми же prefs.can, что и тело.
  const sp = $derived(store.spread)
  const id = $derived(store.selectedCell ?? '')
  const i = $derived(sp ? sp.photos.indexOf(id) : -1)
  const fv = $derived(i >= 0 ? sp?.frames?.[i] : undefined)
  const frame = $derived<Frame>(fv ? { s: fv.s, x: fv.x, y: fv.y, r: fv.r } : DEFAULT_FRAME)
  const cur = $derived(framelive.of(id) ?? frame)
  const contain = $derived(!!sp?.grid && sp.places?.find((p) => p.photo === id)?.fit === 'contain')
  const name = $derived(store.byId.get(id)?.name ?? 'Фото')
  // Свой кадр (098) действует только на «Авто»: на ручном месте место задаёт пропорции, строки нет
  const own = $derived(sp?.grid ? 0 : (fv?.own ?? 0))
  const dpi = $derived(fv?.dpi ?? 0)
  const info = $derived(store.byId.get(id))
  const px = $derived(info && info.w > 0 && info.h > 0 ? `${info.w}×${info.h} px` : '')
  const minDPI = $derived(store.view.minDPI)
  const pct = $derived(Math.round(frame.s * 100))
  const tilt = $derived(framelive.angleOf(id) ?? fv?.tilt ?? 0)
  const profile = $derived(profileLine(store.byId.get(id)))

  // Масштаб: TiltRow ведёт предпросмотр через framelive, SetFrame — на отпускании (onend гасит просмотр)
  async function zoom(pct: number) {
    const f = zoomFrame(frame, pct / 100)
    framelive.set(id, f) // кадр остаётся до ответа Go, силуэт гаснет сразу на отпускании
    await store.run(Api.SetFrame(id, f.s, f.x, f.y))
  }
  // Наклон (010 U5 → 045 → 057): строка TiltRow ведёт предпросмотр на ячейке, SetFrameAngle уходит на отпускании.
  // Наклон держит приближение (057): кадр считает liveTilt от кадра начала сессии — тот же расчёт, что у Go.
  // Сессия (ключ merge в Go) живёт, пока фокус в строке: несколько протяжек — один шаг отмены. Четверти разложил Go.
  const tiltLive = (t: number) => {
    if (!sp) return
    const f = liveTilt(sp, id, photoTilt.keep(id, frame, store.view.rev).base, t)
    if (f) framelive.set(id, f, true)
  }
  async function tiltCommit(t: number) {
    if (!sp) return
    await store.run(Api.SetFrameAngle(sp.id, id, t, photoTilt.keep(id, frame, store.view.rev).id))
    photoTilt.settle(store.view.rev)
  }
  // Смена фото или уход панели закрывают сессию.
  $effect(() => {
    void id
    return () => photoTilt.end()
  })
  // Значения свёрнутых групп (groupsum.ts): считаются из тех же данных, что и строки тела.
  const sample = $derived(placesel.selected[0])
  const border = $derived(
    borderRows(
      targetsOf(
        sp,
        placesel.selected.map((b) => b.id),
      ),
      { missing: store.missing, nameOf: (lib) => decor.frameItem(lib)?.name },
    ),
  )
  const tone = $derived(toneOf(fv))
  const sums = $derived({
    frame: frameSum({ pct, contain, tilt }),
    tone: toneSum({ ...tone, fade: fadeRow(sp, [id]).pct }),
    shape: sample ? shapeSum({ shape: shapeKey(sample), border: border?.name ?? '' }) : '',
    place: sample ? placeSum(sample) : '',
    about: aboutSum({ w: info?.w ?? 0, h: info?.h ?? 0 }),
  })
  let nameText = $state('')
  const lowDpi = $derived(dpi > 0 && dpi < minDPI)
  // Подсказка dpi: размер снимка в px (в строке его нет) и порог печати
  const dpiTip = $derived([px, lowDpi ? `Нужно от ${minDPI} dpi` : 'dpi при печати'].filter(Boolean).join(' · '))
  const showPlace = $derived(prefs.can('manual') || (!!sp?.grid && prefs.can('layers')))
  async function reset() {
    await store.run(Api.ResetFrame(id))
    framelive.clear()
  }
</script>

<div class="top">
  <div class="name" title={name}>{name}</div>
  {#if dpi > 0}<div class="dpi" class:low={lowDpi} title={dpiTip}>≈ {dpi} dpi</div>{/if}
</div>

<CardGroup id="photo.frame" title="Кадр" summary={sums.frame}>
  {#if !prefs.can('frame')}
    <!-- «Простой»: кадр и «Целиком / С обрезкой» не показываются -->
  {:else if contain}
    <div class="note">Вписано целиком</div>
  {:else}
    <TiltRow
      tilt={Math.round(frame.s * 100)}
      label="Масштаб, %"
      numLabel="Масштаб кадра, процент"
      rangeLabel="Масштаб кадра"
      rangeTitle="Масштаб кадра; двойной щелчок — 100 %"
      min={100}
      max={400}
      step={1}
      guide=""
      id="zoom"
      dflt={100}
      reset
      resetLabel="Сбросить кадр"
      resetTitle="Сбросить кадр: фото целиком в месте"
      resetOff={isDefaultFrame(cur) && !own}
      onlive={(v) => framelive.setZoom(id, zoomFrame(frame, v / 100))}
      oncommit={zoom}
      onreset={reset}
      onend={() => framelive.clear()}
    />
    {#if own}
      <!-- Свой кадр (098): раскладки «Авто» строятся под эти пропорции; сброс снимает и кадр, и пропорции -->
      <div class="row own" data-testid="own-crop">
        <span class="lbl">Свой кадр {ownLabel(own)}</span>
        <button class="reset" onclick={reset} aria-label="Сбросить кадр" title="Сбросить кадр: фото снова целое, раскладка подстроится"
          ><Icon name="reset" size={16} /></button
        >
      </div>
    {/if}
  {/if}
  {#if prefs.can('tilt')}
    <TiltRow
      {tilt}
      reset
      onlive={tiltLive}
      oncommit={tiltCommit}
      onend={() => framelive.clear()}
      onleave={() => photoTilt.end()}
      onstraighten={() => straighten.startPhoto(id)}
    />
  {/if}
  {#if prefs.can('frame')}<FitToggle />{/if}
  <!-- Последняя строка «Кадра»; сразу за ней «Цвет | Ч/б» (владелец 2026-10-18, страж photocard.order.dom.test.ts) -->
  {#if prefs.can('turn') || prefs.can('flip')}<TurnFlipRow {id} />{/if}
</CardGroup>

{#if fv}
  <CardGroup id="photo.tone" title="Цвет и тон" summary={sums.tone}>
    {#if prefs.can('adjust')}<AdjustRows {id} />{/if}
    {#if prefs.can('photoFade')}<PhotoFade photos={[id]} />{/if}
  </CardGroup>
{/if}

<CardGroup id="photo.shape" title="Форма и рамочка" summary={sums.shape}>
  {#if prefs.can('frame')}<ShapeToggle />{/if}
  {#if prefs.can('border')}<InspectorBorder />{/if}
</CardGroup>

{#if prefs.can('captions') && id}
  <!-- Имя для полей {Фамилия}… в подписях (030); по фото пересоздаётся: набранное уходит на своё фото -->
  <CardGroup id="photo.name" title="Имя на фото" summary={nameText}>
    {#key id}<PersonFields photo={id} row cap={false} onsummary={(t) => (nameText = t)} />{/key}
  </CardGroup>
{/if}

{#if showPlace}
  <CardGroup id="photo.place" title="Место" summary={sums.place}>
    {#if prefs.can('manual')}
      <PlaceFields />
      <SnapToggle />
    {/if}
    {#if sp?.grid && prefs.can('layers')}
      <LayerRow onpick={(op) => placesel.layer(op)} />
    {/if}
  </CardGroup>
{/if}

<CardGroup id="photo.about" title="О снимке" summary={sums.about}>
  {#if px}<div class="px">{px}</div>{/if}
  {#if profile}
    <div class="profile" class:warn={profile.level === 'warn'} title={profile.title}>{profile.text}</div>
  {/if}
  {#if id}<div data-guide="photo-about"><PhotoAbout {id} /></div>{/if}
</CardGroup>

<style>
  .top {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .name {
    flex: 1;
    min-width: 0;
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
  .own {
    margin-top: 8px;
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
  .dpi {
    flex: none;
    font-size: 12px;
    color: var(--muted);
  }
  .px {
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
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
