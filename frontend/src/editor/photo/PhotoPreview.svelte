<script lang="ts">
  import { store } from '../../base/core/store.svelte'
  import { photoui } from './photoui.svelte'
  import { previewUrl } from '../../base/core/media'
  import { cancelOnUnmount } from '../../base/ui/imgcancel'
  import { fitTilt } from '../../base/geom/frame'
  import { adjlive } from './adjlive.svelte'
  import Icon from '../../base/ui/Icon.svelte'
  import MissPhoto from '../../base/ui/MissPhoto.svelte'
  import { missTip } from '../../panels/library/missing'
  import { autocheck } from '../../panels/folders/autocheck'

  // Крупный просмотр фото (Quick Preview из SmartAlbums, но без клавиши: Пробел занят). Esc,
  // щелчок по фону или ✕ закрывают; остальные клавиши страницу под окном не трогают.
  const id = $derived(photoui.preview)
  const photo = $derived(id ? store.byId.get(id) : undefined)
  let box = $state<HTMLElement>()
  // Поворот фото в рамке (010 U5): просмотр показывает фото целиком, но так, как оно стоит в альбоме.
  const fv = $derived(id ? store.spread?.frames?.[store.spread.photos.indexOf(id)] : undefined)
  const tone = $derived(id ? adjlive.filterOf(id, fv) : undefined) // ч/б и тон (068): просмотр показывает фото как в альбоме
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

  // Снимок пропал из альбома (правка за окном) — закрываем.
  $effect(() => {
    if (id && !store.byId.has(id)) photoui.preview = null
  })
  $effect(() => {
    if (box) box.focus()
  })

  const close = () => (photoui.preview = null)
</script>

<svelte:window
  onkeydowncapture={(e) => {
    if (!id) return
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      close()
    }
  }}
/>

{#if id}
  <div class="back" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && close()} data-chrome>
    <div class="box" role="dialog" aria-modal="true" aria-label="Крупный просмотр" tabindex="-1" bind:this={box}>
      {#if gone}
        <div class="gone" style:--a={ratio}><MissPhoto size={48} tip={failTip} /></div>
      {:else if rotated}
        <!-- Рамка под пропорции повёрнутого на четверти фото; картинка внутри крутится на полный угол -->
        <div class="turned" style:--a={aspect}>
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
        </div>
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
      <div class="cap">
        <span>{photo?.name ?? ''}</span>
        <button class="x" onclick={close} title="Закрыть (Esc)" aria-label="Закрыть"><Icon name="x" size={14} /></button>
      </div>
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
    max-width: 94vw;
    max-height: 94vh;
    outline: none;
  }
  .ph {
    max-width: 94vw;
    max-height: calc(94vh - 36px);
    margin: auto;
    object-fit: contain;
    user-select: none;
  }
  /* Контейнер размеров (cq): картинка внутри задана в долях рамки, поэтому при повороте на 90° её ширина — высота рамки */
  .turned {
    position: relative;
    container-type: size;
    margin: auto;
    --h: min(calc(94vh - 36px), calc(94vw / var(--a, 1)));
    height: var(--h);
    width: calc(var(--h) * var(--a, 1));
  }
  /* Заглушка «нет файла» в пропорциях фото; вписана в ту же область, что и картинка */
  .gone {
    position: relative;
    margin: auto;
    --h: min(calc(94vh - 36px), 70vh, calc(94vw / var(--a, 1)));
    height: var(--h);
    width: calc(var(--h) * var(--a, 1));
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
  .cap span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
