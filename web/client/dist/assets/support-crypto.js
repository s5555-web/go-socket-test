// Ciphertext is the only serializable message representation. Private keys stay on devices.
(function (root) {
  const enc = new TextEncoder(), dec = new TextDecoder();
  const alg = 'SUPPORT-P256-HKDF-A256GCM';
  const b64 = bytes => btoa(String.fromCharCode(...new Uint8Array(bytes)));
  const un64 = text => Uint8Array.from(atob(text), c => c.charCodeAt(0));
  async function key(privateKey, publicKey, context) {
    const pub = await crypto.subtle.importKey('jwk', publicKey, { name: 'ECDH', namedCurve: 'P-256' }, false, []);
    const bits = new Uint8Array(await crypto.subtle.deriveBits({ name: 'ECDH', public: pub }, privateKey, 256));
    const base = await crypto.subtle.importKey('raw', bits, 'HKDF', false, ['deriveKey']); bits.fill(0);
    return crypto.subtle.deriveKey({ name: 'HKDF', hash: 'SHA-256', salt: enc.encode('signal-support-v1'), info: enc.encode(context) }, base, { name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt']);
  }
  async function encrypt(body, threadID, members) {
    if (!members.length || members.some(m => !m.public_key)) throw Error('客服或访客尚未启用加密，请重新登录或刷新插件');
    const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, false, ['deriveBits']);
    const epk = await crypto.subtle.exportKey('jwk', pair.publicKey), recipients = {};
    const text = JSON.stringify({ body, pad: b64(crypto.getRandomValues(new Uint8Array(512 - (enc.encode(body).length % 512)))) });
    for (const member of members) {
      const context = `${threadID}:${member.id}`, iv = crypto.getRandomValues(new Uint8Array(12));
      const aes = await key(pair.privateKey, JSON.parse(member.public_key), context);
      const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv, additionalData: enc.encode(context) }, aes, enc.encode(text));
      recipients[member.id] = { iv: b64(iv), ct: b64(ct) };
    }
    return JSON.stringify({ v: 1, alg, epk: { kty: epk.kty, crv: epk.crv, x: epk.x, y: epk.y }, recipients });
  }
  async function decrypt(value, threadID, recipient, privateKey) {
    try {
      const env = JSON.parse(value), box = env.recipients?.[recipient];
      if (env.v !== 1 || env.alg !== alg || !box) throw Error('missing key');
      const context = `${threadID}:${recipient}`, aes = await key(privateKey, env.epk, context);
      const clear = new Uint8Array(await crypto.subtle.decrypt({ name: 'AES-GCM', iv: un64(box.iv), additionalData: enc.encode(context) }, aes, un64(box.ct)));
      try { return JSON.parse(dec.decode(clear)).body } finally { clear.fill(0) }
    } catch { return '🔒 此消息无法在当前设备解密' }
  }
  root.SignalSupportCrypto = { encrypt, decrypt };
})(globalThis);
