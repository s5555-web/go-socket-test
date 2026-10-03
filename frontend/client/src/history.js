export function createLocalHistory(getMe) {
  const DB = 'signal-web-local-history'
  const STORE = 'messages'
  const PAGE = 40
  const native = window.signalDesktop?.history
  let dbPromise

  const account = () => String(getMe()?.id || '')
  const conversationKey = id => `${account()}:${Number(id)}`
  const recordKey = (conversationID, messageID) => `${conversationKey(conversationID)}:${messageID}`
  const sortKey = message => `${String(new Date(message.created_at).getTime()).padStart(16, '0')}:${String(message.id).padStart(16, '0')}`
  const mirrorKey = id => `signalHistory:${account()}:${Number(id)}`

  function db() {
    if (!dbPromise) dbPromise = new Promise((resolve, reject) => {
      const request = indexedDB.open(DB, 1)
      request.onupgradeneeded = () => {
        const store = request.result.createObjectStore(STORE, { keyPath: 'key' })
        store.createIndex('conversation', 'conversation', { unique: false })
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    return dbPromise
  }

  async function write(mode, work) {
    const database = await db()
    return new Promise((resolve, reject) => {
      const transaction = database.transaction(STORE, mode)
      work(transaction.objectStore(STORE), transaction)
      transaction.oncomplete = () => resolve()
      transaction.onerror = () => reject(transaction.error)
      transaction.onabort = () => reject(transaction.error)
    })
  }

  function readMirror(id) {
    try {
      const value = JSON.parse(localStorage.getItem(mirrorKey(id)) || '[]')
      return Array.isArray(value) ? value : []
    } catch { return [] }
  }

  function saveMirror(id, records) {
    let kept = records.sort((a, b) => a.sort.localeCompare(b.sort)).slice(-300)
    while (kept.length) {
      try {
        const value = JSON.stringify(kept)
        if (value.length > 1500000) { kept = kept.slice(Math.ceil(kept.length / 2)); continue }
        localStorage.setItem(mirrorKey(id), value)
        return
      } catch { kept = kept.slice(Math.ceil(kept.length / 2)) }
    }
    localStorage.removeItem(mirrorKey(id))
  }

  async function indexedPut(message, read) {
    const record = { key: recordKey(message.conversation_id, message.id), conversation: conversationKey(message.conversation_id), sort: sortKey(message), read, message }
    return write('readwrite', store => store.put(record))
  }

  async function indexedPage(conversationID, before = '', limit = PAGE) {
    const database = await db()
    return new Promise((resolve, reject) => {
      const transaction = database.transaction(STORE, 'readonly')
      const index = transaction.objectStore(STORE).index('conversation')
      const items = []
      const request = index.openCursor(IDBKeyRange.only(conversationKey(conversationID)), 'prev')
      request.onsuccess = () => {
        const cursor = request.result
        if (!cursor || items.length >= limit) return resolve(items.reverse())
        if (!before || cursor.value.sort < before) items.push(cursor.value)
        cursor.continue()
      }
      request.onerror = () => reject(request.error)
    })
  }

  async function updateConversation(id, updater) {
    return write('readwrite', store => {
      const request = store.index('conversation').openCursor(IDBKeyRange.only(conversationKey(id)))
      request.onsuccess = () => {
        const cursor = request.result
        if (!cursor) return
        if (updater(cursor.value)) cursor.update(cursor.value)
        cursor.continue()
      }
    })
  }

  async function put(message, readState = false) {
    const read = readState || message.sender_id === getMe()?.id
    const record = { sort: sortKey(message), read, message }
    saveMirror(message.conversation_id, [...readMirror(message.conversation_id).filter(item => String(item.message?.id) !== String(message.id)), record])
    const writes = [indexedPut(message, read)]
    if (native) writes.push(native.put({ accountId: getMe().id, message, read, sort: record.sort }))
    const settled = await Promise.allSettled(writes)
    if (!readMirror(message.conversation_id).some(item => String(item.message?.id) === String(message.id)) && !settled.some(item => item.status === 'fulfilled')) throw Error('无法写入本地消息记录')
  }

  async function page(conversationID, before = '', limit = PAGE) {
    const reads = [indexedPage(conversationID, before, limit)]
    if (native) reads.push(native.page({ accountId: getMe().id, conversationId: conversationID, before, limit }))
    const settled = await Promise.allSettled(reads)
    const merged = new Map()
    for (const item of [...settled.flatMap(result => result.status === 'fulfilled' ? result.value : []), ...readMirror(conversationID)]) {
      if (!before || item.sort < before) merged.set(String(item.message.id), item)
    }
    return [...merged.values()].sort((a, b) => a.sort.localeCompare(b.sort)).slice(-limit)
  }

  async function summaries(ids) {
    const result = new Map()
    for (const id of ids) {
      const records = await page(id, '', 300)
      result.set(Number(id), { latest: records.at(-1)?.message || null, unread: records.filter(item => !item.read && item.message?.sender_id !== getMe()?.id).length })
    }
    return result
  }

  async function markRead(id) {
    saveMirror(id, readMirror(id).map(item => ({ ...item, read: true })))
    const writes = [updateConversation(id, item => { item.read = true; return true })]
    if (native) writes.push(native.markRead({ accountId: getMe().id, conversationId: id }))
    await Promise.allSettled(writes)
  }

  async function remove(message) {
    saveMirror(message.conversation_id, readMirror(message.conversation_id).filter(item => String(item.message?.id) !== String(message.id)))
    const writes = [write('readwrite', store => store.delete(recordKey(message.conversation_id, message.id)))]
    if (native) writes.push(native.delete({ accountId: getMe().id, conversationId: message.conversation_id, messageId: message.id }))
    await Promise.allSettled(writes)
  }

  async function deleteIfSender(conversationID, messageID, senderID) {
    const records = readMirror(conversationID)
    const found = records.some(item => String(item.message?.id) === String(messageID) && item.message?.sender_id === senderID)
    if (found) saveMirror(conversationID, records.filter(item => String(item.message?.id) !== String(messageID)))
    const database = await db()
    const indexed = new Promise((resolve, reject) => {
      const transaction = database.transaction(STORE, 'readwrite')
      const store = transaction.objectStore(STORE)
      const request = store.get(recordKey(conversationID, messageID))
      let removed = false
      request.onsuccess = () => { if (request.result?.message?.sender_id === senderID) { store.delete(request.result.key); removed = true } }
      transaction.oncomplete = () => resolve(removed)
      transaction.onerror = () => reject(transaction.error)
    })
    const writes = [indexed]
    if (native) writes.push(native.deleteIfSender({ accountId: getMe().id, conversationId: conversationID, messageId: messageID, senderId: senderID }))
    const settled = await Promise.allSettled(writes)
    return found || settled.some(item => item.status === 'fulfilled' && item.value)
  }

  async function clear(id) {
    localStorage.removeItem(mirrorKey(id))
    const indexed = write('readwrite', store => {
      const request = store.index('conversation').openKeyCursor(IDBKeyRange.only(conversationKey(id)))
      request.onsuccess = () => { const cursor = request.result; if (cursor) { store.delete(cursor.primaryKey); cursor.continue() } }
    })
    const writes = [indexed]
    if (native) writes.push(native.clear({ accountId: getMe().id, conversationId: id }))
    await Promise.allSettled(writes)
  }

  navigator.storage?.persist?.().catch(() => {})
  return { PAGE, put, page, summaries, markRead, delete: remove, deleteIfSender, clear, sortKey }
}
