const fs = require('fs');
const path = require('path');

const MAX_MESSAGES = 50000;
const MAX_BODY_BYTES = 524288;

function cleanID(value) {
  const id = String(value ?? '');
  if (!/^\d{1,20}$/.test(id) || id === '0') throw new Error('invalid id');
  return id;
}

function cleanMessage(value) {
  if (!value || typeof value !== 'object') throw new Error('invalid message');
  const body = String(value.body ?? '');
  if (!body || Buffer.byteLength(body, 'utf8') > MAX_BODY_BYTES) throw new Error('invalid message body');
  const createdAt = new Date(value.created_at);
  if (Number.isNaN(createdAt.getTime())) throw new Error('invalid message time');
  return {
    id: Number(cleanID(value.id)),
    conversation_id: Number(cleanID(value.conversation_id)),
    sender_id: Number(cleanID(value.sender_id)),
    sender_name: String(value.sender_name ?? '').slice(0, 80),
    body,
    created_at: createdAt.toISOString(),
  };
}

class HistoryStore {
  constructor(filePath) {
    this.filePath = filePath;
    this.data = { version: 1, records: {} };
    this.queue = Promise.resolve();
    this.load();
  }

  load() {
    try {
      const parsed = JSON.parse(fs.readFileSync(this.filePath, 'utf8'));
      if (parsed?.version === 1 && parsed.records && typeof parsed.records === 'object') this.data = parsed;
    } catch (error) {
      if (error.code !== 'ENOENT') console.warn('[history] unable to read local history:', error.message);
    }
  }

  records(accountId, conversationId) {
    const prefix = `${cleanID(accountId)}:${cleanID(conversationId)}:`;
    return Object.entries(this.data.records)
      .filter(([key]) => key.startsWith(prefix))
      .map(([, value]) => value)
      .sort((a, b) => a.sort.localeCompare(b.sort));
  }

  persist() {
    const directory = path.dirname(this.filePath);
    const temporary = `${this.filePath}.tmp`;
    fs.mkdirSync(directory, { recursive: true });
    fs.writeFileSync(temporary, JSON.stringify(this.data), { encoding: 'utf8', mode: 0o600 });
    fs.renameSync(temporary, this.filePath);
  }

  mutate(work) {
    this.queue = this.queue.catch(() => {}).then(() => {
      const result = work();
      const entries = Object.entries(this.data.records);
      if (entries.length > MAX_MESSAGES) {
        entries.sort((a, b) => a[1].sort.localeCompare(b[1].sort));
        for (const [key] of entries.slice(0, entries.length - MAX_MESSAGES)) delete this.data.records[key];
      }
      this.persist();
      return result;
    });
    return this.queue;
  }

  put(input) {
    return this.mutate(() => {
      const accountId = cleanID(input.accountId);
      const message = cleanMessage(input.message);
      const key = `${accountId}:${message.conversation_id}:${message.id}`;
      this.data.records[key] = {
        sort: String(input.sort ?? '').slice(0, 64),
        read: Boolean(input.read) || String(message.sender_id) === accountId,
        message,
      };
      return true;
    });
  }

  page(input) {
    const before = String(input.before ?? '');
    const limit = Math.max(1, Math.min(100, Number(input.limit) || 40));
    return this.records(input.accountId, input.conversationId)
      .filter(record => !before || record.sort < before)
      .slice(-limit);
  }

  summaries(input) {
    const result = {};
    for (const conversationId of input.conversationIds ?? []) {
      const records = this.records(input.accountId, conversationId);
      result[conversationId] = {
        latest: records.at(-1)?.message ?? null,
        unread: records.filter(record => !record.read && String(record.message.sender_id) !== String(input.accountId)).length,
      };
    }
    return result;
  }

  markRead(input) {
    return this.mutate(() => {
      for (const record of this.records(input.accountId, input.conversationId)) record.read = true;
      return true;
    });
  }

  delete(input) {
    return this.mutate(() => delete this.data.records[`${cleanID(input.accountId)}:${cleanID(input.conversationId)}:${cleanID(input.messageId)}`]);
  }

  deleteIfSender(input) {
    return this.mutate(() => {
      const key = `${cleanID(input.accountId)}:${cleanID(input.conversationId)}:${cleanID(input.messageId)}`;
      const record = this.data.records[key];
      if (!record || String(record.message.sender_id) !== cleanID(input.senderId)) return false;
      delete this.data.records[key];
      return true;
    });
  }

  clear(input) {
    return this.mutate(() => {
      const prefix = `${cleanID(input.accountId)}:${cleanID(input.conversationId)}:`;
      for (const key of Object.keys(this.data.records)) if (key.startsWith(prefix)) delete this.data.records[key];
      return true;
    });
  }
}

module.exports = { HistoryStore, cleanMessage };
