<script lang="ts">
  import { untrack } from 'svelte'
  import ExportGroup from '../export/ExportGroup.svelte'
  import Seg from '../../base/tpl/Seg.svelte'
  import ShareInput from './ShareInput.svelte'
  import { share } from './share.svelte'
  import { DAYS_ITEMS, loadDays, saveDays, summaryLine } from './share'
  import {
    CANCEL_LABEL,
    INVENT_LABEL,
    NAME_LABEL,
    PASSWORD_LABEL,
    PASSWORD_PLACEHOLDER,
    START_LABEL,
    TERM_LABEL,
    dataHint,
  } from './sharetext'

  // Форма выгрузки: название, пароль (необязательно), срок; сводка из Go. Окно монтируется заново при каждом открытии.
  // Проверки пароля и названия — в Go (его отказ придёт тостом), здесь только «название не пусто».
  // Название и пароль записи подставляются при «Выгрузить заново» с ссылки другого сервиса; «Отмена» тогда возвращает к ссылке (oncancel).
  let { title: seedTitle, password: seedPass = '', oncancel }: { title?: string; password?: string; oncancel?: () => void } = $props()
  const v = $derived(share.view!)
  let title = $state(untrack(() => seedTitle) ?? share.view?.album.title ?? '')
  let password = $state(untrack(() => seedPass))
  let days = $state(loadDays(share.view?.service.days))
  const working = $derived(share.working)
  const can = $derived(!working && title.trim() !== '')

  async function invent() {
    const p = await share.invent()
    if (p) password = p
  }
  function go() {
    if (!can) return
    saveDays(days)
    void share.start({ title: title.trim(), password, days })
  }
</script>

<ExportGroup cap={NAME_LABEL}>
  <ShareInput bind:value={title} label={NAME_LABEL} onenter={go} />
</ExportGroup>
<ExportGroup cap={PASSWORD_LABEL}>
  <ShareInput bind:value={password} label={PASSWORD_LABEL} placeholder={PASSWORD_PLACEHOLDER} onenter={go} />
  <button class="ghost" disabled={working} onclick={invent}>{INVENT_LABEL}</button>
</ExportGroup>
<ExportGroup cap={TERM_LABEL}>
  <Seg items={DAYS_ITEMS(v.service.days)} value={days} onpick={(d) => (days = d)} label={TERM_LABEL} />
</ExportGroup>
<p class="hint size">{summaryLine(v.album.pages, v.album.bytes)}.</p>
<p class="hint">{dataHint(v.service.host)}</p>

<div class="actions">
  <button class="ghost" onclick={() => (oncancel ? oncancel() : share.close())}>{CANCEL_LABEL}</button>
  <button class="primary" data-autofocus disabled={!can} onclick={go}>{START_LABEL}</button>
</div>

<style>
  .hint {
    color: var(--muted);
    font-size: 12px;
    line-height: 1.45;
    margin: 0 0 8px;
  }
  .size {
    margin-top: -4px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
</style>
