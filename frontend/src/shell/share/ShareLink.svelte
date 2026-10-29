<script lang="ts">
  import Seg from '../../base/tpl/Seg.svelte'
  import ShareInput from './ShareInput.svelte'
  import { share } from './share.svelte'
  import { DAYS_ITEMS } from './share'
  import type { ShareLinkInfo } from './shareapi'
  import {
    CANCEL_LABEL,
    CHANGED_LINE,
    CHANGE_LABEL,
    CLOSE_LABEL,
    COPY_CLIENT_LABEL,
    EXTEND_LABEL,
    EXTEND_NOTE,
    INVENT_LABEL,
    OPEN_LABEL,
    PASSWORD_LABEL,
    PASSWORD_PLACEHOLDER,
    REVOKE_LABEL,
    SAVE_PASSWORD_LABEL,
    UPDATE_LABEL,
    passwordLine,
    termLine,
  } from './sharetext'

  // Блок готовой ссылки (state ok/changed): ссылка текстом, «Скопировать для клиента», «Открыть», пароль и срок с правкой,
  // при changed — «Обновить по ссылке». readonly — без подключения или у ссылки другого сервиса: только копировать и открыть.
  let { link, readonly = false }: { link: ShareLinkInfo; readonly?: boolean } = $props()

  const working = $derived(share.working)
  const maxDays = $derived(share.view?.service.days ?? 0)
  let editPass = $state(false)
  let pass = $state('')
  let extend = $state(false)

  const err = $derived(share.changeError)
  function editPassword() {
    pass = link.password
    share.changeError = null
    editPass = true
  }
  function cancelPassword() {
    share.changeError = null
    editPass = false
  }
  function toggleExtend(on: boolean) {
    share.changeError = null
    extend = on
  }
  async function invent() {
    const p = await share.invent()
    if (p) pass = p
  }
  // Редактор закрывается только при успехе: при отказе Go (строка под редактором) введённое остаётся
  async function savePassword() {
    if (await share.change({ days: 0, setPassword: true, password: pass })) editPass = false
  }
  async function pickDays(d: number) {
    if (await share.change({ days: d, setPassword: false, password: '' })) extend = false
  }
  // Обновление: days 0 — название, пароль и срок служба берёт из записи (T5-6), поля не шлём.
  const update = () => share.start({ days: 0 })
</script>

<div class="link">
  {#if link.published}<p class="meta">{link.published}</p>{/if}
  <p class="url" data-testid="share-url">{link.url}</p>
  <div class="row">
    <button class="primary" disabled={working} onclick={() => share.copy()}>{COPY_CLIENT_LABEL}</button>
    <button disabled={working} onclick={() => share.openLink()}>{OPEN_LABEL}</button>
  </div>

  <div class="line" class:edit={editPass} data-part="password">
    {#if editPass}
      <ShareInput
        bind:value={pass}
        label={PASSWORD_LABEL}
        placeholder={PASSWORD_PLACEHOLDER}
        onenter={savePassword}
        invalid={err?.part === 'password'}
      />
      <button class="ghost" disabled={working} onclick={invent}>{INVENT_LABEL}</button>
      <button class="ghost" onclick={cancelPassword}>{CANCEL_LABEL}</button>
      <button class="primary" disabled={working} onclick={savePassword}>{SAVE_PASSWORD_LABEL}</button>
    {:else}
      <span>{passwordLine(link.password)}</span>
      {#if !readonly}<button class="mini" disabled={working} onclick={editPassword}>{CHANGE_LABEL}</button>{/if}
    {/if}
  </div>

  {#if err?.part === 'password'}<p class="err" role="alert">{err.text}</p>{/if}

  <div class="line" data-part="term">
    <span>{termLine(link.until)}</span>
    {#if !readonly}
      {#if extend}
        <Seg items={DAYS_ITEMS(maxDays)} onpick={pickDays} label={EXTEND_LABEL} off={working} />
        <button class="ghost" onclick={() => toggleExtend(false)}>{CANCEL_LABEL}</button>
      {:else}
        <button class="mini" disabled={working} onclick={() => toggleExtend(true)}>{EXTEND_LABEL}</button>
      {/if}
    {/if}
  </div>
  {#if extend}<p class="meta">{EXTEND_NOTE}</p>{/if}
  {#if err?.part === 'term'}<p class="err" role="alert">{err.text}</p>{/if}

  {#if !readonly && link.state === 'changed'}
    <div class="changed" role="status">
      <span>{CHANGED_LINE}</span>
      <button disabled={working} onclick={update}>{UPDATE_LABEL}</button>
    </div>
  {/if}

  {#if !readonly}
    <footer>
      <button class="ghost" disabled={working} onclick={() => share.revoke()}>{REVOKE_LABEL}</button>
      <button data-autofocus onclick={() => share.close()}>{CLOSE_LABEL}</button>
    </footer>
  {/if}
</div>

<style>
  .link {
    margin: 0 0 8px;
  }
  .meta {
    margin: 0 0 6px;
    color: var(--muted);
    font-size: 12px;
  }
  .url {
    margin: 0 0 10px;
    padding: 6px 8px;
    background: var(--panel2);
    border-radius: 4px;
    font-family: Consolas, monospace;
    font-size: 12px;
    overflow-wrap: anywhere;
    user-select: text;
  }
  .row {
    display: flex;
    gap: 8px;
    margin: 0 0 12px;
  }
  .line {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    min-height: 30px;
    margin: 0 0 6px;
  }
  /* Редактор пароля в одну линию: поле сжимается, кнопки не переносятся */
  .line.edit {
    flex-wrap: nowrap;
  }
  /* Отказ Go под строкой — как на экране подключения */
  .err {
    margin: 0 0 6px;
    color: #e5645a;
    font-size: 12px;
  }
  /* Ссылка-кнопка внутри строки («Изменить», «Продлить»): как «Очистить журнал» в окне агентов */
  .mini {
    all: unset;
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .mini:hover,
  .mini:focus-visible {
    text-decoration-style: solid;
  }
  .mini:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .mini:disabled {
    cursor: default;
    opacity: 0.5;
    text-decoration: none;
  }
  .changed {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin: 10px 0 0;
    padding: 6px 8px;
    border: 1px solid var(--accent);
    border-radius: 6px;
    font-size: 13px;
  }
  footer {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    margin-top: 14px;
  }
</style>
