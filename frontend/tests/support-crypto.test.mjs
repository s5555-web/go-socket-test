import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { webcrypto } from 'node:crypto'
const context = vm.createContext({ crypto: webcrypto, TextEncoder, TextDecoder, Uint8Array, btoa, atob })
vm.runInContext(readFileSync(new URL('../client/public/assets/support-crypto.js', import.meta.url),'utf8'), context)
const support = context.SignalSupportCrypto

test('visitor and multiple agents decrypt only their own boxes; wrong context and tampering fail closed', async () => {
  const keys = await Promise.all([1,2,3].map(() => webcrypto.subtle.generateKey({ name:'ECDH', namedCurve:'P-256' }, false, ['deriveBits'])))
  const members = await Promise.all(keys.map(async (pair,index) => ({ id: index ? `agent:${index}` : 'visitor:abc', public_key: JSON.stringify(await webcrypto.subtle.exportKey('jwk',pair.publicKey)) })))
  const secret = '测试隐私消息 ✅'
  const body = await support.encrypt(secret, 77, members)
  assert.equal(body.includes(secret),false)
  for (let index=0;index<keys.length;index++) assert.equal(await support.decrypt(body,77,members[index].id,keys[index].privateKey),secret)
  assert.match(await support.decrypt(body,78,members[0].id,keys[0].privateKey),/无法/)
  assert.match(await support.decrypt(body,77,members[0].id,keys[1].privateKey),/无法/)
  const env=JSON.parse(body);env.recipients[members[0].id].ct='AAAA'+env.recipients[members[0].id].ct.slice(4)
  assert.match(await support.decrypt(JSON.stringify(env),77,members[0].id,keys[0].privateKey),/无法/)
  assert.match(await support.decrypt(secret,77,members[0].id,keys[0].privateKey),/无法/)
  assert.equal(keys[0].privateKey.extractable,false)
})
