// Курсор роликов README (102, design §4.1): init-скрипт страницы. Системный курсор в захват CDP не попадает,
// поэтому рисуем свой: стрелка 22×22 и кольцо нажатия. Узел висит на documentElement, а не в body: Svelte его не
// перерисует; pointer-events: none — elementFromPoint (цели переноса) его пропускает.
(() => {
  if (window.__demoCursor) return
  // Счётчик загрузок страницы во вкладке: сцена видит, что страница перезагрузилась (стенд пересобрался) — запись негодна.
  try {
    if (window.top === window)
      sessionStorage.setItem('__demoLoads', String(Number(sessionStorage.getItem('__demoLoads') || 0) + 1))
  } catch (e) {
    // нет sessionStorage — защита от перезагрузки не работает, запись идёт
  }
  const NS = 'http://www.w3.org/2000/svg'
  let box = null
  let ring = null
  let shown = false
  let x = 0
  let y = 0

  function build() {
    const root = document.documentElement
    if (box || !root) return
    box = document.createElement('div')
    box.id = '__demo-cursor'
    box.setAttribute('aria-hidden', 'true')
    box.style.cssText =
      'position:fixed;left:0;top:0;width:22px;height:22px;pointer-events:none;z-index:2147483647;opacity:0;' +
      'will-change:transform;filter:drop-shadow(0 1px 2px rgba(0,0,0,.55))'
    const svg = document.createElementNS(NS, 'svg')
    svg.setAttribute('viewBox', '0 0 22 22')
    svg.setAttribute('width', '22')
    svg.setAttribute('height', '22')
    const arrow = document.createElementNS(NS, 'path')
    // Классическая стрелка: остриё в (1,1) — туда и смещён узел, чтобы остриё стояло ровно в точке указателя.
    arrow.setAttribute('d', 'M1.5 1.5 L1.5 17 L5.6 13.2 L8.6 19.6 L11.4 18.3 L8.5 12 L14.2 12 Z')
    arrow.setAttribute('fill', '#fff')
    arrow.setAttribute('stroke', '#1b1b1d')
    arrow.setAttribute('stroke-width', '1.3')
    arrow.setAttribute('stroke-linejoin', 'round')
    svg.appendChild(arrow)
    box.appendChild(svg)
    ring = document.createElement('div')
    ring.id = '__demo-cursor-ring'
    ring.setAttribute('aria-hidden', 'true')
    ring.style.cssText =
      'position:fixed;left:0;top:0;width:28px;height:28px;margin:-14px 0 0 -14px;border-radius:50%;pointer-events:none;' +
      'z-index:2147483646;border:2px solid rgba(255,255,255,.9);background:rgba(255,220,0,.28);opacity:0;' +
      'box-shadow:0 0 0 1px rgba(0,0,0,.35)'
    root.appendChild(ring)
    root.appendChild(box)
  }

  function place() {
    if (!box) return
    box.style.transform = 'translate(' + (x - 1.5) + 'px,' + (y - 1.5) + 'px)'
  }

  function move(e) {
    build()
    x = e.clientX
    y = e.clientY
    if (!shown) {
      shown = true
      if (box) box.style.opacity = '1'
    }
    place()
  }

  function down(e) {
    build()
    if (!ring) return
    ring.style.left = e.clientX + 'px'
    ring.style.top = e.clientY + 'px'
    ring.style.transition = 'none'
    ring.style.opacity = '1'
    ring.style.transform = 'scale(.6)'
    // Гаснет за 250 мс: следующий кадр включает переход.
    requestAnimationFrame(() => {
      ring.style.transition = 'opacity 250ms ease-out, transform 250ms ease-out'
      ring.style.opacity = '0'
      ring.style.transform = 'scale(1.15)'
    })
  }

  window.__demoCursor = {
    // Явная постановка (до первого движения мыши): сценарий ставит курсор в исходную точку.
    set(px, py) {
      build()
      x = px
      y = py
      shown = true
      if (box) box.style.opacity = '1'
      place()
    },
    hide() {
      if (box) box.style.opacity = '0'
      shown = false
    },
  }
  window.addEventListener('pointermove', move, { capture: true, passive: true })
  window.addEventListener('mousemove', move, { capture: true, passive: true })
  window.addEventListener('pointerdown', down, { capture: true, passive: true })
  window.addEventListener('mousedown', down, { capture: true, passive: true })
  if (document.documentElement) build()
  else document.addEventListener('DOMContentLoaded', build, { once: true })
})()
