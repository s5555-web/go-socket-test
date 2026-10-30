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
    roomSessions: new Map(), reactive, entries, activeId: ref(1), me: ref({ id: 2 }), members: ref([]), token: ref('test'),
    E2EE: { decryptPayload: async () => ({ attachment: { id: 'image', size: 3, mime: 'image/png' } }), decryptAttachment: async () => new Uint8Array(3) },
    fetch: async () => ({ ok: downloadOK, arrayBuffer: async () => new ArrayBuffer(3) }),
    URL: { createObjectURL: () => 'blob:decrypted' }, Blob, objectURLs: new Set(),
    document: {hidden:false}, nextTick: async () => {}, scrollBottom() {}
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

test('support decryption does not retain display plaintext if hidden or switched while decrypting', async () => {
  let resolve
  const document={hidden:false},supportActive=ref(7)
  let decryptCalls=0
  const context=vm.createContext({document,supportActive,me:ref({id:1}),E2EE:{decryptSupport(){decryptCalls++;return new Promise(done=>{resolve=done})}}})
  vm.runInContext(section('async function decryptSupportMessages(', '\nasync function openSupport('),context)
  let pending=context.decryptSupportMessages([{id:1,thread_id:7,body:'cipher'}])
  document.hidden=true;resolve('private message')
  assert.equal((await pending).length,0)
  await context.decryptSupportMessages([{id:1,thread_id:7,body:'cipher'}])
  assert.equal(decryptCalls,1)
  document.hidden=false
  pending=context.decryptSupportMessages([{id:1,thread_id:7,body:'cipher'}])
  supportActive.value=8;resolve('private message')
  assert.equal((await pending).length,0)
})

test('silent history merge reuses existing image nodes and keeps socket deliveries', async () => {
  let finish, calls = 0
  const first = reactive({ message: { id: 1, body: 'cipher', conversation_id: 7 }, payload: {}, imageUrl: 'blob:cached' })
  const entries = ref([first])
  const context = vm.createContext({
    reactive, entries, activeId: ref(7), me: ref({id:1}), members: ref([]), document: {hidden:false},
    roomSessions: new Map(), roomRevision: 1,
    historyStore: {sortKey: message => String(message.id).padStart(5,'0')},
    E2EE: {decryptPayload: () => {calls++;return new Promise(resolve => {finish=resolve})}},
    loadEntryImage: () => assert.fail('cached images must not be downloaded again')
  })
  vm.runInContext(section('async function decryptMessages(', 'async function loadEntryImage('), context)
  const pending = context.decryptMessages([first.message, {id:2,body:'new',conversation_id:7}])
  const socketEntry = reactive({message:{id:3,body:'socket',conversation_id:7},payload:{}})
  entries.value.push(socketEntry)
  finish({body:'new message'})
  await pending
  assert.equal(calls,1)
  assert.equal(entries.value[0],first)
  assert.equal(entries.value[2],socketEntry)
  assert.deepEqual(Array.from(entries.value,entry=>entry.message.id),[1,2,3])
})

test('stale history decryption cannot populate a newer room selection', async () => {
  let finish
  const entries=ref([])
  const context=vm.createContext({mentionRange:ref(null),showMembers:ref(false),selectedMentions:ref([]),reactive,entries,activeId:ref(7),me:ref({id:1}),members:ref([]),document:{hidden:false},roomRevision:1,
    E2EE:{decryptPayload:()=>new Promise(resolve=>{finish=resolve})}})
  vm.runInContext(section('async function decryptMessages(', 'async function loadEntryImage('),context)
  const pending=context.decryptMessages([{id:1,body:'cipher',conversation_id:7}])
  context.roomRevision=2
  finish({body:'old selection'})
  await pending
  assert.equal(entries.value.length,0)
})

test('support cache persists ciphertext only and merges duplicate messages by id', () => {
  const data=new Map(),supportMessages=ref([])
  const context=vm.createContext({me:ref({id:5}),supportMessages,localStorage:{getItem:key=>data.get(key),setItem:(key,value)=>data.set(key,value)}})
  vm.runInContext(section('function supportCacheKey(', 'async function openSupport('),context)
  const message={id:1,thread_id:7,body:JSON.stringify({alg:'encrypted',recipients:{}}),displayBody:'SECRET',created_at:'2026-10-30T00:00:00Z'}
  context.cacheSupport(7,[message,message])
  assert.equal([...data.values()].join('').includes('SECRET'),false)
  assert.equal(context.readSupportCache(7).length,1)
  context.mergeSupport([message,message])
  assert.equal(supportMessages.value.length,1)
})

test('reopening the current room keeps history visible while network sync is pending', async () => {
  let finishMembers
  const entry=reactive({message:{id:1,conversation_id:7,body:'cipher'},payload:{body:'cached'}})
  const entries=ref([entry]),memberList=[{id:1,public_key:'public'}]
  const context=vm.createContext({
    mentionRange:ref(null),showMembers:ref(false),selectedMentions:ref([]),reactive,entries,activeId:ref(7),activeView:ref('chats'),settingsMobileOpen:ref(false),supportActive:ref(0),
    members:ref(memberList),me:ref({id:1}),roomRevision:0,roomSessions:new Map(),memberSessions:new Map([[7,memberList]]),
    historyCursor:ref(''),historyDone:ref(false),messagesBox:ref(null),document:{hidden:false},console,
    localStorage:{getItem:()=>null,setItem(){}},historyStore:{PAGE:40,page:async()=>[{sort:'1',message:entry.message}],markRead:async()=>{}},
    decryptMessages:async()=>{},nextTick:async()=>{},scrollBottom(){},loadChats:async()=>{},
    E2EE:{changedMemberKeys:async()=>[]},
    api:path=>path.endsWith('/members')?new Promise(resolve=>{finishMembers=resolve}):Promise.resolve([])
  })
  vm.runInContext(section('async function openRoom(', '\nasync function loadOlder()'),context)
  const pending=context.openRoom(7)
  await new Promise(resolve=>setImmediate(resolve))
  assert.equal(entries.value[0],entry)
  finishMembers(memberList)
  await pending
  assert.equal(entries.value[0],entry)
})

for (const cached of [false, true]) test(`opening ${cached?'cached':'remote-only'} history positions at bottom after messages render`, async () => {
  const message={id:1,conversation_id:7,body:'cipher'},entries=ref([])
  let records=[],scrolls=0
  const context=vm.createContext({
    mentionRange:ref(null),showMembers:ref(false),selectedMentions:ref([]),reactive,entries,activeId:ref(0),activeView:ref('chats'),settingsMobileOpen:ref(false),supportActive:ref(0),
    members:ref([]),me:ref({id:1}),roomRevision:0,stickToLatest:true,
    roomSessions:new Map(cached?[[7,[{message,payload:{body:'cached'}}]]]:[]),memberSessions:new Map(),
    historyCursor:ref(''),historyDone:ref(false),messagesBox:ref({scrollHeight:1200,scrollTop:0,clientHeight:300}),document:{hidden:false},console,
    localStorage:{getItem:()=>null,setItem(){}},
    historyStore:{PAGE:40,page:async()=>records,markRead:async()=>{},put:async message=>{records=[{sort:'1',message}]}},
    decryptMessages:async messages=>{if(messages.length)entries.value=messages.map(message=>({message,payload:{}}))},
    nextTick:async()=>{},scrollBottom(){assert.ok(entries.value.length);scrolls++},loadChats:async()=>{},
    E2EE:{changedMemberKeys:async()=>[]},
    api:async path=>path.endsWith('/members')?[{id:1,public_key:'public'}]:path.endsWith('/messages')?[message]:[]
  })
  vm.runInContext(section('async function openRoom(', '\nasync function loadOlder()'),context)
  await context.openRoom(7)
  assert.ok(scrolls>0,'newly opened conversation must scroll even when initial cache is empty or already restored')
})

test('image layout follows bottom until the user scrolls away',async()=>{
  let scrolls=0
  const box={scrollHeight:1500,scrollTop:1200,clientHeight:300}
  const context=vm.createContext({stickToLatest:true,messagesBox:ref(box),activeId:ref(7),nextTick:async()=>{},loadOlder:async()=>{},showFailToast(){}})
  vm.runInContext(section('function onMessageScroll()', 'function scrollSupportBottom()'),context)
  await context.onMessageImageLoad({message:{conversation_id:7}})
  assert.equal(box.scrollTop,1500)
  box.scrollTop=500
  context.onMessageScroll()
  await context.onMessageImageLoad({message:{conversation_id:7}})
  assert.equal(box.scrollTop,500)
})
