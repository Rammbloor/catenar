import { mount } from 'svelte'
import './app.css'
import './traffic-workspace.css'
import './redesign.css'
import App from './App.svelte'

mount(App, {
  target: document.getElementById('app')!,
})
