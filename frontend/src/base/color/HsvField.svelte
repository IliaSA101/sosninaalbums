<script lang="ts">
  import { untrack } from 'svelte'
  import { primaryPress } from '../core/platform'
  import { screenDpr } from '../ui/dpr.svelte'
  import { hexToHsv, hsvToHex, type Hsv } from './palette'

  // «Свой оттенок» (109-7): поле насыщенность/яркость и полоса оттенка. Протяжка — onpreview на ходу (в историю не
  // идёт), отпускание — один oncommit (один шаг Ctrl+Z), прерванный жест — oncancel.
  let {
    hex,
    onpreview,
    oncommit,
    oncancel,
  }: {
    hex: string // текущий цвет окна: от него полоса и поле начинаются
    onpreview: (hex: string) => void
    oncommit: (hex: string) => void
    oncancel: () => void
  } = $props()

  const W = 250
  const SV_H = 110
  const HUE_H = 12

  let hsv = $state<Hsv>({ h: 0, s: 0, v: 0 })
  let sv = $state<HTMLCanvasElement>()
  let hue = $state<HTMLCanvasElement>()
  let drag: 'sv' | 'hue' | null = null

  // Цвет снаружи сменился (не нами): берём его, а у серого оставляем оттенок, где полоса была.
  // Следит только за hex: во время протяжки hsv уходит вперёд от hex, и это не повод его сбрасывать.
  $effect(() => {
    const h = hex
    untrack(() => {
      if (h === hsvToHex(hsv)) return
      const n = hexToHsv(h)
      hsv = { h: n.s === 0 ? hsv.h : n.h, s: n.s, v: n.v }
    })
  })

  function paint() {
    const dpr = screenDpr.value
    const a = sv?.getContext('2d')
    const b = hue?.getContext('2d')
    if (!sv || !hue || !a || !b) return
    sv.width = Math.round(W * dpr)
    sv.height = Math.round(SV_H * dpr)
    hue.width = Math.round(W * dpr)
    hue.height = Math.round(HUE_H * dpr)
    a.scale(dpr, dpr)
    b.scale(dpr, dpr)

    a.fillStyle = hsvToHex({ h: hsv.h, s: 1, v: 1 })
    a.fillRect(0, 0, W, SV_H)
    let g = a.createLinearGradient(0, 0, W, 0)
    g.addColorStop(0, '#fff')
    g.addColorStop(1, 'rgba(255,255,255,0)')
    a.fillStyle = g
    a.fillRect(0, 0, W, SV_H)
    g = a.createLinearGradient(0, 0, 0, SV_H)
    g.addColorStop(0, 'rgba(0,0,0,0)')
    g.addColorStop(1, '#000')
    a.fillStyle = g
    a.fillRect(0, 0, W, SV_H)
    a.strokeStyle = hsv.v > 0.6 ? '#000' : '#fff'
    a.lineWidth = 2
    a.beginPath()
    a.arc(hsv.s * W, (1 - hsv.v) * SV_H, 6, 0, Math.PI * 2)
    a.stroke()

    const hg = b.createLinearGradient(0, 0, W, 0)
    for (let i = 0; i <= 6; i++) hg.addColorStop(i / 6, hsvToHex({ h: (i * 60) % 360, s: 1, v: 1 }))
    b.fillStyle = hg
    b.fillRect(0, 0, W, HUE_H)
    b.strokeStyle = '#fff'
    b.lineWidth = 2
    b.strokeRect((hsv.h / 360) * W - 3, 1, 6, HUE_H - 2)
  }
  $effect(paint)

  // Доля по осям внутри полотна, зажатая в [0, 1]; нулевой размер (ещё не разложено) — 0.
  function frac(e: PointerEvent, el: HTMLElement): [number, number] {
    const r = el.getBoundingClientRect()
    const f = (v: number, size: number) => (size > 0 ? Math.max(0, Math.min(1, v / size)) : 0)
    return [f(e.clientX - r.left, r.width), f(e.clientY - r.top, r.height)]
  }

  function move(e: PointerEvent) {
    if (!drag) return
    const el = drag === 'sv' ? sv : hue
    if (!el) return
    const [x, y] = frac(e, el)
    hsv = drag === 'sv' ? { ...hsv, s: x, v: 1 - y } : { ...hsv, h: Math.min(359.99, x * 360) }
    onpreview(hsvToHex(hsv))
  }

  function down(kind: 'sv' | 'hue') {
    return (e: PointerEvent) => {
      if (!primaryPress(e)) return
      drag = kind
      ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
      move(e)
    }
  }

  function up() {
    if (!drag) return
    drag = null
    oncommit(hsvToHex(hsv))
  }

  function cancel() {
    if (!drag) return
    drag = null
    oncancel()
  }
</script>

<div class="hsv">
  <canvas
    bind:this={sv}
    class="sv"
    aria-label="Насыщенность и яркость"
    style:width="{W}px"
    style:height="{SV_H}px"
    onpointerdown={down('sv')}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={cancel}
  ></canvas>
  <canvas
    bind:this={hue}
    class="hue"
    aria-label="Оттенок"
    style:width="{W}px"
    style:height="{HUE_H}px"
    onpointerdown={down('hue')}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={cancel}
  ></canvas>
  <div class="hint">Тяните по полю и полосе — разворот меняется сразу</div>
</div>

<style>
  .hsv {
    margin-top: 8px;
  }
  canvas {
    display: block;
    border-radius: 4px;
    cursor: crosshair;
    touch-action: none;
  }
  .hue {
    margin-top: 6px;
  }
  .hint {
    margin-top: 4px;
    color: var(--muted);
    font-size: 11px;
  }
</style>
