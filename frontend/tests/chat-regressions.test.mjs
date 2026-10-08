import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, effect } from 'vue'
const source = readFileSync(new URL('../client/src/App.vue', import.meta.url), 'utf8')
function section(start, end) { return source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start))) }

test('decrypted image and download errors update the rendered entry', async () => {
  const entries = ref([])
  let downloadOK = true
  const context = vm.createContext({
    reactive, entries, activeId: ref(1), me: ref({ id: 2 }), members: ref([]), token: ref('test'),
    E2EE: { decryptPayload: async () => ({ attachment: { id: 'image', size: 3, mime: 'image/png' } }), decryptAttachment: async () => new Uint8Array(3) },
    fetch: async () => ({ ok: downloadOK, arrayBuffer: async () => new ArrayBuffer(3) }),
    URL: { createObjectURL: () => 'blob:decrypted' }, Blob, objectURLs: new Set(),
    nextTick: async () => {}, scrollBottom() {}
  })
  vm.runInContext(section('async function loadEntryImage(', '\nasync function openRoom(') + section('async function addMessage(', '\nfunction focusComposer('), context)
  let rendered
  effect(() => { rendered = entries.value.map(entry => entry.imageUrl || entry.imageError || 'loading') })
  await context.addMessage({ id: 1, conversation_id: 1 }, false)
  await new Promise(resolve => setImmediate(resolve))
  assert.deepEqual(rendered, ['blob:decrypted'])
  downloadOK = false
  await context.addMessage({ id: 2, conversation_id: 1 }, false)
  await new Promise(resolve => setImmediate(resolve))
  assert.deepEqual(rendered, ['blob:decrypted', '图片下载失败'])
})

test('support polling only runs while disconnected and refreshes are coalesced', async () => {
  const timers = new Map()
  let nextID = 0, requests = 0, complete
  const context = vm.createContext({
    me: ref({ is_support: true }), connectionState: ref('connected'), navigator: { onLine: true },
    supportThreads: ref([]), supportActive: ref(0), supportMessages: ref([]), console,
    setInterval(fn) { timers.set(++nextID, fn); return nextID }, clearInterval(id) { timers.delete(id) },
    api: () => { requests++; return new Promise(resolve => { complete = resolve }) }
  })
  vm.runInContext('let supportTimer = null; let supportRefresh = null;\n' + section('function refreshSupport()', 'watch([connectionState,'), context)
  context.updateSupportPolling()
  assert.equal(timers.size, 0)
  context.connectionState.value = 'reconnecting'
  context.updateSupportPolling()
  assert.equal(timers.size, 1)
  const first = context.refreshSupport()
  const second = context.refreshSupport()
  assert.equal(requests, 1)
  complete([])
  await Promise.all([first, second])
  context.connectionState.value = 'connected'
  context.updateSupportPolling()
  assert.equal(timers.size, 0)
  context.me.value.is_support = false
  context.connectionState.value = 'offline'
  context.updateSupportPolling()
  assert.equal(timers.size, 0)
})

test('group creation displays the API error and preserves the draft', async () => {
  const group = { name: '测试群', member_ids: [2] }
  const showGroup = ref(true)
  let toast
  const context = vm.createContext({
    group, showGroup,
    api: async () => { throw Error('只能邀请好友加入群聊') },
    showFailToast: message => { toast = message },
    loadChats: () => assert.fail('must not refresh on failure'),
    openRoom: () => assert.fail('must not open room on failure')
  })
  vm.runInContext(section('async function createGroup()', '\nasync function saveProfile'), context)
  await context.createGroup()
  assert.equal(toast, '只能邀请好友加入群聊')
  assert.equal(showGroup.value, true)
  assert.deepEqual(group, { name: '测试群', member_ids: [2] })
})
