<script lang="ts">
  import PackRow from './PackRow.svelte'
  import { packsUi } from './packs.svelte'
  import {
    AVAIL_DOWNLOAD,
    AVAIL_TITLE,
    availAtLabel,
    availEmpty,
    availHint,
    availMeta,
    availNote,
    availWhy,
    downloadAllLabel,
  } from './packavail'

  // Раздел «Можно скачать» окна «Наборы» (137): наборы каталога, которых нет в библиотеке. Список — из сохранённого каталога
  // последней проверки (сеть при открытии окна не трогается); что показать, серое и размер решает Go.
  // Название, описание, лицензия и названия дополнений — недоверенный текст каталога: только как текст (не {@html}).
  // locked — кнопки серые (идёт ход, только чтение), lockHint — подсказка серой кнопки.
  let { locked, lockHint }: { locked: boolean; lockHint: string } = $props()
  const net = packsUi.net

  let root = $state<HTMLElement>()
  const empty = $derived(availEmpty(packsUi.packs, packsUi.availAt, packsUi.available.length))
  const all = $derived(downloadAllLabel(packsUi.all))
  const shown = $derived(!!empty || packsUi.available.length > 0)

  // «Обновить» нашло что скачать или нажали «Показать» в тосте: окно прокручивается к разделу. Раздела нет — просьба гаснет.
  $effect(() => {
    if (!packsUi.reveal || packsUi.loading) return
    if (shown && !root) return // раздел ещё монтируется: bind:this вернёт сюда
    root?.scrollIntoView?.({ block: 'start' })
    packsUi.reveal = false
  })
</script>

{#if shown}
  <section bind:this={root} aria-label={AVAIL_TITLE}>
    {#if packsUi.available.length > 0}
      <div class="head">
        <span class="cap">{AVAIL_TITLE}</span>
        {#if availAtLabel(packsUi.availAt)}<span class="at">{availAtLabel(packsUi.availAt)}</span>{/if}
        <span class="grow"></span>
        {#if all}
          <button class="link" disabled={locked} title={lockHint || undefined} onclick={() => net.downloadAll()}>{all}</button>
        {/if}
      </div>
      <ul class="list" aria-label={AVAIL_TITLE}>
        {#each packsUi.available as a (a.id)}
          <PackRow title={a.title} lines={[availMeta(a), availNote(a), availWhy(a)]} about={a.about}>
            <button disabled={locked || !!a.why} title={lockHint && !a.why ? lockHint : availHint(a)} onclick={() => net.download(a)}
              >{AVAIL_DOWNLOAD}</button
            >
          </PackRow>
        {/each}
      </ul>
    {/if}
    {#if empty}<p class="note">{empty}</p>{/if}
  </section>
{/if}

<style>
  section {
    margin: 0 0 10px;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 10px;
    margin: 0 0 6px;
  }
  .cap {
    font-weight: 600;
  }
  .at {
    color: var(--muted);
    font-size: 12px;
  }
  .grow {
    flex: 1;
  }
  .note {
    margin: 0 0 12px;
    color: var(--muted);
    line-height: 1.4;
  }
  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 6px;
  }
  /* Ссылка-кнопка: как «Вернуть» под списком окна */
  .link {
    all: unset;
    cursor: pointer;
    color: var(--muted);
    font-size: 12px;
    text-decoration: underline dotted;
    text-underline-offset: 3px;
  }
  .link:hover,
  .link:focus-visible {
    text-decoration-style: solid;
  }
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .link:disabled {
    cursor: default;
    opacity: 0.5;
    text-decoration: none;
  }
</style>
