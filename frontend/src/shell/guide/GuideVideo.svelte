<script lang="ts">
  import { tick } from 'svelte'
  import NetConsent from '../packs/NetConsent.svelte'
  import { CONSENT_DOWNLOAD } from '../packs/packnet'
  import { GUIDE_VIDEOS } from '../guidetext/videos'
  import VideoWindow from './VideoWindow.svelte'
  import { videos } from './videos.svelte'
  import { allLink, BROKEN_TEXT, busyLine, hasVideo, metaLine, percent, rowState, secondsLabel } from './videorow'

  // Строка «Ролик» карточки F1 (спека 140, design §4): скачать · ход · смотреть · не открылся (+ скачать заново). Образец вида — строки
  // «Где», «Клавиши»; вопрос о сети и ход — как у «Скачать» набора. Статья без ролика — строки нет (0.30-140-15).
  let { id, title }: { id: string; title: string } = $props()

  const item = $derived(videos.items.get(id))
  const known = $derived(hasVideo(id, GUIDE_VIDEOS, videos.items))
  const kind = $derived(rowState(id, item, videos.busy, videos.failed))
  const seconds = $derived(item?.seconds || GUIDE_VIDEOS[id]?.seconds || 0)
  const meta = $derived(metaLine(seconds, item?.size ?? 0))
  const all = $derived(allLink([...videos.items.values()]))
  const pct = $derived(percent(videos.progress, id))

  let watch = $state<HTMLButtonElement>()
  // Esc закрыл окно ролика: фокус — на «Смотреть» (окно под ним забирает его себе позже, поэтому после двух тактов)
  async function closeWindow() {
    videos.stop()
    await tick()
    await tick()
    watch?.focus()
  }
</script>

{#if known}
  <div class="row">
    <b>Ролик</b>
    <div class="line">
      {#if kind === 'get'}
        <span class="what">
          <button class="link" onclick={() => videos.get([id])}>Скачать</button>
          {#if meta}<span class="muted">{meta}</span>{/if}
          {#if all}<button class="link all" onclick={() => videos.get(all.ids)}>{all.label}</button>{/if}
        </span>
      {:else if kind === 'busy'}
        <span class="what">
          <span role="status">{videos.cancelling ? 'Отменяем…' : busyLine(videos.progress, id)}</span>
          <button class="link" disabled={videos.cancelling} onclick={() => videos.cancel()}>Отменить</button>
        </span>
        <progress max="100" value={pct ?? undefined} aria-label="Ход скачивания ролика"></progress>
      {:else if kind === 'ready'}
        <span class="what">
          <button class="watch" bind:this={watch} onclick={() => videos.play(id)}>▶ Смотреть</button>
          {#if seconds > 0}<span class="muted">· {secondsLabel(seconds)}</span>{/if}
        </span>
      {:else}
        <span class="what">
          <span>{BROKEN_TEXT}</span>
          <button class="link" onclick={() => videos.reveal(id)}>Показать файл</button>
          <!-- Файл в кэше испорчен при том же размере: Go сверит sha256 и заменит его (0.30-140-28) -->
          <button class="link" onclick={() => videos.get([id])}>Скачать заново</button>
        </span>
      {/if}
    </div>
  </div>
{/if}

{#if videos.playing === id}
  <VideoWindow {id} {title} onclose={closeWindow} onfail={() => videos.fail(id)} />
{/if}

{#if videos.consent}
  <!-- Первое сетевое действие: без согласия программа в сеть не ходит (тот же вопрос, что в «Наборах») -->
  <NetConsent
    action={CONSENT_DOWNLOAD}
    bind:start={videos.consentStart}
    onconfirm={() => videos.confirmConsent()}
    oncancel={() => videos.dismissConsent()}
  />
{/if}

<style>
  .row {
    display: grid;
    grid-template-columns: 72px 1fr;
    gap: 8px;
    color: var(--muted);
  }
  .row b {
    font-weight: 600;
    color: var(--text);
  }
  .line {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .what {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px 8px;
  }
  .muted {
    color: var(--muted);
  }
  .all {
    margin-left: 4px;
  }
  .link {
    height: auto;
    padding: 0;
    background: transparent;
    color: var(--accent);
    white-space: normal;
    text-align: left;
  }
  .link:disabled {
    opacity: 0.5;
  }
  .watch {
    height: auto;
    padding: 2px 8px;
  }
  progress {
    width: 100%;
    height: 6px;
    accent-color: var(--accent);
  }
</style>
