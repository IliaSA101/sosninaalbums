<script lang="ts">
  import { agents } from './agents.svelte'
  import { EDIT_ELEVATED, EDIT_LABEL, EDIT_TITLE, MUTED_LABEL, MUTED_TEXT } from './agentstext'

  // Блок «Правки» окна «ИИ-агенты» (149): флажок «Разрешить агентам менять альбом» (выключен по умолчанию, от уровня чтения
  // не зависит) и «Разрешить вопросы» после трёх отказов подряд. Включение спрашивает (ASK_EDIT в AgentsDialog), снятие — сразу.
  // Программа от администратора: флажок серый, рядом причина и что делать (owner-ux §2.7), а не просто неактивная кнопка.
  const info = $derived(agents.info)
  const blocked = $derived(info.editBlocked === 'elevated')

  // Состояние флажка — у Go: до ответа (или вопроса) вид прежний.
  function flag(e: Event & { currentTarget: HTMLInputElement }) {
    const on = e.currentTarget.checked
    e.currentTarget.checked = info.edit
    agents.toggleEdit(on)
  }
</script>

<h3>{EDIT_TITLE}</h3>
<label class="chk" class:off={blocked}>
  <input type="checkbox" checked={info.edit} disabled={agents.busy || blocked} onchange={flag} />
  {EDIT_LABEL}
</label>
{#if blocked}<p class="note" role="note">{EDIT_ELEVATED}</p>{/if}
{#if info.asksMuted}
  <div class="row">
    <span class="note grow">{MUTED_TEXT}</span>
    <button disabled={agents.busy} onclick={() => agents.allowAsks()}>{MUTED_LABEL}</button>
  </div>
{/if}

<style>
  h3 {
    margin: 12px 0 6px;
    font-size: 13px;
    font-weight: 600;
  }
  .chk {
    display: flex;
    align-items: center;
    gap: 6px;
    margin: 0 0 6px;
  }
  .chk.off {
    color: var(--muted);
  }
  .note {
    margin: 0 0 10px;
    color: var(--muted);
    font-size: 12px;
    line-height: 1.4;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0 0 6px;
  }
  .grow {
    flex: 1;
    margin: 0;
  }
</style>
