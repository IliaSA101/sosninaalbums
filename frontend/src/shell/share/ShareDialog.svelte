<script lang="ts">
  import { untrack } from 'svelte'
  import { EventsOn } from '../../../wailsjs/runtime/runtime'
  import Modal from '../../base/modal/Modal.svelte'
  import { store } from '../../base/core/store.svelte'
  import ShareConnect from './ShareConnect.svelte'
  import ShareForm from './ShareForm.svelte'
  import ShareLink from './ShareLink.svelte'
  import ShareRun from './ShareRun.svelte'
  import { share } from './share.svelte'
  import { hasLink, loadDays, screenOf } from './share'
  import type { ShareDone, ShareProgress } from './shareapi'
  import {
    AGAIN_LABEL,
    BUSY_OTHER,
    CANCEL_LABEL,
    CLOSE_LABEL,
    CONTINUE_LABEL,
    EXPIRED_LINE,
    FORGET_LABEL,
    GONE_LINE,
    INCOMPLETE_LINE,
    SAVE_LABEL,
    SESSION_NOTE,
    WAIT_LINE,
    SHARE_TITLE,
    otherLine,
    serviceLine,
  } from './sharetext'

  // Окно «Файл ▸ Отправить по ссылке…» (146, design §5). Образец — «Экспорт»: Modal шириной 560 как у экспорта (одна на всех экранах), «Отмена» слева от основной, ход как ExportRun.
  // Смонтировано всегда: события share:* нужны и при закрытом окне (тост «Ссылка готова»). Что показать — screenOf (share.ts), решает Go.
  $effect(() => {
    const off = [
      EventsOn('share:progress', (p: ShareProgress) => share.progressed(p)),
      EventsOn('share:done', (d: ShareDone) => void share.finished(d)),
    ]
    return () => off.forEach((f) => f?.())
  })
  // Правка альбома при открытом окне меняет «изменён после выгрузки» и сводку: перечитываем (без сети).
  $effect(() => {
    void store.view.rev
    if (share.open) untrack(() => void share.refresh())
  })

  // Другой альбом при открытой странице: сброс серых кнопок и перечитывание вида (share.watchAlbum).
  $effect(() => {
    const key = `${store.openSeq}|${store.view.path}`
    untrack(() => share.watchAlbum(key))
  })

  const v = $derived(share.view)
  const screen = $derived(v ? screenOf(v) : null)
  const link = $derived(v?.link ?? null)
  const working = $derived(share.working)
  // «Продолжить»: только days 0 — название, пароль и срок служба берёт из записи (у недоделанной выгрузки пароль в записи мог ещё не появиться, T5-6).
  const resume = () => share.start({ days: 0 })
  // «Выгрузить заново» после срока: новая ссылка с названием и паролем записи (она опубликована, пароль настоящий), срок — из последнего выбора.
  const again = () => link && share.start({ title: link.title, password: link.password, days: loadDays(v?.service.days) })
  // Ссылка другого сервиса: «Выгрузить заново» открывает форму (название и пароль записи), вопрос «будет новая ссылка» задаёт Go (T5-8).
  let redo = $state(false)
  $effect(() => {
    if (screen !== 'other') redo = false
  })
</script>

{#if share.open}
  <Modal title={SHARE_TITLE} onclose={() => share.close()} width={560}>
    {#if v && screen}
      {#if v.service.host && v.service.token && screen !== 'run'}
        <div class="svc">
          <span>{serviceLine(v.service.host)}</span>
          <span class="grow"></span>
          <button disabled={working} onclick={() => share.forget()}>{FORGET_LABEL}</button>
        </div>
        {#if v.service.store === 'session'}<p class="hint">{SESSION_NOTE}</p>{/if}
      {/if}
      <!-- Причина, не отменяющая готовую ссылку (T5-7): копировать и отозвать можно, выгрузить заново — нет -->
      {#if v.album.block && !['blocked', 'busyOther', 'run'].includes(screen)}
        <p class="note" role="status">{v.album.block}</p>
      {/if}

      {#if screen === 'busyOther'}
        <p class="note" role="status">{BUSY_OTHER}</p>
        <div class="actions"><button data-autofocus onclick={() => share.close()}>{CLOSE_LABEL}</button></div>
      {:else if screen === 'run'}
        <ShareRun />
      {:else if screen === 'blocked'}
        <p class="note" role="status">{v.album.block}</p>
        <div class="actions">
          {#if v.album.save}
            <button class="ghost" onclick={() => share.close()}>{CANCEL_LABEL}</button>
            <button class="primary" data-autofocus disabled={working} onclick={() => share.save()}>{SAVE_LABEL}</button>
          {:else}
            <button data-autofocus onclick={() => share.close()}>{CLOSE_LABEL}</button>
          {/if}
        </div>
      {:else if screen === 'connect'}
        {#if link && hasLink(v)}<ShareLink {link} readonly />{/if}
        <ShareConnect quiet={!!link && hasLink(v)} />
      {:else if screen === 'other' && link}
        {#if redo}
          <ShareForm title={link.title} password={link.password} oncancel={() => (redo = false)} />
        {:else}
          {#if hasLink(v)}<ShareLink {link} readonly />{/if}
          <p class="note">{otherLine(link.host)}</p>
          <div class="actions">
            <button data-autofocus onclick={() => share.close()}>{CLOSE_LABEL}</button>
            <button disabled={working} onclick={() => (redo = true)}>{AGAIN_LABEL}</button>
          </div>
        {/if}
      {:else if screen === 'incomplete'}
        <p class="note" role="status">{INCOMPLETE_LINE}</p>
        <div class="actions">
          <button class="ghost" onclick={() => share.close()}>{CANCEL_LABEL}</button>
          <button class="primary" data-autofocus disabled={working} onclick={resume}>{CONTINUE_LABEL}</button>
        </div>
      {:else if screen === 'expired' && link}
        <p class="note" role="status">{link.state === 'gone' ? GONE_LINE : EXPIRED_LINE}</p>
        <div class="actions">
          <button class="ghost" onclick={() => share.close()}>{CANCEL_LABEL}</button>
          <button class="primary" data-autofocus disabled={working} onclick={again}>{AGAIN_LABEL}</button>
        </div>
      {:else if screen === 'link' && link}
        <ShareLink {link} />
      {:else if screen === 'form'}
        <ShareForm />
      {/if}
      {#if share.waiting && screen !== 'run'}<p class="hint wait" role="status">{WAIT_LINE}</p>{/if}
    {/if}
  </Modal>
{/if}

<style>
  .note {
    margin: 0 0 10px;
    line-height: 1.4;
  }
  /* Строка состояния: слово слева, действие справа — как «Включено · Выключить» в окне агентов */
  .svc {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 12px;
    color: var(--muted);
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
  .hint {
    margin: -6px 0 12px;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
  }
  .wait {
    margin: 8px 0 0;
    text-align: right;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
