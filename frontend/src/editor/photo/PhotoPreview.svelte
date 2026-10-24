<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../../base/core/store.svelte'
  import { photoui } from './photoui.svelte'
  import { caption } from './previewlist'
  import { eventChord } from '../../base/keys/chord'
  import { keymap } from '../../base/keys/keymap.svelte'
  import { modalStack } from '../../base/modal/modalstack.svelte'
  import { markOf } from '../../panels/marks/marks'
  import { stars } from '../../panels/marks/thumbmark'
  import { previewUrl } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { fitTilt } from '../../base/geom/frame'
  import { prefs } from '../../base/prefs/prefs.svelte'
  import { isMac } from '../../base/core/platform'
  import { ZOOM_STEP } from '../zoom/zoom'
  import { hintLine } from './loupe'
  import { PreviewLoupe } from './loupe.svelte'
  import { adjlive } from './adjlive.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import MissPhoto from '../../base/ui/MissPhoto.svelte'
  import { missTip } from '../../panels/library/missing'
  import { autocheck } from '../../panels/folders/autocheck'

  // Крупный просмотр фото (Quick Preview из SmartAlbums). Открывают: Ctrl+правый щелчок по плитке, пункт меню, клавиша (128).
  // Esc, щелчок по фону или ✕ закрывают; ← → листают ленту; лупа (Л): щелчок или клавиша — 1:1 в точке, Ctrl+колесо — зум. Окно в modalStack: клавиши программы под ним молчат,
  // кроме отмены и (из ленты) отметок показанному фото — их пускает диспетчер (shell/menu/keys.ts).
  const st = $derived(photoui.preview)
  const id = $derived(photoui.previewId)
  const lib = $derived(st?.from === 'library')
  const photo = $derived(id ? store.byId.get(id) : undefined)
  const mark = $derived(photo ? markOf(photo, store.marks[photo.id]) : null)
  let box = $state<HTMLElement>()
  // Поворот фото в рамке (010 U5): с холста просмотр показывает фото так, как оно стоит в альбоме.
  // Из ленты — исходник без поворота, наклона и тона (0.26-128-9), даже если фото стоит на текущем развороте.
  const fv = $derived(id && !lib ? store.spread?.frames?.[store.spread.photos.indexOf(id)] : undefined)
  const tone = $derived(id && !lib ? adjlive.filterOf(id, fv) : undefined) // ч/б и тон (068): просмотр показывает фото как в альбоме
  const turned = $derived((fv?.turn ?? 0) % 2 === 1)
  let nat = $state({ w: 0, h: 0 }) // размер картинки просмотра, px
  // Файла нет (088): Go уже сообщил (store.missing) или картинка не загрузилась — заглушка вместо битой картинки.
  let failed = $state(false)
  const gone = $derived(failed || (!!id && store.missing.includes(id)))
  const ratio = $derived(photo && photo.w > 0 && photo.h > 0 ? photo.w / photo.h : 1.5)
  const failTip = $derived(id ? missTip(id, { [id]: photo?.name ?? '' }) : '')
  function onFail() {
    failed = true
    autocheck.soon(true)
  }
  // Размер и сбой от прошлого фото не переносим.
  $effect(() => {
    if (id) {
      nat = { w: 0, h: 0 }
      failed = false
    }
  })
  // Пропорции после поворота на четверти; пока картинка не загружена — неизвестны, рисуем как есть.
  const aspect = $derived(nat.w > 0 && nat.h > 0 ? (turned ? nat.h / nat.w : nat.w / nat.h) : 0)
  const rotated = $derived(!!fv?.r && aspect > 0)
  const k = $derived(fitTilt(fv?.tilt ?? 0, aspect || 1)) // наклонённое фото не вылезает из рамки просмотра

  // Лупа: размер кадра в px превью с учётом четверти поворота (рамка лупы — пропорции повёрнутого фото).
  const eff = $derived(turned ? { w: nat.h, h: nat.w } : nat)
  const guess = $derived({ w: ratio * 1000, h: 1000 }) // до загрузки — по данным фото, только для рамки
  const loupe = new PreviewLoupe(() => ({ size: eff, guess, off: gone }))
  const at = $derived(loupe.at)
  // Загрузился кадр: зум и точка переносятся при тех же пропорциях, иначе вписано.
  $effect(() => {
    if (eff.w > 0 && eff.h > 0) untrack(() => loupe.loaded(eff))
  })
  // Окно закрыто — лупа сбрасывается: следующее открытие начинается с «вписано».
  $effect(() => {
    if (!st) untrack(() => loupe.reset())
  })
  const mod = isMac() ? '⌘' : 'Ctrl'
  const hint = $derived(keymap.text(hintLine(lib, mod, prefs.can('marks'))))
  // Колесо: слушатель с passive: false, иначе preventDefault не сработает и Ctrl+колесо масштабирует страницу.
  let back = $state<HTMLElement>()
  $effect(() => {
    const node = back
    if (!node) return
    const wheel = (e: WheelEvent) => loupe.wheel(e)
    node.addEventListener('wheel', wheel, { passive: false })
    return () => node.removeEventListener('wheel', wheel)
  })

  // Снимок пропал из альбома (правка за окном) — закрываем.
  $effect(() => {
    if (id && !store.byId.has(id)) photoui.closePreview()
  })
  $effect(() => {
    if (box) box.focus()
  })
  // Окно — в стек, пока оно открыто (любой путь закрытия освобождает его): буфер и клавиши программы под ним молчат.
  const open = $derived(!!st)
  $effect(() => {
    if (!open) return
    const mid = untrack(() => modalStack.push(true))
    return () => modalStack.remove(mid)
  })

  const close = () => photoui.closePreview()

  // Свои клавиши окна: Esc, ← →, а также «Крупный просмотр» (1:1), «Вписать» и масштаб — они не дойдут до холста под окном.
  // Остальное идёт диспетчеру: отмена и отметки (shell/menu/keys.ts).
  const OWN = ['photoPreview', 'fitZoom', 'zoomIn', 'zoomOut']
  function onKey(e: KeyboardEvent) {
    if (!id) return
    const own = () => {
      e.preventDefault()
      e.stopPropagation()
    }
    if (e.key === 'Escape') {
      own()
      close()
      return
    }
    if ((e.key === 'ArrowLeft' || e.key === 'ArrowRight') && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      own()
      photoui.step(e.key === 'ArrowRight' ? 1 : -1)
      return
    }
    const chord = eventChord(e)
    const cmd = chord ? OWN.find((c) => keymap.keys(c).includes(chord)) : undefined
    if (!cmd) return
    own()
    loupeKey(cmd, e.repeat)
  }
  // Команда лупы с клавиатуры: «Крупно» — 1:1 (автоповтор не мигает), «Вписать», масштаб шагом холста.
  function loupeKey(cmd: string, repeat: boolean) {
    if (cmd === 'photoPreview') {
      if (!repeat) loupe.toggleAt()
    } else if (cmd === 'fitZoom') loupe.fit()
    else loupe.zoomBy(cmd === 'zoomIn' ? ZOOM_STEP : 1 / ZOOM_STEP)
  }

  // Нажатие в окне просмотра: по кадру — жест лупы, по пустому месту вокруг — закрыть (как щелчок по фону).
  function stageDown(e: PointerEvent) {
    if (e.target === e.currentTarget) close()
    else if ((e.target as Element).closest('.frame')) loupe.down(e)
  }
</script>

<svelte:window onkeydowncapture={onKey} />

{#if id}
  <div class="back" role="presentation" bind:this={back} onpointerdown={(e) => e.target === e.currentTarget && close()} data-chrome>
    <div class="box" role="dialog" aria-modal="true" aria-label="Крупный просмотр" tabindex="-1" bind:this={box}>
      <div
        class="view"
        role="presentation"
        bind:this={loupe.stage}
        bind:clientWidth={loupe.box.w}
        bind:clientHeight={loupe.box.h}
        onpointerdown={stageDown}
        onpointermove={(e) => loupe.move(e)}
        onpointerup={(e) => loupe.up(e)}
        onpointercancel={() => loupe.cancel()}
      >
        {#if gone}
          <div class="gone" style:--a={ratio}><MissPhoto size={48} tip={failTip} /></div>
        {:else}
          <!-- Рамка лупы: размер и место из loupe.ts (width/height и сдвиг, не scale — браузер пересчитывает картинку чётко) -->
          <div
            class="frame"
            class:turned={rotated}
            class:canzoom={loupe.canZoom && !loupe.zoomed}
            class:zoomed={loupe.zoomed}
            class:panning={loupe.panning}
            style:width="{at.w}px"
            style:height="{at.h}px"
            style:transform="translate({at.left}px, {at.top}px)"
          >
            {#if rotated}
              <!-- Картинка внутри рамки в долях рамки (cq): при повороте на 90° её ширина — высота рамки -->
              <img
                class="ph raw"
                src={previewUrl(id)}
                alt={photo?.name ?? ''}
                draggable="false"
                style:width={turned ? '100cqh' : '100cqw'}
                style:height={turned ? '100cqw' : '100cqh'}
                style:transform="translate(-50%, -50%) rotate({fv?.r}deg) scale({k})"
                style:filter={tone}
                onerror={onFail}
                {@attach cancelOnUnmount}
              />
            {:else}
              <img
                class="ph"
                src={previewUrl(id)}
                alt={photo?.name ?? ''}
                draggable="false"
                bind:naturalWidth={nat.w}
                bind:naturalHeight={nat.h}
                style:filter={tone}
                onerror={onFail}
                {@attach cancelOnUnmount}
              />
            {/if}
          </div>
        {/if}
      </div>
      <div class="cap">
        <span class="name">{st ? caption(photo?.name ?? '', st) : ''}</span>
        {#if lib && mark}
          <span class="mk">
            {#if mark.rating > 0}<span class="stars">{stars(mark.rating)}</span>{/if}
            {#if mark.flag === 1}<Icon name="flag" size={14} />{:else if mark.flag === -1}<Icon name="flag-off" size={14} />{/if}
            {#if mark.label !== ''}<i class="dot {mark.label}"></i>{/if}
          </span>
        {/if}
        {#if loupe.label}<span class="zoom">{loupe.label}</span>{/if}
        <button class="x" onclick={close} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
      </div>
      <div class="hint">{hint}</div>
    </div>
  </div>
{/if}

<style>
  .back {
    position: fixed;
    inset: 0;
    z-index: 85;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.82);
  }
  .box {
    position: relative;
    display: grid;
    grid-template-rows: 1fr 36px auto;
    width: 94vw;
    height: 94vh;
    outline: none;
  }
  /* Окно лупы: кадр внутри него вписан или увеличен, лишнее обрезается */
  .view {
    position: relative;
    min-height: 0;
    overflow: hidden;
    touch-action: none;
  }
  /* Рамка кадра: размер и сдвиг задаёт лупа; контейнер размеров (cq) нужен повёрнутой картинке */
  .frame {
    position: absolute;
    left: 0;
    top: 0;
    container-type: size;
  }
  .frame.canzoom {
    cursor: zoom-in;
  }
  .frame.zoomed {
    cursor: grab;
  }
  .frame.panning {
    cursor: grabbing;
  }
  .ph {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: contain;
    user-select: none;
  }
  /* Заглушка «нет файла» в пропорциях фото; вписана в окно */
  .gone {
    position: absolute;
    inset: 0;
    margin: auto;
    height: min(100%, 70vh);
    max-width: 100%;
    aspect-ratio: var(--a, 1);
  }
  .ph.raw {
    position: absolute;
    left: 50%;
    top: 50%;
    max-width: none;
    max-height: none;
    margin: 0;
    object-fit: fill;
  }
  .cap {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    height: 36px;
    color: var(--muted);
    font-size: 13px;
  }
  .cap .name {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .zoom {
    margin-left: auto;
    padding: 2px 8px;
    border-radius: 4px;
    background: var(--line);
    color: var(--text);
    white-space: nowrap;
  }
  .hint {
    color: var(--muted);
    font-size: 12px;
    padding-bottom: 2px;
  }
  /* Отметки показанного фото (128): звёзды, флажок, цвет метки */
  .mk {
    flex: 1;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #fff;
  }
  .stars {
    letter-spacing: 1px;
  }
  .dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;
    background: var(--lbl-other);
  }
  .dot.red {
    background: var(--lbl-red);
  }
  .dot.yellow {
    background: var(--lbl-yellow);
  }
  .dot.green {
    background: var(--lbl-green);
  }
  .dot.blue {
    background: var(--lbl-blue);
  }
  .dot.purple {
    background: var(--lbl-purple);
  }
  .x {
    flex: none;
    width: 26px;
    height: 26px;
    padding: 0;
    display: grid;
    place-items: center;
    background: transparent;
    color: var(--text);
  }
</style>
