<script lang="ts">
  import Icon from '../../base/ui/Icon.svelte'
  import { isMac } from '../../base/core/platform'
  import { chordText } from '../../base/keys/chordtext'
  import { keymap } from '../../base/keys/keymap.svelte'
  import type { KeymapEntry } from '../../base/types/keys'
  import KeyChips from './KeyChips.svelte'
  import { cmdsAt, rankOf, reasonOf, shadowsOf } from './keysel'
  import { keysUi } from './keysui.svelte'
  import { hid, phrase, plural, q, SCOPE_LEAD, SCOPE_TAG } from './keywords'

  // Карточка справа (074, макет Г): выбранная команда — что делает, где действует, сочетания, «Стандартное», перекрытия.
  // Выбор клавиши на карте даёт ту же карточку, что выбор строки; несколько команд на клавише — переключатель между ними.
  // Свободная клавиша — «Назначить команду…», служебная — замок и причина.
  const mac = isMac()
  const sel = $derived(keysUi.sel)
  const chord = $derived(sel?.kind === 'key' ? sel.chord : '')
  const why = $derived(chord ? reasonOf(chord, keysUi.ctx.locked) : '')
  const cmd = $derived(keysUi.cur ? keymap.entry(keysUi.cur) : undefined)
  const onKey = $derived(chord && !why ? cmdsAt(keysUi.ctx, chord) : [])
  const scopeTitle = (e: KeymapEntry) => keymap.view.scopes.find((s) => s.id === e.scope)?.title ?? ''
  const reasonOfCmd = (e: KeymapEntry) => reasonOf(e.defaults[0] ?? '', keysUi.ctx.locked) || 'служебная клавиша'
  const defaults = (e: KeymapEntry) => e.defaults.map((k) => chordText(k, mac)).join(', ') || 'без клавиши'

  // Служебная клавиша: все команды на ней, и у какой область главнее.
  const hits = $derived(chord ? cmdsAt(keysUi.ctx, chord, true) : [])
  const winner = $derived.by(() => {
    const top = hits[0]
    if (!top || hits.length < 2) return ''
    const view = keymap.view
    return shadowsOf(view, keymap.index, top, chord).some((o) => rankOf(view, o) < rankOf(view, top))
      ? `${SCOPE_LEAD[top.scope]} сработает ${q(top.label)} — область главнее.`
      : ''
  })
</script>

<div class="card">
  {#if !sel}
    <p class="muted">
      Нажмите клавишу на клавиатуре или щёлкните её на карте — увидите, что на ней. Чтобы назначить свою, щёлкните сочетание команды (или
      «+») и нажмите нужную клавишу.
    </p>
  {:else if why}
    <h2>{chordText(chord, mac)}<Icon name="lock" size={14} /></h2>
    <p class="what">Служебная клавиша: {why}. Не переназначается.</p>
    {#each hits as e (e.id)}
      <div class="hit"><span><b>{e.label}</b>{hid(keysUi.visible(e))}</span><span class="sc">{scopeTitle(e)}</span></div>
    {/each}
    {#if winner}<p class="note">{winner}</p>{/if}
  {:else if chord && !cmd}
    {@const hidden = hits[0]}
    <h2>{phrase(chord, mac)} — свободна</h2>
    {#if hidden}
      <p class="what muted">Занята командой {q(hidden.label)}{hid(false)}: на этом уровне её нет в списке.</p>
    {:else if keysUi.pend === chord}
      <p class="what">Выберите команду в списке — {phrase(chord, mac)} станет её сочетанием.</p>
      <div><button class="small ghost" onclick={() => keysUi.cancelPend()}>Отмена</button></div>
    {:else}
      <p class="what muted">Пока ни одной команды.</p>
      <div>
        <button class="small primary" title="Затем выберите команду в списке" onclick={() => keysUi.assignKey(chord)}
          >Назначить команду…</button
        >
      </div>
    {/if}
  {:else if cmd}
    {#if onKey.length > 1}
      <div class="pick" role="group" aria-label="Команды клавиши">
        <span class="pl">{phrase(chord, mac)} — {onKey.length} {plural(onKey.length, 'команда', 'команды', 'команд')}</span>
        {#each onKey as e (e.id)}
          <button class="pk" class:on={e.id === cmd.id} aria-pressed={e.id === cmd.id} onclick={() => keysUi.pickCmd(e.id)}>
            <span class="t">{e.label}</span><small>{SCOPE_TAG[e.scope] || 'везде'}</small>
          </button>
        {/each}
      </div>
    {/if}
    <h2>
      {cmd.label}{#if cmd.fixed}<Icon name="lock" size={14} />{/if}
    </h2>
    {#if cmd.hint}<p class="what">{cmd.hint}</p>{/if}
    <div class="rw"><b>Действует</b><span class="v">{scopeTitle(cmd)}</span></div>
    <div class="rw"><b>Клавиши</b><span class="v"><KeyChips e={cmd} /></span></div>
    {#if cmd.fixed}
      <div class="rw warn"><b>Служебная</b><span class="v">{reasonOfCmd(cmd)}. Не переназначается.</span></div>
    {/if}
    {#if cmd.custom}
      <div class="rw">
        <b>Стандартное</b>
        <span class="v">
          <span>{defaults(cmd)}</span>
          <button class="small" title="Вернуть стандартные клавиши этой команды" onclick={() => void keysUi.resetOne(cmd.id)}
            >Стандартное</button
          >
        </span>
      </div>
    {/if}
    {#each keysUi.notesFor(cmd) as n (n)}<p class="note">{n}</p>{/each}
  {/if}
</div>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    line-height: 1.4;
  }
  h2 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  p {
    margin: 0;
  }
  .muted {
    color: var(--muted);
  }
  .rw {
    display: grid;
    grid-template-columns: 84px 1fr;
    gap: 8px;
    color: var(--muted);
    align-items: start;
  }
  .rw b {
    font-weight: 600;
    color: var(--text);
  }
  .v {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .warn .v {
    color: var(--accent);
  }
  .small {
    height: 22px;
    padding: 0 8px;
    font-size: 12px;
  }
  .note {
    color: var(--muted);
    font-style: italic;
  }
  .hit {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 8px;
    padding: 4px 8px;
    border-radius: 4px;
    background: #333337;
  }
  .sc {
    color: var(--muted);
    font-size: 12px;
  }
  /* Переключатель команд клавиши: щелчок выбирает строку команды, как в списке */
  .pick {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .pl {
    font-size: 12px;
    color: var(--muted);
  }
  .pk {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    height: auto;
    min-height: 28px;
    padding: 4px 8px;
    text-align: left;
    white-space: normal;
    background: #333337;
    border: 1px solid transparent;
    border-radius: 4px;
  }
  .pk:hover:not(.on) {
    background: #3c3c40;
  }
  .pk.on {
    background: #46464b;
    border-color: var(--accent);
  }
  .pk small {
    font-size: 11px;
    color: var(--muted);
  }
</style>
