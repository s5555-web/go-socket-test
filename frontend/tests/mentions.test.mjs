import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {webcrypto} from 'node:crypto'
import vm from 'node:vm'
import {mentionAt,insertMention,sentMentions} from '../client/src/mentions.js'

test('mention picker finds typed query at caret, ignores email and preserves following text',()=>{
  assert.equal(mentionAt('user@example.com',16),null)
  const text='你好 @小 后面的内容',range=mentionAt(text,5)
  assert.deepEqual(range,{start:3,end:5,query:'小'})
  const inserted=insertMention(text,range,{display_name:'小明'})
  assert.equal(inserted.text,'你好 @小明  后面的内容')
  assert.equal(inserted.cursor,7)
  assert.equal(mentionAt(inserted.text,7),null)
})

test('removed mentions are excluded from message metadata',()=>{
  const selected=[{id:2,name:'小明'},{id:3,name:'小红'}]
  assert.deepEqual(sentMentions('你好 @小红',selected),[{id:3,name:'小红'}])
})

test('encrypted message roundtrip retains mention identity without exposing it in envelope',async()=>{
  const source=readFileSync(new URL('../client/src/crypto/e2ee.js',import.meta.url),'utf8')
  const pair=await webcrypto.subtle.generateKey({name:'ECDH',namedCurve:'P-256'},true,['deriveBits'])
  const public_key=JSON.stringify(await webcrypto.subtle.exportKey('jwk',pair.publicKey))
  const context=vm.createContext({crypto:webcrypto,enc:new TextEncoder(),dec:new TextDecoder(),identity:pair,
    b64:bytes=>Buffer.from(bytes).toString('base64'),un64:text=>new Uint8Array(Buffer.from(text,'base64'))})
  vm.runInContext(source.slice(source.indexOf('  async function classicSecret('),source.indexOf('  async function memberKeyHash(')),context)
  const members=[{id:2,display_name:'小明',public_key}]
  const ciphertext=await context.encrypt('你好 @小明',7,members,null,null,[{id:2,name:'小明'}])
  assert.equal(ciphertext.includes('小明'),false)
  const payload=await context.decryptPayload(ciphertext,7,2,members,2)
  assert.equal(payload.body,'你好 @小明')
  assert.equal(payload.mentions[0].id,2)
  assert.equal(payload.mentions[0].name,'小明')
  const old=await context.encrypt('旧格式兼容',7,members)
  assert.equal((await context.decryptPayload(old,7,2,members,2)).mentions.length,0)
})

test('Enter selects a suggested group member before sending, preserving caret suffix',async()=>{
  const {ref}=await import('vue')
  const source=readFileSync(new URL('../client/src/App.vue',import.meta.url),'utf8')
  const composer=ref('hi @小 后文'),range=mentionAt(composer.value,5)
  const context=vm.createContext({composer,mentionRange:ref(range),mentionIndex:ref(0),selectedMentions:ref([]),
    mentionOptions:ref([{id:9,display_name:'小明'}]),composerInput:ref({focus(){},setSelectionRange(){}}),
    insertMention,nextTick:fn=>Promise.resolve().then(fn),sendMessage:()=>assert.fail('selection must not send')})
  vm.runInContext(source.slice(source.indexOf('function updateMention()'),source.indexOf('function resizeComposer()')),context)
  let prevented=false
  context.onComposerKeydown({key:'Enter',preventDefault(){prevented=true}})
  assert.equal(prevented,true)
  assert.equal(composer.value,'hi @小明  后文')
  assert.equal(context.selectedMentions.value[0].id,9)
  assert.equal(context.mentionRange.value,null)
})
