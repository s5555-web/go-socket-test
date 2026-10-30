<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
defineProps({ src: { type: String, required: true } })
const emit = defineEmits(['close'])
const dialog = ref(null)
let previousFocus, previousOverflow
onMounted(() => {
  previousFocus = document.activeElement
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  dialog.value?.focus()
})
onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow
  previousFocus?.focus?.({ preventScroll: true })
})
</script>

<template>
  <Teleport to="body">
    <div ref="dialog" class="chat-image-preview" role="dialog" aria-modal="true" aria-label="图片预览，点击图片关闭" tabindex="-1" @click="emit('close')" @keydown.esc.stop.prevent="emit('close')">
      <img :src="src" alt="聊天图片，点击关闭" :draggable="false">
      <button type="button" class="chat-image-preview-close" aria-label="关闭图片预览">×</button>
    </div>
  </Teleport>
</template>

<style>
.chat-image-preview { position:fixed; inset:0; z-index:10000; display:flex; align-items:center; justify-content:center; background:rgba(0,0,0,.94); padding:24px; box-sizing:border-box; cursor:zoom-out; outline:none; }
.chat-image-preview img { display:block; max-width:100%; max-height:100%; object-fit:contain; user-select:none; }
.chat-image-preview-close { position:absolute; top:max(16px,env(safe-area-inset-top)); right:max(16px,env(safe-area-inset-right)); width:44px; height:44px; border:0; border-radius:50%; color:#fff; background:#ffffff24; font-size:30px; cursor:pointer; }
.chat-image-preview-close:focus-visible { outline:2px solid #fff; }
</style>
