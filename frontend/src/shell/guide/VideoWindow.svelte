<script lang="ts">
  import Modal from '../../base/modal/Modal.svelte'
  import ProgressRun from '../../base/ui/ProgressRun.svelte'
  import { guideVideoUrl } from '../../base/core/media'
  import { blobType, busyLine, percent } from './videorow'
  import { videos } from './videos.svelte'

  // Окно ролика (спека 140, design §4): Modal 960 над справочником, Esc закрывает только его (modalstack).
  // Файл берём fetch целиком и играем Blob: прямой <video src="/media/…"> в окне Linux не играет (0.30-140-19).
  // Первый раз ролика в кэше нет: пока он грузится (videos.loading), окно показывает ход и «Отменить» (0.30-140-31), потом играет.
  // Blob отзывается при закрытии. Не открылось (сеть Wails, тип, декодер) — onfail: окно закрывается, в карточке «Показать файл».
  let { id, title, onclose, onfail }: { id: string; title: string; onclose: () => void; onfail: () => void } = $props()

  let src = $state('')
  let ratio = $state(1280 / 760) // до метаданных — как кадр записи (окно 1280×760)

  const loading = $derived(videos.loading === id)

  $effect(() => {
    if (loading) return
    const ac = new AbortController()
    let url = ''
    src = ''
    void (async () => {
      try {
        const res = await fetch(guideVideoUrl(id), { signal: ac.signal })
        const type = blobType(res.headers.get('Content-Type'))
        if (!res.ok || !type) return onfail()
        const blob = new Blob([await res.blob()], { type })
        if (ac.signal.aborted) return
        url = URL.createObjectURL(blob)
        src = url
      } catch {
        if (!ac.signal.aborted) onfail()
      }
    })()
    return () => {
      ac.abort()
      if (url) URL.revokeObjectURL(url)
    }
  })
</script>

<Modal title="Ролик: {title}" width={960} {onclose}>
  <div class="box" class:wait={loading} aria-busy={!src}>
    {#if loading}
      <!-- Ход и «Отменить» — та же полоса, что у загрузки набора (PackRun) -->
      <ProgressRun
        text={busyLine(videos.progress, id)}
        percent={percent(videos.progress, id)}
        label="Ход скачивания ролика"
        cancelling={videos.cancelling}
        autofocus
        oncancel={onclose}
      />
    {:else if src}
      <video
        {src}
        controls
        autoplay
        muted
        playsinline
        preload="auto"
        disablepictureinpicture
        data-autofocus
        style:aspect-ratio={ratio}
        onloadedmetadata={(e) => {
          const v = e.currentTarget
          if (v.videoWidth > 0 && v.videoHeight > 0) ratio = v.videoWidth / v.videoHeight
        }}
        onerror={onfail}
      ></video>
    {/if}
  </div>
</Modal>

<style>
  .box {
    min-height: 200px;
    background: #000;
    border-radius: 6px;
    overflow: hidden;
  }
  .box.wait {
    background: transparent;
    min-height: 0;
  }
  video {
    display: block;
    width: 100%;
    max-height: calc(100vh - 110px);
    object-fit: contain;
  }
</style>
