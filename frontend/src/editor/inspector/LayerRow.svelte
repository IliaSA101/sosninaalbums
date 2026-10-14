<script lang="ts">
  import type { LayerOp } from '../places/minipanel'
  import Icon, { type IconName } from '../../base/ui/Icon.svelte'

  // «Расположение» в карточке (спека 009 U4): порядок слоёв четырьмя кнопками. Показывает вызывающий,
  // и только в «Ручной». Подписи короче, чем в меню «Слой»; значок + подпись не влезают в четыре
  // кнопки по ~55 px, поэтому сетка 2×2: пары «передний/задний план» и «выше/ниже».
  let { onpick }: { onpick: (op: LayerOp) => void } = $props()

  const OPS: { op: LayerOp; label: string; title: string; icon: IconName }[] = [
    { op: 'front', label: 'Вперёд', title: 'На передний план', icon: 'layer-front' },
    { op: 'back', label: 'Назад', title: 'На задний план', icon: 'layer-back' },
    { op: 'up', label: 'Выше', title: 'На слой выше, через ближайшее перекрывающееся', icon: 'layer-up' },
    { op: 'down', label: 'Ниже', title: 'На слой ниже, через ближайшее перекрывающееся', icon: 'layer-down' },
  ]
</script>

<div class="lbl" title="Alt+щелчок по холсту — выбрать объект под верхним">Расположение</div>
<div class="seg" role="group" aria-label="Расположение" data-guide="layer-row">
  {#each OPS as o (o.op)}
    <button title={o.title} onmousedown={(e) => e.preventDefault()} onclick={() => onpick(o.op)}
      ><Icon name={o.icon} size={16} />{o.label}</button
    >
  {/each}
</div>

<style>
  .lbl {
    margin-top: 10px;
    font-size: 12px;
    color: var(--muted);
  }
  .seg {
    display: grid;
    grid-template-columns: 1fr 1fr;
    margin-top: 4px;
    border-radius: 5px;
    overflow: hidden;
    gap: 1px;
  }
  .seg button {
    height: 26px;
    padding: 0 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: 12px;
    border-radius: 0;
    background: #38383c;
    color: #cfcfd4;
  }
  .seg button:hover {
    background: #444449;
    color: #fff;
  }
</style>
