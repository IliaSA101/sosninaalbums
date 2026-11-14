// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import { pickDrop } from './hit'

// Холст с листом и плавающая карточка поверх него (как Inspector): стопка elementsFromPoint задаётся вручную.
let sheet: HTMLElement
let card: HTMLElement
let cardText: HTMLElement
let zone: HTMLElement
let block: HTMLElement
let place: HTMLElement
let cell: HTMLElement

beforeEach(() => {
  document.body.innerHTML = `
    <div id="sheet" data-drop="spread"><i id="cell"></i><u id="inner" data-through></u></div>
    <div id="place" data-drop="place"></div><b id="block" data-through></b>
    <section id="card" data-chrome><p id="txt"></p><div id="zone" data-drop="backdrop"></div></section>`
  sheet = document.getElementById('sheet')!
  cell = document.getElementById('cell')!
  card = document.getElementById('card')!
  cardText = document.getElementById('txt')!
  zone = document.getElementById('zone')!
  block = document.getElementById('block')!
  place = document.getElementById('place')!
})

describe('pickDrop', () => {
  it('точка рядом с панелью: лист', () => {
    expect(pickDrop([cell, sheet])).toBe(sheet)
  })

  it('точка над панелью: цели нет, лист под ней не считается', () => {
    expect(pickDrop([cardText, card, sheet])).toBeNull()
    expect(pickDrop([card, sheet])).toBeNull()
  })

  it('своя цель внутри панели (зона фона) остаётся', () => {
    expect(pickDrop([zone, card, sheet])).toBe(zone)
  })

  it('пустая стопка: цели нет', () => {
    expect(pickDrop([])).toBeNull()
  })

  it('блок текста (data-through): бросок в место под ним', () => {
    expect(pickDrop([block, place, sheet])).toBe(place)
    expect(pickDrop([document.getElementById('inner')!, sheet])).toBe(sheet) // места под текстом нет — ближайшая сверху
  })
})
