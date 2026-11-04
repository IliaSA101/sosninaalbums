<script lang="ts">
  import type { LayerOp } from '../places/minipanel'
  import Seg, { type SegItem } from '../../base/tpl/Seg.svelte'

  // «Расположение» в карточке (спека 009 U4): порядок слоёв четырьмя кнопками. Показывает вызывающий,
  // и только в «Ручной». Подписи короче, чем в меню «Слой»; значок + подпись не влезают в четыре
  // кнопки по ~55 px, поэтому сетка 2×2: пары «передний/задний план» и «выше/ниже».
  let { onpick }: { onpick: (op: LayerOp) => void } = $props()

  const OPS: SegItem<LayerOp>[] = [
    { id: 'front', label: 'Вперёд', title: 'На передний план', icon: 'layer-front' },
    { id: 'back', label: 'Назад', title: 'На задний план', icon: 'layer-back' },
    { id: 'up', label: 'Выше', title: 'На слой выше, через ближайшее перекрывающееся', icon: 'layer-up' },
    { id: 'down', label: 'Ниже', title: 'На слой ниже, через ближайшее перекрывающееся', icon: 'layer-down' },
  ]
</script>

<div class="lbl" title="Alt+щелчок по холсту — выбрать объект под верхним">Расположение</div>
<div class="seg" data-guide="layer-row">
  <Seg look="card" action keepFocus cols={2} label="Расположение" items={OPS} {onpick} />
</div>

<style>
  .lbl {
    margin-top: 10px;
    font-size: 12px;
    color: var(--muted);
  }
  .seg {
    margin-top: 4px;
  }
</style>
