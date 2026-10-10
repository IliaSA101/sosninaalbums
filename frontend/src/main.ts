import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { wire } from './shell/boot/wire'

// Верх подключается к основе до первого кадра (base/ его не импортирует).
wire()

export default mount(App, { target: document.getElementById('app')! })
