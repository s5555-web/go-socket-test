const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const test = require('node:test');
const { HistoryStore } = require('./history-store');

const message = (id, sender = 2) => ({
  id,
  conversation_id: 7,
  sender_id: sender,
  sender_name: 'Test',
  body: '{"v":2,"ciphertext":"encrypted"}',
  created_at: new Date(1700000000000 + id).toISOString(),
});

test('persists and reloads paginated history', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'signal-history-'));
  const file = path.join(directory, 'history.json');
  const store = new HistoryStore(file);
  await store.put({ accountId: 1, message: message(1), sort: '0001', read: false });
  await store.put({ accountId: 1, message: message(2, 1), sort: '0002', read: true });

  const reloaded = new HistoryStore(file);
  assert.deepEqual(reloaded.page({ accountId: 1, conversationId: 7, limit: 1 }).map(item => item.message.id), [2]);
  assert.equal(reloaded.summaries({ accountId: 1, conversationIds: [7] })[7].unread, 1);
  await reloaded.markRead({ accountId: 1, conversationId: 7 });
  assert.equal(new HistoryStore(file).summaries({ accountId: 1, conversationIds: [7] })[7].unread, 0);
});

test('isolates accounts and validates synchronized deletion sender', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'signal-history-'));
  const file = path.join(directory, 'history.json');
  const store = new HistoryStore(file);
  await store.put({ accountId: 1, message: message(3), sort: '0003' });
  await store.put({ accountId: 9, message: message(3), sort: '0003' });
  assert.equal(await store.deleteIfSender({ accountId: 1, conversationId: 7, messageId: 3, senderId: 99 }), false);
  assert.equal(await store.deleteIfSender({ accountId: 1, conversationId: 7, messageId: 3, senderId: 2 }), true);
  assert.equal(store.page({ accountId: 1, conversationId: 7 }).length, 0);
  assert.equal(store.page({ accountId: 9, conversationId: 7 }).length, 1);
});
