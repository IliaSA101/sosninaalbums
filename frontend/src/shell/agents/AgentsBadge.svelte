<script lang="ts">
  import { agents } from './agents.svelte'
  import { BADGE_HINT, BADGE_LABEL, badgeHintLive } from './agentstext'

  // Значок «ИИ» в верхней строке (143, У1): виден, только пока доступ агентов включён; точка — пока кто-то подключён.
  // Щелчок открывает окно «ИИ-агенты (MCP)». Образец — LevelBadge. Состояние держит `agents` (события agents:state).
  $effect(() => agents.watch())
</script>

{#if agents.on}
  <span class="wrap" data-agents-badge data-chrome data-text-ui>
    <button
      class="ghost"
      class:live={agents.live}
      title={agents.live ? badgeHintLive(agents.names) : BADGE_HINT}
      onclick={() => agents.show()}
    >
      {BADGE_LABEL}{#if agents.live}<span class="dot" aria-hidden="true"></span>{/if}
    </button>
  </span>
{/if}

<style>
  .wrap {
    flex: none;
  }
  button {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent);
  }
</style>
