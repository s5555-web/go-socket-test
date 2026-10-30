import {test} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'

test('browser mirrors and desktop bridge receive ciphertext only; plaintext is rejected before any write', async()=>{
  const data=new Map(),native=[]
  const storage={getItem:key=>data.get(key)||null,setItem:(key,value)=>data.set(key,value),removeItem:key=>data.delete(key)}
  const context=vm.createContext({window:{signalDesktop:{history:{put:async value=>native.push(value)}}},localStorage:storage,navigator:{},indexedDB:{open(){const request={error:Error('unavailable')};queueMicrotask(()=>request.onerror());return request}},queueMicrotask})
  vm.runInContext(readFileSync(new URL('../client/src/history.js',import.meta.url),'utf8').replace('export function','function'),context)
  const history=context.createLocalHistory(()=>({id:1}))
  const message={id:1,conversation_id:7,sender_id:2,sender_name:'sender',created_at:new Date().toISOString(),body:JSON.stringify({v:1,alg:'P256-HKDF-A256GCM',recipients:{1:{iv:'cipher',ct:'cipher'}}}),payload:{body:'SECRET'},displayBody:'SECRET'}
  await history.put(message)
  assert.equal([...data.values()].join('').includes('SECRET'),false)
  assert.equal(JSON.stringify(native).includes('SECRET'),false)
  await assert.rejects(history.put({...message,body:'SECRET'}))
  assert.equal(native.length,1)
})
