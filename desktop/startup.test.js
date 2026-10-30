const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const {EventEmitter}=require('node:events');
function harness({fail=false,mounted=true}={}){
  const calls={loads:0,files:[],clears:[],cache:0},timers=new Map(),windows=[];
  let id=0;
  class Window extends EventEmitter{
    constructor(){super();windows.push(this);this.url='';this.webContents=new EventEmitter();Object.assign(this.webContents,{setWindowOpenHandler(){},getURL:()=>this.url,executeJavaScript:async()=>mounted});}
    isDestroyed(){return false}
    async loadFile(file,options){this.url=require('node:url').pathToFileURL(file).href;calls.files.push(options.query.failed)}
    async loadURL(url){calls.loads++;this.url=url;if(fail)throw Object.assign(Error('failed'),{code:'ERR_NAME_NOT_RESOLVED'})}
  }
  const electron={BrowserWindow:Window,app:{setAppUserModelId(){},getVersion:()=> '1.2.1',getPath:()=>'/tmp',whenReady:()=>({then(){}}),on(){}},
    ipcMain:{handle(){},on(){}},session:{defaultSession:{setPermissionRequestHandler(){},clearCache:async()=>calls.cache++,clearStorageData:async opts=>calls.clears.push(opts)}},shell:{},nativeImage:{}};
  const ctx=vm.createContext({__dirname:__dirname,require:name=>name==='electron'?electron:name==='fs'?{mkdirSync(){},existsSync(){return false},appendFileSync(){}}:name==='./history-store'?{HistoryStore:class{}}:require(name),setTimeout:(fn,delay)=>{timers.set(++id,{fn,delay});return id},clearTimeout:id=>timers.delete(id),console,process});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'main.js'),'utf8'),ctx);
  ctx.createWindow();
  return {ctx,calls,timers,windows};
}
const flush=()=>new Promise(resolve=>setImmediate(resolve));
test('network failure retries once, preserves user storage, and shows recovery UI',async()=>{
  const {calls,timers}=harness({fail:true});await flush();await flush();
  assert.equal(calls.loads,2);assert.equal(calls.cache,1);
  assert.deepEqual(Array.from(calls.clears[0].storages),['serviceworkers','cachestorage']);
  assert.equal(calls.files.at(-1),'1');assert.equal(timers.size,0);
});
test('successful app mount stops startup timeout without clearing caches',async()=>{
  const {calls,timers,windows}=harness();await flush();windows[0].webContents.emit('did-finish-load');await flush();
  assert.equal(timers.size,0);assert.equal(calls.cache,0);assert.equal(calls.loads,1);
});
test('empty Vue root triggers cache recovery rather than accepting blank page',async()=>{
  const {calls,windows}=harness({mounted:false});await flush();
  windows[0].webContents.emit('did-finish-load');await flush();
  assert.equal(calls.cache,1);assert.equal(calls.loads,2);
  windows[0].webContents.emit('did-finish-load');await flush();
  assert.equal(calls.files.at(-1),'1');
});
