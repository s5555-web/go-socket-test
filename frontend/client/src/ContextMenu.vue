<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
const props = defineProps({ anchor: { type: Object, required: true }, label: String })
const emit = defineEmits(['close'])
const menu = ref(null)
const position = ref({ visibility: 'hidden' })
let previousFocus
function close() { emit('close') }
function outside(event) { if (!menu.value?.contains(event.target)) close() }
function scroll(event) { if (!menu.value?.contains(event.target)) close() }
function keydown(event) {
  if (event.key === 'Escape' || event.key === 'Tab') { close(); return }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const items = [...menu.value.querySelectorAll('button:not(:disabled)')]
  const current = items.indexOf(document.activeElement)
  const index = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
  items[index]?.focus()
}
onMounted(async () => {
  previousFocus = document.activeElement
  await nextTick()
  const { width, height } = menu.value.getBoundingClientRect()
  const { x, top, bottom } = props.anchor
  const gap = 8
  const below = window.innerHeight - bottom - gap
  const y = below >= height || below >= top - gap ? bottom + gap : top - height - gap
  position.value = { left: `${Math.max(gap, Math.min(x, window.innerWidth - width - gap))}px`, top: `${Math.max(gap, Math.min(y, window.innerHeight - height - gap))}px` }
  menu.value.querySelector('button')?.focus({ preventScroll: true })
  document.addEventListener('pointerdown', outside, true)
  document.addEventListener('keydown', keydown)
  window.addEventListener('resize', close)
  window.addEventListener('scroll', scroll, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', outside, true)
  document.removeEventListener('keydown', keydown)
  window.removeEventListener('resize', close)
  window.removeEventListener('scroll', scroll, true)
  if (menu.value?.contains(document.activeElement)) previousFocus?.focus({ preventScroll: true })
})
</script>

<template>
  <Teleport to="body">
    <div ref="menu" class="context-menu" :style="position" role="menu" :aria-label="label" @contextmenu.prevent><slot /></div>
  </Teleport>
</template>

<style>
.context-menu { position: fixed; z-index: 3000; width: 240px; max-width: calc(100vw - 16px); max-height: calc(100dvh - 16px); overflow-y: auto; padding: 8px; border: 1px solid var(--border); border-radius: 12px; background: var(--panel); color: var(--text); box-shadow: 0 8px 32px #0003; }
.context-menu button { display: flex; align-items: center; gap: 14px; width: 100%; min-height: 44px; padding: 10px 14px; border: 0; border-radius: 6px; background: transparent; text-align: left; cursor: pointer; }
.context-menu button:hover, .context-menu button:focus-visible { background: var(--surface-hover); outline: none; }
.context-menu .van-icon { font-size: 21px; }
.context-menu .danger { color: #e53935; }
.context-menu hr { margin: 6px 8px; border: 0; border-top: 1px solid var(--border); }
</style>
