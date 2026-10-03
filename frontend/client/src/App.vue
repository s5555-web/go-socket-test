<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { showConfirmDialog, showFailToast, showImagePreview, showSuccessToast, showToast } from 'vant'
import { createLocalHistory } from './history.js'
import E2EE from './crypto/e2ee.js'

const token = ref(localStorage.token || '')
const me = ref(null)
const historyStore = createLocalHistory(() => me.value)
const mode = ref('login')
const authBusy = ref(false)
const authError = ref('')
const auth = reactive({ username: '', password: '', display_name: '', about: '' })
const chats = ref([])
const friends = ref([])
const requests = ref([])
const userResults = ref([])
const members = ref([])
const activeId = ref(0)
const activeView = ref('chats')
const search = ref('')
const showArchived = ref(false)
const unreadOnly = ref(false)
const settingsSection = ref('profile')
const settingsMobileOpen = ref(false)
const theme = ref(localStorage.signalTheme || 'dark')
const zoom = ref(Number(localStorage.signalZoom || 100))
let storedPreferences = {}
try { storedPreferences = JSON.parse(localStorage.signalChatPreferences || '{}') } catch {}
const preferences = reactive({ keepMutedArchived: false, spellcheck: true, emojiConvert: true, linkPreview: true, readReceipts: true, typingIndicators: true, ...storedPreferences })
const entries = ref([])
const historyCursor = ref('')
const historyDone = ref(false)
const historyLoading = ref(false)
const connectionState = ref('connecting')
const connectionError = ref('')
const composer = ref('')
const composerInput = ref(null)
const sending = ref(false)
const replyDraft = ref(null)
const imageDraft = ref(null)
const imageDraftURL = ref('')
const messagesBox = ref(null)
const imageInput = ref(null)
const showEmoji = ref(false)
const emojiCategory = ref('最近')
const showFriendSearch = ref(false)
const friendQuery = ref('')
const showProfile = ref(false)
const profile = reactive({ display_name: '', about: '' })
const showGroup = ref(false)
const group = reactive({ name: '', member_ids: [] })
const conversationMenu = ref(null)
const messageMenu = ref(null)
const showForward = ref(false)
const forwardEntry = ref(null)
const forwarding = ref(false)
const installPrompt = ref(null)
const pushEnabled = ref(false)
const supportThreads = ref([])
const supportActive = ref(0)
const supportMessages = ref([])
const supportComposer = ref('')
const supportBusy = ref(false)
let socket = null
let reconnectTimer = null
let reconnectAttempt = 0
let socketConnectTimer = null
let socketEverConnected = false
let socketEventChain = Promise.resolve()
let supportTimer = null
let friendSearchTimer = null
const objectURLs = new Set()

const loggedIn = computed(() => Boolean(me.value && token.value))
const activeChat = computed(() => chats.value.find(item => item.id === activeId.value))
const mobileRoomOpen = computed(() => Boolean(activeId.value || supportActive.value || settingsMobileOpen.value))
const filteredFriends = computed(() => {
  const query = search.value.trim().toLowerCase()
  return friends.value.filter(item => !query || `${item.display_name} ${item.username} ${item.about || ''}`.toLowerCase().includes(query))
})
const newChatFriends = computed(() => {
  const query = friendQuery.value.trim().toLowerCase()
  return friends.value.filter(item => !query || `${item.display_name} ${item.username}`.toLowerCase().includes(query))
})
const filteredChats = computed(() => {
  const query = search.value.trim().toLowerCase()
  const source = showArchived.value ? chats.value.filter(item => item.archived) : chats.value.filter(item => !item.archived)
  return source.filter(item => (!unreadOnly.value || item.unread) && (!query || (item.name || '').toLowerCase().includes(query)))
})
const archivedCount = computed(() => chats.value.filter(item => item.archived).length)
const unreadCount = computed(() => chats.value.reduce((sum, item) => sum + (item.unread || 0), 0))
const activeSupportThread = computed(() => supportThreads.value.find(item => item.id === supportActive.value))
const emojiSets = reactive({
  最近: [],
  笑脸: '😀 😃 😄 😁 😆 😅 😂 🤣 😊 😇 🙂 🙃 😉 😌 😍 🥰 😘 😋 😛 😜 🤪 🤨 🧐 🤓 😎 🥳 😏 😒 😞 😔 😟 😕 🙁 ☹️ 😣 😖 😫 😩 🥺 😢 😭 😤 😠 😡 🤬 🤯 😳 🥵 🥶 😱 😨 😰 🤗 🤔 🤭 🤫 🤥 😶 😐 😑 😬 🙄 😯 😴 🤤 😪 😵 🤐 🤢 🤮 🤧 😷 🤒 🤕'.split(' '),
  手势: '👋 🤚 🖐️ ✋ 🖖 👌 🤌 🤏 ✌️ 🤞 🤟 🤘 🤙 👈 👉 👆 👇 ☝️ 👍 👎 ✊ 👊 🤛 🤜 👏 🙌 👐 🤲 🤝 🙏 ✍️ 💪 🫶'.split(' '),
  爱心: '❤️ 🧡 💛 💚 💙 💜 🖤 🤍 🤎 💔 ❣️ 💕 💞 💓 💗 💖 💘 💝 💟 💯 ✨ ⭐ 🌟 💫 🔥 🎉 🎊'.split(' '),
  物品: '🌹 🌸 🌺 🌻 🌞 🌙 ☀️ 🌈 ☁️ ⚡ ❄️ ☕ 🍵 🍺 🍻 🥂 🍷 🍹 🎂 🍰 🍎 🍉 🍓 🍒 🍜 🍕 🍔 🎁 🎈 ⚽ 🏀 🏆 🚗 ✈️ 🚀 ⌚ 📱 💻 📷 🎵 ✅ ❌ ❓ ❗'.split(' ')
})
try { emojiSets.最近 = JSON.parse(localStorage.signalRecentEmoji || '[]').slice(0, 24) } catch {}
const currentEmoji = computed(() => emojiCategory.value === '最近' && !emojiSets.最近.length ? emojiSets.笑脸.slice(0, 24) : emojiSets[emojiCategory.value])

function initials(value) { return (value || '?').slice(0, 2).toUpperCase() }
function selectView(view) {
  activeView.value = view
  settingsMobileOpen.value = false
  if (view !== 'chats') activeId.value = 0
  if (view !== 'support') supportActive.value = 0
  if (view === 'settings') Object.assign(profile, { display_name: me.value.display_name, about: me.value.about || '' })
}
function openSetting(section) { settingsSection.value = section; settingsMobileOpen.value = true }
function openNewChat() { selectView('new-chat'); friendQuery.value = ''; userResults.value = [] }
function applyAppearance() {
  document.documentElement.dataset.theme = theme.value
  document.documentElement.style.setProperty('--ui-scale', String(zoom.value / 100))
  localStorage.signalTheme = theme.value
  localStorage.signalZoom = String(zoom.value)
}
function notify(message, type = 'success') { type === 'error' ? showFailToast(message) : showSuccessToast(message) }
function messageSummary(payload, limit = 140) {
  const text = (payload?.body || '').trim().replace(/\s+/g, ' ')
  return text ? (text.length > limit ? `${text.slice(0, limit)}…` : text) : payload?.attachment ? '📷 图片' : '消息'
}
function dayKey(value) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '' : `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}` }
function dayLabel(value) {
  const date = new Date(value), now = new Date(), today = new Date(now.getFullYear(), now.getMonth(), now.getDate()), day = new Date(date.getFullYear(), date.getMonth(), date.getDate()), days = Math.round((today - day) / 86400000), week = date.toLocaleDateString('zh-CN', { weekday: 'short' })
  if (days === 0) return '今天'
  if (days === 1) return `昨天 · ${week}`
  return date.getFullYear() === now.getFullYear() ? `${date.getMonth() + 1}月${date.getDate()}日 · ${week}` : `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 · ${week}`
}

async function api(path, options = {}) {
  options.headers = { ...(options.headers || {}), 'Content-Type': 'application/json', Authorization: `Bearer ${token.value}` }
  const response = await fetch(`/api${path}`, options)
  if (response.status === 204) return null
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw Error(data.error || '请求失败')
  return data
}

async function submitAuth() {
  authBusy.value = true
  authError.value = ''
  try {
    const registering = mode.value === 'register'
    const result = await api(registering ? '/register' : '/login', { method: 'POST', body: JSON.stringify(auth) })
    token.value = result.token
    localStorage.token = result.token
    await start(auth.password)
    notify(registering ? '注册成功' : '登录成功')
  } catch (error) {
    authError.value = error.message
    showFailToast(error.message)
  } finally { authBusy.value = false }
}

async function start(password = '') {
  if (!window.isSecureContext || !crypto.subtle) throw Error('端到端加密要求使用 HTTPS 安全连接')
  try {
    me.value = await api('/me')
    const keys = await E2EE.init(me.value.username, me.value.public_key, me.value.key_backup, me.value.pq_public_key, me.value.pq_key_backup, password)
    if (keys.created) await api('/me/key', { method: 'PUT', body: JSON.stringify({ public_key: keys.publicKey, key_backup: keys.keyBackup, password, replace: keys.replace }) })
    if (keys.pqCreated) await api('/me/pq-key', { method: 'PUT', body: JSON.stringify({ public_key: keys.pqPublicKey, key_backup: keys.pqKeyBackup, password }) })
    me.value.public_key = keys.publicKey
    me.value.pq_public_key = keys.pqPublicKey
    await Promise.all([loadChats(), loadFriends()])
    connect()
		await applySupportRole(me.value.is_support)
    const launchID = Number(new URLSearchParams(location.search).get('conversation'))
    if (launchID && chats.value.some(item => item.id === launchID)) { history.replaceState(null, '', location.pathname); await openRoom(launchID) }
    if (keys.pqNeedsPassword) showFailToast('重新登录后将自动启用抗量子混合加密')
    if ('Notification' in window && Notification.permission === 'granted') syncPushSubscription().catch(() => {})
  } catch (error) {
    localStorage.removeItem('token')
    token.value = ''
    me.value = null
    throw error
  }
}

async function loadChats() {
  const rows = await api('/conversations')
  const summaries = await historyStore.summaries(rows.map(item => item.id))
  for (const chat of rows) {
    const local = summaries.get(chat.id)
    if (local?.latest) { chat.last_message = local.latest.body; chat.last_at = local.latest.created_at }
    chat.unread = Math.max(chat.manual_unread ? 1 : 0, local?.unread || 0)
  }
  chats.value = rows.sort((a, b) => Number(b.pinned) - Number(a.pinned) || new Date(b.last_at || 0) - new Date(a.last_at || 0))
  updateBadge()
}

async function loadFriends() { [friends.value, requests.value] = await Promise.all([api('/friends'), api('/friend-requests')]) }
async function createDirect(id) { const result = await api('/conversations', { method: 'POST', body: JSON.stringify({ member_ids: [id] }) }); showFriendSearch.value = false; await loadChats(); await openRoom(result.id) }
async function acceptFriend(id) { await api(`/friend-requests/${id}/accept`, { method: 'PUT' }); await loadFriends(); notify('已添加为好友') }
async function removeFriend(user, pending = false) {
  try { await showConfirmDialog({ title: pending ? '拒绝申请' : '删除好友', message: pending ? `拒绝 ${user.display_name} 的好友申请？` : `删除 ${user.display_name}？现有聊天记录不会删除。` }) } catch { return }
  await api(`/friends/${user.id}`, { method: 'DELETE' }); await loadFriends(); notify(pending ? '已拒绝申请' : '好友已删除')
}
async function searchUsers() { const query = friendQuery.value.trim(); userResults.value = query ? await api(`/users?q=${encodeURIComponent(query)}`) : [] }
async function relationshipAction(user) {
  if (user.relationship === 'friend') return createDirect(user.id)
  if (user.relationship === 'incoming') { showFriendSearch.value = false; activeView.value = 'friends'; return }
  if (user.relationship === 'none') { await api('/friend-requests', { method: 'POST', body: JSON.stringify({ user_id: user.id }) }); notify('好友申请已发送') }
  else await api(`/friends/${user.id}`, { method: 'DELETE' })
  await searchUsers()
}

function revokeImages() { for (const url of objectURLs) URL.revokeObjectURL(url); objectURLs.clear() }
async function decryptMessages(rawMessages) {
  revokeImages()
  const result = []
  for (const message of rawMessages) {
    const payload = await E2EE.decryptPayload(message.body, message.conversation_id, me.value.id, members.value, message.sender_id)
    const entry = { message, payload, imageUrl: '', imageError: '' }
    result.push(entry)
    if (payload.attachment) loadEntryImage(entry)
  }
  entries.value = result
}
async function loadEntryImage(entry) {
  try {
    const metadata = entry.payload.attachment
    const response = await fetch(`/api/conversations/${entry.message.conversation_id}/attachments/${metadata.id}`, { headers: { Authorization: `Bearer ${token.value}` } })
    if (!response.ok) throw Error('图片下载失败')
    const clear = await E2EE.decryptAttachment(await response.arrayBuffer(), metadata)
    if (metadata.size && clear.byteLength !== metadata.size) throw Error('图片完整性校验失败')
    const url = URL.createObjectURL(new Blob([clear], { type: metadata.mime }))
    objectURLs.add(url); entry.imageUrl = url
  } catch (error) { entry.imageError = error.message }
}

async function openRoom(id) {
  activeView.value = 'chats'
  settingsMobileOpen.value = false
  activeId.value = id
  supportActive.value = 0
  entries.value = []
  members.value = await api(`/conversations/${id}/members`)
  if (activeId.value !== id) return
  const changed = await E2EE.changedMemberKeys(members.value)
  if (changed.length) {
    try { await showConfirmDialog({ title: '安全密钥已改变', message: `${changed.map(item => item.display_name).join('、')} 的密钥发生变化。确认继续信任此新密钥？` }) }
    catch { activeId.value = 0; return }
    await E2EE.trustMemberKeys(changed)
  }
  await historyStore.markRead(id)
  await api(`/conversations/${id}/read`, { method: 'POST' }).catch(() => {})
  const page = await historyStore.page(id)
  historyCursor.value = page[0]?.sort || ''
  historyDone.value = page.length < historyStore.PAGE
  await decryptMessages(page.map(item => item.message))
  await loadChats()
  await nextTick(); scrollBottom()
}

async function loadOlder() {
  if (historyLoading.value || historyDone.value || !activeId.value) return
  historyLoading.value = true
  const oldHeight = messagesBox.value?.scrollHeight || 0
  const page = await historyStore.page(activeId.value, historyCursor.value)
  if (page.length) {
    historyCursor.value = page[0].sort
    const old = entries.value
    const payloads = []
    for (const item of page) {
      const payload = await E2EE.decryptPayload(item.message.body, item.message.conversation_id, me.value.id, members.value, item.message.sender_id)
      payloads.push({ message: item.message, payload, imageUrl: '', imageError: '' })
    }
    entries.value = [...payloads, ...old]
    for (const item of payloads) if (item.payload.attachment) loadEntryImage(item)
    historyDone.value = page.length < historyStore.PAGE
    await nextTick(); if (messagesBox.value) messagesBox.value.scrollTop = messagesBox.value.scrollHeight - oldHeight
  } else historyDone.value = true
  historyLoading.value = false
}
function onMessageScroll() { if ((messagesBox.value?.scrollTop || 0) < 48) loadOlder().catch(error => showFailToast(error.message)) }
function scrollBottom() { if (messagesBox.value) messagesBox.value.scrollTop = messagesBox.value.scrollHeight }

async function addMessage(message, persist = true) {
  const active = activeId.value === message.conversation_id
  if (persist) await historyStore.put(message, active && !document.hidden)
  if (!active || entries.value.some(item => Number(item.message.id) === Number(message.id))) return
  const payload = await E2EE.decryptPayload(message.body, message.conversation_id, me.value.id, members.value, message.sender_id)
  const entry = { message, payload, imageUrl: '', imageError: '' }
  entries.value.push(entry)
  if (payload.attachment) loadEntryImage(entry)
  await nextTick(); scrollBottom()
}

function chooseImage(file) {
  const source = file?.file || file
  if (!source) return
  if (!['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(source.type)) return showFailToast('仅支持 JPG、PNG、WebP、GIF 图片')
  if (source.size > 8 * 1024 * 1024) return showFailToast('图片不能超过 8MB')
  clearImage(); imageDraft.value = source; imageDraftURL.value = URL.createObjectURL(source)
}
function clearImage() { if (imageDraftURL.value) URL.revokeObjectURL(imageDraftURL.value); imageDraft.value = null; imageDraftURL.value = '' }
async function uploadAttachment(cipher, conversationID) {
  const response = await fetch(`/api/conversations/${conversationID}/attachments`, { method: 'POST', headers: { Authorization: `Bearer ${token.value}`, 'Content-Type': 'application/octet-stream' }, body: cipher })
  const data = await response.json().catch(() => ({})); if (!response.ok) throw Error(data.error || '图片上传失败'); return data
}
async function sendMessage() {
  let clear = composer.value.trim()
  if (preferences.emojiConvert) clear = clear.replace(/(^|\s):-?\)(?=\s|$)/g, '$1🙂').replace(/(^|\s):-?\((?=\s|$)/g, '$1🙁')
  const draft = imageDraft.value, conversationID = activeId.value
  if ((!clear && !draft) || !conversationID || sending.value) return
  sending.value = true
  let uploadedID = ''
  try {
    let attachment = null
    if (draft) {
      showToast({ message: '正在加密图片…', forbidClick: true, duration: 0 })
      const encrypted = await E2EE.encryptAttachment(draft), uploaded = await uploadAttachment(encrypted.cipher, conversationID)
      uploadedID = uploaded.id; attachment = { id: uploaded.id, ...encrypted.metadata }
    }
    const body = await E2EE.encrypt(clear, conversationID, [...members.value], attachment, replyDraft.value ? { ...replyDraft.value } : null)
    const sent = await api(`/conversations/${conversationID}/messages`, { method: 'POST', body: JSON.stringify({ body }) })
    await addMessage(sent)
    uploadedID = ''; composer.value = ''; replyDraft.value = null; clearImage(); showEmoji.value = false; await loadChats(); showSuccessToast('已发送')
  } catch (error) {
    if (uploadedID) fetch(`/api/conversations/${conversationID}/attachments/${uploadedID}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token.value}` } }).catch(() => {})
    showFailToast(error.message)
  } finally { sending.value = false }
}
function onComposerKeydown(event) { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); sendMessage() } }
function resizeComposer() {
  nextTick(() => {
    const element = composerInput.value
    if (!element) return
    element.style.height = '30px'
    element.style.height = `${Math.max(30, element.scrollHeight)}px`
  })
}
function insertEmoji(value) { composer.value += value; emojiSets.最近 = [value, ...emojiSets.最近.filter(item => item !== value)].slice(0, 24); localStorage.signalRecentEmoji = JSON.stringify(emojiSets.最近) }

function openMessageMenu(entry) { messageMenu.value = entry }
async function copyMessage(entry) { await navigator.clipboard.writeText(entry.payload.body); notify('已复制') }
function replyMessage(entry) { replyDraft.value = { message_id: Number(entry.message.id), sender_name: String(entry.message.sender_name || '联系人').slice(0, 80), body: messageSummary(entry.payload, 160), has_attachment: Boolean(entry.payload.attachment) } }
async function deleteMessage(entry, all) {
  try { await showConfirmDialog({ title: all ? '撤回消息' : '删除消息', message: all ? '在线设备将同步删除本地副本。' : '只从当前设备删除，其他成员仍可看到。' }) } catch { return }
  if (all) { const query = new URLSearchParams({ scope: 'all' }); if (entry.payload.attachment?.id) query.set('attachment', entry.payload.attachment.id); await api(`/conversations/${entry.message.conversation_id}/messages/${entry.message.id}?${query}`, { method: 'DELETE' }) }
  await historyStore.delete(entry.message); entries.value = entries.value.filter(item => item !== entry); await loadChats(); notify(all ? '消息已撤回' : '已从本机删除')
}
async function forwardMessage(conversationID) {
  const entry = forwardEntry.value; if (!entry || forwarding.value) return
  forwarding.value = true
  let uploadedID = ''
  try {
    const targetMembers = await api(`/conversations/${conversationID}/members`)
    let attachment = null
    if (entry.payload.attachment) {
      const response = await fetch(`/api/conversations/${entry.message.conversation_id}/attachments/${entry.payload.attachment.id}`, { headers: { Authorization: `Bearer ${token.value}` } })
      if (!response.ok) throw Error('原图片已不存在')
      const clear = await E2EE.decryptAttachment(await response.arrayBuffer(), entry.payload.attachment)
      const file = new File([clear], entry.payload.attachment.name || '转发图片', { type: entry.payload.attachment.mime })
      const encrypted = await E2EE.encryptAttachment(file), uploaded = await uploadAttachment(encrypted.cipher, conversationID)
      uploadedID = uploaded.id; attachment = { id: uploaded.id, ...encrypted.metadata }
    }
    const body = await E2EE.encrypt(entry.payload.body || '', conversationID, targetMembers, attachment, null)
    await api(`/conversations/${conversationID}/messages`, { method: 'POST', body: JSON.stringify({ body }) })
    uploadedID = ''; showForward.value = false; forwardEntry.value = null; notify('消息已转发')
  } catch (error) { showFailToast(error.message) } finally { forwarding.value = false }
}

async function conversationAction(action, seconds = 0) {
  const chat = conversationMenu.value; if (!chat) return
  conversationMenu.value = null
  if (action === 'clear') return clearConversation(chat.id)
  if (action === 'delete') return deleteConversation(chat.id)
  const body = { action }; if (action === 'mute') body.mute_seconds = seconds
  await api(`/conversations/${chat.id}/state`, { method: 'PUT', body: JSON.stringify(body) }); await loadChats()
}
async function clearConversation(id) {
  try { await showConfirmDialog({ title: '清除双方记录', message: '在线设备立即清除；离线设备下次连接服务器后自动清除。' }) } catch { return }
  await api(`/conversations/${id}/messages`, { method: 'DELETE' }); await historyStore.clear(id); if (activeId.value === id) entries.value = []; await loadChats(); notify('清除指令已同步')
}
async function deleteConversation(id) {
  try { await showConfirmDialog({ title: '删除对话', message: '确定从会话列表中删除？' }) } catch { return }
  await api(`/conversations/${id}`, { method: 'DELETE' }); if (activeId.value === id) activeId.value = 0; await loadChats(); notify('对话已删除')
}
async function createGroup() { const result = await api('/conversations', { method: 'POST', body: JSON.stringify(group) }); showGroup.value = false; Object.assign(group, { name: '', member_ids: [] }); await loadChats(); await openRoom(result.id) }
async function saveProfile() { await api('/me/profile', { method: 'PUT', body: JSON.stringify(profile) }); Object.assign(me.value, profile); showProfile.value = false; notify('个人资料已保存') }
function logout() { localStorage.removeItem('token'); location.reload() }

async function refreshSupport() {
	if (!me.value?.is_support) return
	supportThreads.value = await api('/support/threads')
	if (!supportActive.value) return
	const latestID = supportMessages.value.reduce((max, item) => Math.max(max, Number(item.id) || 0), 0)
	const incoming = await api(`/support/threads/${supportActive.value}/messages?after=${latestID}`)
	const existing = new Set(supportMessages.value.map(item => Number(item.id)))
	supportMessages.value.push(...incoming.filter(item => !existing.has(Number(item.id))))
}
async function applySupportRole(enabled) {
	if (!me.value) return
	me.value.is_support = Boolean(enabled)
	clearInterval(supportTimer)
	supportTimer = null
	if (me.value.is_support) {
		await refreshSupport()
		supportTimer = setInterval(() => refreshSupport().catch(console.warn), 8000)
		return
	}
	supportThreads.value = []
	supportMessages.value = []
	supportActive.value = 0
	if (activeView.value === 'support') activeView.value = 'chats'
}
async function openSupport(id) { activeId.value = 0; supportActive.value = id; supportMessages.value = await api(`/support/threads/${id}/messages`); await nextTick(); scrollBottom() }
async function sendSupport() { const body = supportComposer.value.trim(); if (!body || !supportActive.value || supportBusy.value) return; supportBusy.value = true; try { const message = await api(`/support/threads/${supportActive.value}/messages`, { method: 'POST', body: JSON.stringify({ body }) }); supportMessages.value.push(message); supportComposer.value = ''; await refreshSupport() } finally { supportBusy.value = false } }
async function toggleSupportStatus() { const thread = activeSupportThread.value; if (!thread) return; await api(`/support/threads/${thread.id}/status`, { method: 'PUT', body: JSON.stringify({ status: thread.status === 'open' ? 'closed' : 'open' }) }); await refreshSupport() }

async function ackMessage(id) { await api(`/messages/${id}/ack`, { method: 'POST' }) }
async function applyClear(event) { await historyStore.clear(event.conversation_id); if (activeId.value === event.conversation_id) entries.value = []; if (event.event_id) await api(`/conversation-clears/${event.event_id}/ack`, { method: 'POST' }) }
async function syncOffline() {
  let cleared = 0
  for (let page = 0; page < 20; page++) { const events = await api('/conversation-clears/pending'); if (!events.length) break; for (const event of events) { await applyClear(event); cleared++ } if (events.length < 100) break }
  let received = 0
  for (let page = 0; page < 20; page++) { const messages = await api('/messages/pending'); if (!messages.length) break; for (const message of messages) { await historyStore.put(message, activeId.value === message.conversation_id && !document.hidden); await ackMessage(message.id); received++ } if (messages.length < 100) break }
  if (received || cleared) { await loadChats(); if (activeId.value) await openRoom(activeId.value); notify(received ? `已接收 ${received} 条离线消息` : `已同步 ${cleared} 个清除指令`) }
}
async function notifyIncoming(message) {
  if (message.sender_id === me.value.id || (!document.hidden && activeId.value === message.conversation_id)) return
  try {
    const targetMembers = activeId.value === message.conversation_id ? members.value : await api(`/conversations/${message.conversation_id}/members`)
    const payload = await E2EE.decryptPayload(message.body, message.conversation_id, me.value.id, targetMembers, message.sender_id)
    const body = payload.body || (payload.attachment ? '📷 图片' : '新消息')
    showToast(`${message.sender_name}：${body}`)
    if ('Notification' in window && Notification.permission === 'granted') {
      const registration = await navigator.serviceWorker.ready
      await registration.showNotification(message.sender_name, { body, tag: `conversation-${message.conversation_id}`, icon: '/assets/icon.svg', data: { url: `/?conversation=${message.conversation_id}`, conversation_id: message.conversation_id } })
    }
  } catch (error) { console.warn('notification failed', error) }
}
async function handleSocketEvent(event) {
	if (event.type === 'socket_ready') { connectionState.value = 'connected'; connectionError.value = '' }
	else if (event.type === 'message') { await addMessage(event.data); if (event.data.sender_id !== me.value.id) await ackMessage(event.data.id); await notifyIncoming(event.data); if (activeId.value === event.data.conversation_id && !document.hidden) api(`/conversations/${event.data.conversation_id}/read`, { method: 'POST' }).catch(() => {}); await loadChats() }
	else if (event.type === 'friendship') { await loadFriends(); notify('好友列表已更新') }
	else if (event.type === 'support_message') { await refreshSupport(); if (supportActive.value === Number(event.thread_id) && !supportMessages.value.some(item => Number(item.id) === Number(event.data.id))) supportMessages.value.push(event.data) }
	else if (event.type === 'support_role') { await applySupportRole(event.is_support); notify(event.is_support ? '客服权限已启用' : '客服权限已取消') }
  else if (event.type === 'conversation') {
    if (event.action === 'cleared') await applyClear(event)
    else if (event.action === 'message_deleted' && event.scope === 'all') { await historyStore.deleteIfSender(event.conversation_id, event.message_id, event.actor_id); entries.value = entries.value.filter(item => Number(item.message.id) !== Number(event.message_id)) }
    await loadChats()
  }
}
function scheduleReconnect(reason = '') {
	clearTimeout(reconnectTimer)
	if (!token.value || !navigator.onLine) { connectionState.value = 'offline'; connectionError.value = '网络不可用'; return }
	connectionState.value = 'reconnecting'
	connectionError.value = reason || '实时连接已断开'
	reconnectTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** Math.min(reconnectAttempt++, 5)) + Math.random() * 500)
}
function reconnectNow() {
	clearTimeout(reconnectTimer)
	clearTimeout(socketConnectTimer)
	reconnectAttempt = 0
	connectionError.value = ''
	const current = socket
	socket = null
	current?.close()
	connect()
}
async function connect() {
	clearTimeout(reconnectTimer)
	if (!token.value || !navigator.onLine) return scheduleReconnect()
	if (socket && [WebSocket.OPEN, WebSocket.CONNECTING].includes(socket.readyState)) return
	connectionState.value = socketEverConnected ? 'reconnecting' : 'connecting'
	connectionError.value = ''
	try {
		const issued = await api('/ws-ticket', { method: 'POST' }), scheme = location.protocol === 'https:' ? 'wss' : 'ws'
		const current = new WebSocket(`${scheme}://${location.host}/ws?ticket=${encodeURIComponent(issued.ticket)}`); socket = current
		socketConnectTimer = setTimeout(() => { if (socket === current && current.readyState === WebSocket.CONNECTING) { connectionError.value = '连接服务器超时'; current.close() } }, 12000)
		current.onopen = async () => {
			if (socket !== current) return
			clearTimeout(socketConnectTimer)
			socketEverConnected = true
			reconnectAttempt = 0
			try {
				const latestMe = await api('/me')
				if (Boolean(latestMe.is_support) !== Boolean(me.value?.is_support)) await applySupportRole(latestMe.is_support)
				else if (latestMe.is_support) await refreshSupport()
				await syncOffline()
			} catch (error) { console.warn('realtime sync failed', error) }
		}
		current.onmessage = message => { let data; try { data = JSON.parse(message.data) } catch { return } socketEventChain = socketEventChain.then(() => handleSocketEvent(data)).catch(console.warn) }
		current.onerror = () => { if (socket !== current) return; connectionError.value = '无法建立实时连接'; current.close() }
		current.onclose = () => { clearTimeout(socketConnectTimer); if (socket === current) { socket = null; scheduleReconnect(connectionError.value) } }
	} catch (error) { scheduleReconnect(error.message || '连接服务器失败') }
}

function updateBadge() {
  const count = unreadCount.value
  document.title = count ? `(${count}) Signal Web` : 'Signal Web'
  if ('setAppBadge' in navigator) count ? navigator.setAppBadge(count).catch(() => {}) : navigator.clearAppBadge().catch(() => {})
  if (!window.signalDesktop) return
  if (!count) return window.signalDesktop.setBadge(0, '')
  const canvas = document.createElement('canvas'); canvas.width = 64; canvas.height = 64
  const context = canvas.getContext('2d'); context.fillStyle = '#e53935'; context.beginPath(); context.arc(32, 32, 30, 0, Math.PI * 2); context.fill(); context.fillStyle = '#fff'; context.font = 'bold 28px system-ui'; context.textAlign = 'center'; context.textBaseline = 'middle'; context.fillText(count > 99 ? '99+' : String(count), 32, 33)
  window.signalDesktop.setBadge(count, canvas.toDataURL())
}
function vapidBytes(value) { const padded = (value + '='.repeat((4 - value.length % 4) % 4)).replace(/-/g, '+').replace(/_/g, '/'); return Uint8Array.from(atob(padded), char => char.charCodeAt(0)) }
async function syncPushSubscription() { const config = await api('/push/config'); if (!config.enabled) return false; const registration = await navigator.serviceWorker.ready; let subscription = await registration.pushManager.getSubscription(); if (!subscription) subscription = await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: vapidBytes(config.public_key) }); await api('/push/subscriptions', { method: 'POST', body: JSON.stringify(subscription.toJSON()) }); pushEnabled.value = true; return true }
async function enableNotifications() { if (!('Notification' in window) || !('serviceWorker' in navigator)) return showFailToast('当前环境不支持后台推送'); const permission = Notification.permission === 'granted' ? 'granted' : await Notification.requestPermission(); if (permission !== 'granted') return showFailToast('未获得通知权限'); await syncPushSubscription(); notify('后台通知已开启') }
async function installApp() { if (installPrompt.value) { await installPrompt.value.prompt(); installPrompt.value = null } else showToast(/iPhone|iPad/.test(navigator.userAgent) ? '请使用“分享 → 添加到主屏幕”' : '请使用浏览器菜单安装应用') }

watch(friendQuery, () => { clearTimeout(friendSearchTimer); friendSearchTimer = setTimeout(() => searchUsers().catch(error => showFailToast(error.message)), 220) })
watch(activeView, view => { if (view === 'support') refreshSupport() })
watch(unreadCount, updateBadge)
watch(composer, resizeComposer)
watch([theme, zoom], applyAppearance)
watch(preferences, value => { localStorage.signalChatPreferences = JSON.stringify(value) }, { deep: true })

onMounted(() => {
  applyAppearance()
  window.addEventListener('online', connect)
	window.addEventListener('offline', () => { connectionState.value = 'offline'; connectionError.value = '网络不可用' })
  window.addEventListener('beforeinstallprompt', event => { event.preventDefault(); installPrompt.value = event })
  document.addEventListener('visibilitychange', () => { if (!document.hidden) connect() })
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').then(registration => registration.update()).catch(() => {})
    navigator.serviceWorker.addEventListener('message', event => { if (event.data?.type === 'open-conversation' && event.data.conversation_id) openRoom(Number(event.data.conversation_id)) })
  }
  if (token.value) start().catch(error => { authError.value = error.message })
})
onBeforeUnmount(() => { clearInterval(supportTimer); clearTimeout(reconnectTimer); clearTimeout(socketConnectTimer); socket?.close(); revokeImages(); clearImage() })
</script>

<template>
  <div v-if="!loggedIn" class="auth-page">
    <section class="auth-card">
      <div class="brand-mark">S</div><h1>Signal Web</h1><p>安静、专注的即时通讯</p>
      <van-form @submit="submitAuth">
        <van-cell-group inset>
          <van-field v-if="mode==='register'" v-model="auth.display_name" label="昵称" placeholder="请输入昵称" :rules="[{ required:true }]" />
          <van-field v-if="mode==='register'" v-model="auth.about" label="简介" maxlength="160" placeholder="选填" />
          <van-field v-model="auth.username" label="用户名" autocomplete="username" placeholder="请输入用户名" :rules="[{ required:true }]" />
          <van-field v-model="auth.password" type="password" label="密码" autocomplete="current-password" placeholder="请输入密码" :rules="[{ required:true }]" />
        </van-cell-group>
        <div class="auth-actions"><van-button block round type="primary" native-type="submit" :loading="authBusy">{{ mode==='register'?'注册':'登录' }}</van-button><van-button block plain round type="primary" @click="mode=mode==='login'?'register':'login'">{{ mode==='login'?'没有账号？注册':'已有账号？登录' }}</van-button></div>
      </van-form>
      <p v-if="authError" class="error">{{ authError }}</p>
      <div class="downloads"><a href="/downloads/SignalWeb-Windows-x64-v1.2.0.zip">Windows</a><a href="/downloads/SignalWeb-Android-v1.0.0.apk">Android</a><a href="/downloads/SignalWeb-iOS.mobileconfig">iPhone</a></div>
    </section>
  </div>

  <main v-else class="signal-shell" :class="{ 'mobile-room-open': mobileRoomOpen }">
    <nav class="app-rail" aria-label="主导航">
      <div class="window-dots"><i></i><i></i><i></i></div>
      <van-button class="rail-menu" icon="bars" round aria-label="菜单" />
      <van-badge :content="unreadCount||undefined" :show-zero="false"><van-button :class="{active:activeView==='chats'||activeView==='new-chat'}" icon="chat-o" round @click="selectView('chats')" aria-label="聊天" /></van-badge>
      <van-badge :content="requests.length||undefined" :show-zero="false"><van-button :class="{active:activeView==='friends'}" icon="friends-o" round @click="selectView('friends')" aria-label="联系人" /></van-badge>
      <van-button icon="phone-o" round @click="showToast('当前版本暂未启用音视频通话')" aria-label="通话" />
      <van-button v-if="me.is_support" :class="{active:activeView==='support'}" icon="service-o" round @click="selectView('support')" aria-label="客服" />
      <van-button class="rail-settings" :class="{active:activeView==='settings'}" icon="setting-o" round @click="selectView('settings')" aria-label="设置" />
    </nav>

    <aside class="list-pane">
      <template v-if="activeView==='settings'">
        <header class="panel-title"><h1>设置</h1></header>
        <button class="settings-profile" @click="openSetting('profile')"><span class="avatar large">{{ initials(me.display_name) }}</span><span><b>{{ me.display_name }}</b><small>@{{ me.username }}</small><small>{{ me.about||'编辑个人资料' }}</small></span><van-icon name="qr" /></button>
        <div class="settings-menu side-scroll">
          <button :class="{active:settingsSection==='profile'}" @click="openSetting('profile')"><van-icon name="contact"/>账户</button>
          <button :class="{active:settingsSection==='general'}" @click="openSetting('general')"><van-icon name="setting-o"/>通用</button>
          <button :class="{active:settingsSection==='appearance'}" @click="openSetting('appearance')"><van-icon name="eye-o"/>外观</button>
          <button :class="{active:settingsSection==='chat'}" @click="openSetting('chat')"><van-icon name="chat-o"/>聊天</button>
          <button :class="{active:settingsSection==='notifications'}" @click="openSetting('notifications')"><van-icon name="bell"/>提醒</button>
          <button :class="{active:settingsSection==='privacy'}" @click="openSetting('privacy')"><van-icon name="lock"/>隐私</button>
          <button :class="{active:settingsSection==='data'}" @click="openSetting('data')"><van-icon name="bar-chart-o"/>数据使用量</button>
          <button :class="{active:settingsSection==='backup'}" @click="openSetting('backup')"><van-icon name="underway-o"/>备份</button>
          <button class="logout-setting" @click="logout"><van-icon name="revoke"/>退出登录</button>
        </div>
      </template>

      <template v-else-if="activeView==='new-chat'">
        <header class="panel-title"><van-button icon="arrow-left" round @click="selectView('chats')"/><h1>新聊天</h1></header>
        <van-search v-model="friendQuery" autofocus placeholder="姓名或用户名" />
        <section class="side-scroll new-chat-list">
          <button class="quick-action" @click="showGroup=true"><span><van-icon name="cluster-o"/></span>新建群组</button>
          <button class="quick-action" @click="showFriendSearch=true"><span><van-icon name="contact"/></span>按用户名查找</button>
          <h3>联系人</h3>
          <button v-for="user in newChatFriends" :key="user.id" class="contact-row" @click="createDirect(user.id)"><span class="avatar">{{ initials(user.display_name) }}</span><span><b>{{ user.display_name }}</b><small>@{{ user.username }}</small></span></button>
          <van-cell v-for="user in userResults" :key="`result-${user.id}`" :title="user.display_name" :label="`@${user.username}`"><template #right-icon><van-button size="small" type="primary" @click="relationshipAction(user)">{{ {none:'添加',outgoing:'取消申请',incoming:'去确认',friend:'发消息'}[user.relationship] }}</van-button></template></van-cell>
        </section>
      </template>

      <template v-else>
        <header class="panel-title"><h1>{{ activeView==='chats'?'聊天':activeView==='friends'?'联系人':'客服' }}</h1><div><van-button v-if="activeView==='chats'" icon="edit" round @click="openNewChat"/><van-button icon="ellipsis" round @click="activeView==='chats'&&(showArchived=!showArchived)"/></div></header>
        <div class="panel-search"><van-search v-model="search" :placeholder="activeView==='friends'?'搜索联系人':activeView==='chats'?'搜索聊天':'搜索客服会话'"/><van-button v-if="activeView==='chats'" :class="{active:unreadOnly}" icon="filter-o" round type="primary" @click="unreadOnly=!unreadOnly"/></div>
        <section v-if="activeView==='chats'" class="side-scroll">
          <div v-if="unreadOnly" class="filter-title"><b>按未读筛选</b><button @click="unreadOnly=false">清除筛选</button></div>
          <van-cell v-if="archivedCount||showArchived" is-link :title="showArchived?'返回消息':'已存档'" :value="showArchived?'':archivedCount" @click="showArchived=!showArchived" />
          <van-swipe-cell v-for="chat in filteredChats" :key="chat.id"><button class="chat-row" :class="{active:activeId===chat.id}" @click="openRoom(chat.id)" @contextmenu.prevent="conversationMenu=chat"><span class="avatar">{{ initials(chat.name) }}</span><span class="chat-copy"><span><b>{{ chat.name||'对话' }}</b><van-badge v-if="chat.unread" :content="chat.unread" /></span><small>{{ chat.last_message?'端到端加密消息':'开始聊天' }}</small></span><van-icon v-if="chat.pinned" name="star"/><van-icon v-if="chat.muted" name="volume-o"/></button><template #right><van-button square type="primary" text="操作" class="swipe-action" @click="conversationMenu=chat" /></template></van-swipe-cell>
          <div v-if="!filteredChats.length" class="list-empty"><van-icon name="chat-o"/><p>{{ unreadOnly?'没有未读的聊天记录':showArchived?'没有已存档会话':'还没有聊天' }}</p><van-button v-if="unreadOnly" round @click="unreadOnly=false">清除筛选</van-button><van-button v-else round type="primary" @click="openNewChat">发起新聊天</van-button></div>
        </section>
        <section v-else-if="activeView==='friends'" class="side-scroll"><template v-if="requests.length"><h3>好友申请</h3><van-cell v-for="user in requests" :key="user.id" :title="user.display_name" :label="`@${user.username}`"><template #right-icon><van-space><van-button size="mini" type="primary" @click="acceptFriend(user.id)">接受</van-button><van-button size="mini" type="danger" plain @click="removeFriend(user,true)">拒绝</van-button></van-space></template></van-cell></template><h3>联系人</h3><van-swipe-cell v-for="user in filteredFriends" :key="user.id"><button class="contact-row" @click="createDirect(user.id)"><span class="avatar">{{ initials(user.display_name) }}</span><span><b>{{ user.display_name }}</b><small>@{{ user.username }}</small></span></button><template #right><van-button square type="danger" text="删除" class="swipe-action" @click="removeFriend(user)"/></template></van-swipe-cell><van-empty v-if="!filteredFriends.length" description="还没有联系人" /></section>
        <section v-else class="side-scroll"><button v-for="thread in supportThreads" :key="thread.id" class="chat-row" :class="{active:supportActive===thread.id}" @click="openSupport(thread.id)"><span class="avatar">客</span><span class="chat-copy"><span><b>{{ thread.visitor_name||'游客' }}</b><van-tag :type="thread.status==='open'?'success':'default'">{{ thread.status==='open'?'进行中':'已结束' }}</van-tag></span><small>{{ thread.site_name }} · {{ thread.last_message||'等待游客消息' }}</small></span></button><van-empty v-if="!supportThreads.length" description="暂无游客咨询" /></section>
      </template>
    </aside>

	<section class="conversation-pane">
	  <div v-if="connectionState!=='connected'&&(activeId||supportActive)" class="connection-banner" :class="connectionState"><van-icon name="warning-o"/><span><b>{{ connectionState==='offline'?'已离线':connectionState==='connecting'?'正在连接服务器':'实时连接已断开' }}</b><small>{{ connectionError||'正在尝试恢复连接' }}</small></span><button @click="reconnectNow">立即重连</button></div>
      <template v-if="activeView==='settings'">
        <header class="settings-content-title"><van-button class="mobile-only" icon="arrow-left" round @click="settingsMobileOpen=false"/><h2>{{ {profile:'个人资料',general:'通用',appearance:'外观',chat:'聊天',notifications:'提醒',privacy:'隐私',data:'数据使用量',backup:'备份'}[settingsSection] }}</h2></header>
        <div class="settings-content">
          <section v-if="settingsSection==='profile'" class="settings-page profile-page"><div class="profile-avatar"><span class="avatar huge">{{ initials(me.display_name) }}</span><van-button round size="small" @click="showProfile=true">编辑资料</van-button></div><div class="setting-card"><van-field v-model="profile.display_name" label="昵称" maxlength="80" left-icon="contact"/><van-field v-model="profile.about" label="关于" maxlength="160" left-icon="edit"/></div><p class="setting-help">您的个人资料以及对其所做的更改将对联系人和群组可见。</p><div class="setting-card"><van-cell icon="label-o" title="用户名" :value="`@${me.username}`"/><van-cell icon="qr" title="二维码或链接" is-link @click="showToast('用户名：@'+me.username)"/></div><van-button type="primary" round @click="saveProfile">保存个人资料</van-button></section>

          <section v-else-if="settingsSection==='general'" class="settings-page"><div class="setting-card"><van-cell title="用户名" :value="`@${me.username}`"/><van-cell title="设备名称" :value="window?.signalDesktop?'Windows 客户端':'网页端'"/></div><h3>系统</h3><div class="setting-card"><van-cell title="安装到桌面" is-link @click="installApp"/><van-cell title="退出当前账号" is-link @click="logout"/></div></section>

          <section v-else-if="settingsSection==='appearance'" class="settings-page"><div class="setting-card"><van-cell icon="globe-o" title="语言" value="简体中文"/><van-cell icon="eye-o" title="主题"><template #value><van-radio-group v-model="theme" direction="horizontal"><van-radio name="dark">深色</van-radio><van-radio name="light">浅色</van-radio></van-radio-group></template></van-cell><van-cell icon="flower-o" title="聊天颜色"><template #value><span class="accent-dot"></span></template></van-cell><van-field v-model="zoom" type="number" label="缩放级别" input-align="right" suffix="%" min="85" max="125"/></div><p class="setting-help">外观设置只保存在当前设备。</p></section>

          <section v-else-if="settingsSection==='chat'" class="settings-page"><div class="setting-card"><van-cell center title="继续存档静音聊天" label="收到新消息时仍保持存档"><template #right-icon><van-switch v-model="preferences.keepMutedArchived"/></template></van-cell></div><h3>文本输入</h3><div class="setting-card"><van-cell center title="检查消息输入框中的文字拼写"><template #right-icon><van-switch v-model="preferences.spellcheck"/></template></van-cell><van-cell center title="将文字表情转换为表情符号"><template #right-icon><van-switch v-model="preferences.emojiConvert"/></template></van-cell><van-cell center title="生成链接预览"><template #right-icon><van-switch v-model="preferences.linkPreview"/></template></van-cell></div></section>

          <section v-else-if="settingsSection==='notifications'" class="settings-page"><div class="setting-card"><van-cell center title="桌面消息通知" label="允许在应用处于后台时接收提醒"><template #right-icon><van-switch :model-value="pushEnabled" @click="enableNotifications"/></template></van-cell><van-cell title="安装应用" label="获得更稳定的桌面通知" is-link @click="installApp"/></div></section>

          <section v-else-if="settingsSection==='privacy'" class="settings-page"><h3>消息传输</h3><div class="setting-card"><van-cell center title="已读回执"><template #right-icon><van-switch v-model="preferences.readReceipts"/></template></van-cell><van-cell center title="“正在输入”提示"><template #right-icon><van-switch v-model="preferences.typingIndicators"/></template></van-cell></div><p class="setting-help">这些偏好设置仅影响当前设备界面，不改变端到端加密机制。</p></section>

          <section v-else-if="settingsSection==='data'" class="settings-page"><div class="setting-card"><van-cell icon="photo-o" title="图片上传上限" value="8 MB"/><van-cell icon="records-o" title="消息历史" value="仅存储在本机"/><van-cell icon="shield-o" title="传输方式" value="端到端加密"/></div></section>

          <section v-else class="settings-page"><div class="setting-card"><van-cell icon="underway-o" title="本地聊天记录" label="聊天记录保存在当前设备的本地数据库中"/><van-cell icon="desktop-o" title="Windows 数据" label="Windows 客户端使用独立本地持久化存储"/></div><p class="setting-help">清除浏览器或应用数据会移除本机历史记录，请妥善保管设备。</p></section>
        </div>
      </template>

      <div v-else-if="activeView==='new-chat'||(!activeId&&!supportActive)" class="empty-state"><van-icon name="chat-o" size="82"/><h2>欢迎使用 Signal</h2><p>{{ activeView==='new-chat'?'选择联系人开始聊天':'从左侧选择一段对话' }}</p></div>
      <template v-else-if="activeId">
		<van-nav-bar :title="activeChat?.name||'对话'" left-arrow @click-left="activeId=0" />
        <div ref="messagesBox" class="messages" @scroll="onMessageScroll">
          <button v-if="!historyDone" class="history-button" @click="loadOlder">{{ historyLoading?'正在读取…':'上滑加载更早消息' }}</button>
          <template v-for="(entry,index) in entries" :key="entry.message.id">
            <div v-if="index===0||dayKey(entries[index-1].message.created_at)!==dayKey(entry.message.created_at)" class="day-label">{{ dayLabel(entry.message.created_at) }}</div>
            <article class="message" :class="{mine:entry.message.sender_id===me.id}" @contextmenu.prevent="openMessageMenu(entry)" @dblclick="openMessageMenu(entry)">
              <small>{{ entry.message.sender_name }} · {{ new Date(entry.message.created_at).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'}) }}</small>
              <button v-if="entry.payload.reply" class="reply-quote">↩ {{ entry.payload.reply.sender_name }} · {{ entry.payload.reply.body }}</button>
              <p v-if="entry.payload.body">{{ entry.payload.body }}</p>
              <van-image v-if="entry.imageUrl" :src="entry.imageUrl" fit="cover" radius="10" @click="showImagePreview([entry.imageUrl])" />
              <span v-else-if="entry.payload.attachment" class="image-loading">{{ entry.imageError||'🔒 正在解密图片…' }}</span>
              <van-button class="message-more" icon="ellipsis" size="mini" plain @click="openMessageMenu(entry)" />
            </article>
          </template>
        </div>
        <div class="composer-wrap">
          <div v-if="replyDraft" class="draft"><span><b>引用 {{ replyDraft.sender_name }}</b>{{ replyDraft.body }}</span><van-icon name="cross" @click="replyDraft=null"/></div>
          <div v-if="imageDraftURL" class="draft image-draft"><img :src="imageDraftURL"><span>{{ imageDraft.name }}</span><van-icon name="cross" @click="clearImage"/></div>
          <div v-if="showEmoji" class="emoji-panel"><van-tabs v-model:active="emojiCategory" shrink><van-tab v-for="(_,name) in emojiSets" :key="name" :name="name" :title="name"/></van-tabs><div><button v-for="emoji in currentEmoji" :key="emoji" @click="insertEmoji(emoji)">{{ emoji }}</button></div></div>
          <div class="composer"><van-button class="composer-tool" icon="smile-o" @click="showEmoji=!showEmoji"/><van-button class="composer-tool" icon="photograph" @click="imageInput.click()"/><textarea ref="composerInput" v-model="composer" rows="1" placeholder="发送消息" :spellcheck="preferences.spellcheck" @input="resizeComposer" @keydown="onComposerKeydown"/><input ref="imageInput" hidden type="file" accept="image/jpeg,image/png,image/webp,image/gif" @change="chooseImage($event.target.files?.[0])"></div>
        </div>
      </template>

      <template v-else>
        <van-nav-bar :title="activeSupportThread?.visitor_name||'游客咨询'" left-arrow @click-left="supportActive=0"><template #right><van-button size="small" :type="activeSupportThread?.status==='open'?'danger':'primary'" @click="toggleSupportStatus">{{ activeSupportThread?.status==='open'?'结束咨询':'重新开启' }}</van-button></template></van-nav-bar>
        <div class="messages"><article v-for="message in supportMessages" :key="message.id" class="message" :class="{mine:message.sender_type==='agent',system:message.sender_type==='system'}"><small>{{ message.sender_name||message.sender_type }}</small><p>{{ message.body }}</p></article></div>
        <div v-if="activeSupportThread?.status==='open'" class="composer-wrap"><div class="composer"><textarea v-model="supportComposer" maxlength="2000" placeholder="回复游客" @keydown.enter.exact.prevent="sendSupport"/><van-button icon="guide-o" round type="primary" :loading="supportBusy" @click="sendSupport"/></div></div>
      </template>
    </section>
  </main>

  <van-popup v-model:show="showFriendSearch" position="bottom" round class="sheet"><van-nav-bar title="添加好友" left-text="关闭" @click-left="showFriendSearch=false"/><van-search v-model="friendQuery" autofocus placeholder="搜索用户名或昵称"/><van-cell v-for="user in userResults" :key="user.id" :title="user.display_name" :label="`@${user.username}`"><template #right-icon><van-button size="small" type="primary" @click="relationshipAction(user)">{{ {none:'添加',outgoing:'取消申请',incoming:'去确认',friend:'发消息'}[user.relationship] }}</van-button></template></van-cell><van-empty v-if="friendQuery&&!userResults.length" description="没有找到用户"/></van-popup>
  <van-popup v-model:show="showProfile" position="bottom" round class="sheet"><van-nav-bar title="个人资料" left-text="取消" right-text="保存" @click-left="showProfile=false" @click-right="saveProfile"/><van-cell-group inset><van-field :model-value="`@${me?.username||''}`" label="用户名" readonly/><van-field v-model="profile.display_name" label="昵称" maxlength="80"/><van-field v-model="profile.about" label="个人简介" maxlength="160" type="textarea" autosize/></van-cell-group></van-popup>
  <van-popup v-model:show="showGroup" position="bottom" round class="sheet"><van-nav-bar title="新建群聊" left-text="取消" right-text="创建" @click-left="showGroup=false" @click-right="createGroup"/><van-cell-group inset><van-field v-model="group.name" label="群名称" placeholder="请输入群聊名称"/><van-checkbox-group v-model="group.member_ids"><van-cell v-for="user in friends" :key="user.id" :title="user.display_name" clickable @click="group.member_ids.includes(user.id)?group.member_ids.splice(group.member_ids.indexOf(user.id),1):group.member_ids.push(user.id)"><template #right-icon><van-checkbox :name="user.id"/></template></van-cell></van-checkbox-group></van-cell-group></van-popup>
  <van-action-sheet :show="Boolean(conversationMenu)" title="会话操作" cancel-text="取消" @cancel="conversationMenu=null" @close="conversationMenu=null"><div class="action-grid"><van-button @click="conversationAction('mark_unread')">标记未读</van-button><van-button @click="conversationAction(conversationMenu?.pinned?'unpin':'pin')">{{ conversationMenu?.pinned?'取消置顶':'置顶' }}</van-button><van-button @click="conversationAction(conversationMenu?.archived?'unarchive':'archive')">{{ conversationMenu?.archived?'取消存档':'存档' }}</van-button><van-button @click="conversationAction(conversationMenu?.muted?'unmute':'mute',3600)">{{ conversationMenu?.muted?'取消静音':'静音 1 小时' }}</van-button><van-button type="warning" @click="conversationAction('clear')">清除双方记录</van-button><van-button type="danger" @click="conversationAction('delete')">删除对话</van-button></div></van-action-sheet>
  <van-action-sheet :show="Boolean(messageMenu)" title="消息操作" cancel-text="取消" @cancel="messageMenu=null" @close="messageMenu=null"><div class="action-grid"><van-button @click="replyMessage(messageMenu);messageMenu=null">引用</van-button><van-button @click="forwardEntry=messageMenu;showForward=true;messageMenu=null">转发</van-button><van-button v-if="messageMenu?.payload.body" @click="copyMessage(messageMenu);messageMenu=null">复制文字</van-button><van-button type="warning" @click="deleteMessage(messageMenu,false);messageMenu=null">仅本机删除</van-button><van-button v-if="messageMenu?.message.sender_id===me?.id" type="danger" @click="deleteMessage(messageMenu,true);messageMenu=null">撤回双方</van-button></div></van-action-sheet>
  <van-popup v-model:show="showForward" position="bottom" round class="sheet"><van-nav-bar title="转发到" left-text="取消" @click-left="showForward=false"/><van-cell v-for="chat in chats" :key="chat.id" is-link :title="chat.name" label="端到端加密转发" @click="forwardMessage(chat.id)"/></van-popup>
</template>
