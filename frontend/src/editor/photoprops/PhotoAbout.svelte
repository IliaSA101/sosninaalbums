<script lang="ts">
  import { Api, store } from '../../base/core/store.svelte'
  import { profileLine } from '../photo/color'
  import { useLines } from '../../base/words/whereuse'
  import { AboutLoader, USES_MAX } from './aboutload.svelte'
  import { goUse } from './gouse.svelte'

  // «О снимке» (084): камера, параметры, дата и «Использовано N раз» со ссылками на листы. Строки готовит Go.
  // head — раздел «Фото из ленты»: сверху имя файла, размеры и профиль (в карточке холста они уже есть выше).
  let { id, head = false }: { id: string; head?: boolean } = $props()

  const loader = new AboutLoader((i) => Api.PhotoAbout(i))
  // Запрос — на смену фото и на каждую правку альбома (Go держит кэш файла, вхождения считает заново).
  $effect(() => {
    void loader.sync(id, store.view.rev)
  })

  const a = $derived(loader.data)
  const uses = $derived(useLines(a?.uses ?? [], USES_MAX))
  const info = $derived(store.byId.get(id))
  const profile = $derived(head ? profileLine(info) : null)
</script>

{#if a}
  <div class="about">
    {#if head}
      <div class="name" title={info?.name}>{info?.name ?? 'Фото'}</div>
      {#if a.file}<div class="line">{a.file}</div>{/if}
      {#if profile}
        <div class="line" class:warn={profile.level === 'warn'} title={profile.title}>{profile.text}</div>
      {/if}
    {/if}
    {#each a.shot as line, i (i)}<div class="line">{line}</div>{/each}
    {#if a.note}<div class="line">{a.note}</div>{/if}
    <div class="line">
      {a.used}{uses.shown.length ? ': ' : ''}{#each uses.shown as u, i (i)}{i > 0 ? ', ' : ''}<button
          type="button"
          class="link"
          onclick={() => goUse(u)}>{u.page}</button
        >{u.what ? ` — ${u.what}` : ''}{/each}{uses.more > 0 ? `, ещё ${uses.more}` : ''}
    </div>
  </div>
{/if}

<style>
  .about {
    margin-top: 8px;
    display: grid;
    gap: 4px;
    font-size: 12px;
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  .name {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .warn {
    color: #e0803a;
    font-weight: 600;
  }
  .link {
    all: unset;
    cursor: pointer;
    color: inherit;
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
</style>
