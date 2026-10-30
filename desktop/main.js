const {app,BrowserWindow,ipcMain,nativeImage,session,shell}=require('electron');
const path=require('path');
const fs=require('fs');
const {pathToFileURL}=require('url');
const{HistoryStore}=require('./history-store');
const APP_URL='https://msg.trip-vn.com/';
let mainWindow,historyStore;
const STARTUP_FILE=path.join(__dirname,'startup.html');
let loadGeneration=0,recoveryAttempts=0,startupTimer;
function logStartup(event,details=''){
  try{const dir=app.getPath('logs');fs.mkdirSync(dir,{recursive:true});const file=path.join(dir,'startup.log');if(fs.existsSync(file)&&fs.statSync(file).size>1024*1024)fs.renameSync(file,file+'.previous');fs.appendFileSync(file,`${new Date().toISOString()} ${event} ${details}\n`)}catch{}
}
function isStartupFrame(event){return event.senderFrame?.url.split('?')[0]===pathToFileURL(STARTUP_FILE).href}
async function showStartup(failed=false){
  clearTimeout(startupTimer);
  if(!mainWindow||mainWindow.isDestroyed())return;
  await mainWindow.loadFile(STARTUP_FILE,{query:{failed:failed?'1':'0'}});
}
async function recoverStartup(generation,reason){
  if(generation!==loadGeneration||!mainWindow||mainWindow.isDestroyed())return;
  clearTimeout(startupTimer);
  ++loadGeneration;
  logStartup('load-failed',reason);
  if(recoveryAttempts++<1){
    try{
      // Only resource caches; preserve cookies, IndexedDB, localStorage and native history.
      await session.defaultSession.clearCache();
      await session.defaultSession.clearStorageData({origin:APP_URL,storages:['serviceworkers','cachestorage']});
    }catch(error){logStartup('cache-recovery-failed',error.code||error.name)}
    await connectWindow();
  }else await showStartup(true);
}
async function connectWindow(){
  const generation=++loadGeneration;
  await showStartup();
  if(generation!==loadGeneration)return;
  logStartup('load-start',app.getVersion());
  startupTimer=setTimeout(()=>recoverStartup(generation,'startup-timeout').catch(()=>{}),25000);
  try{await mainWindow.loadURL(APP_URL)}catch(error){await recoverStartup(generation,error.code||'navigation-failed')}
}

app.setAppUserModelId('SignalWeb.Desktop');

function createWindow(){
  mainWindow=new BrowserWindow({
    width:1280,height:820,minWidth:860,minHeight:600,
    title:'Signal Web',backgroundColor:'#f5f6fa',autoHideMenuBar:true,
    webPreferences:{preload:path.join(__dirname,'preload.js'),contextIsolation:true,nodeIntegration:false,sandbox:true}
  });
  session.defaultSession.setPermissionRequestHandler((webContents,permission,callback,details)=>{
    callback(permission==='notifications'&&details.requestingUrl.startsWith(APP_URL));
  });
  mainWindow.webContents.setWindowOpenHandler(({url})=>{if(url.startsWith(APP_URL))return{action:'allow'};shell.openExternal(url);return{action:'deny'}});
  mainWindow.webContents.on('will-navigate',(event,url)=>{if(!url.startsWith(APP_URL)){event.preventDefault();shell.openExternal(url)}});
  mainWindow.webContents.on('did-finish-load',async()=>{
    if(!mainWindow.webContents.getURL().startsWith(APP_URL))return;
    const generation=loadGeneration;
    try{
      const mounted=await mainWindow.webContents.executeJavaScript("Boolean(document.querySelector('#app')?.children.length)");
      if(!mounted)return recoverStartup(generation,'empty-app-root');
      if(generation!==loadGeneration)return;
      clearTimeout(startupTimer);logStartup('app-ready');
    }catch{await recoverStartup(generation,'renderer-check-failed')}
  });
  mainWindow.webContents.on('render-process-gone',(_event,details)=>{recoverStartup(loadGeneration,`renderer-${details.reason}`).catch(()=>{})});
  mainWindow.webContents.on('preload-error',()=>logStartup('preload-error'));
  mainWindow.on('closed',()=>{++loadGeneration;clearTimeout(startupTimer)});
  recoveryAttempts=0;
  connectWindow().catch(()=>showStartup(true));
}

ipcMain.handle('startup:retry',event=>{if(!isStartupFrame(event))throw Error('untrusted startup request');recoveryAttempts=0;return connectWindow()});
ipcMain.handle('startup:logs',event=>{if(!isStartupFrame(event))throw Error('untrusted startup request');return shell.openPath(app.getPath('logs'))});

ipcMain.on('signal-badge',(_event,{count,dataUrl})=>{
  if(!mainWindow||mainWindow.isDestroyed())return;
  if(!count||!dataUrl){mainWindow.setOverlayIcon(null,'无未读消息');return}
  const icon=nativeImage.createFromDataURL(dataUrl).resize({width:32,height:32});
  mainWindow.setOverlayIcon(icon,`${count} 条未读消息`);
});

app.whenReady().then(()=>{
  historyStore=new HistoryStore(path.join(app.getPath('userData'),'encrypted-message-history.json'));
  const handlers={
    'history:put':input=>historyStore.put(input),
    'history:page':input=>historyStore.page(input),
    'history:summaries':input=>historyStore.summaries(input),
    'history:mark-read':input=>historyStore.markRead(input),
    'history:delete':input=>historyStore.delete(input),
    'history:delete-if-sender':input=>historyStore.deleteIfSender(input),
    'history:clear':input=>historyStore.clear(input)
  };
  for(const[channel,handler]of Object.entries(handlers))ipcMain.handle(channel,(event,input)=>{
    if(!event.senderFrame.url.startsWith(APP_URL))throw new Error('untrusted history request');
    return handler(input||{});
  });
  createWindow();
});
app.on('window-all-closed',()=>{if(process.platform!=='darwin')app.quit()});
app.on('activate',()=>{if(BrowserWindow.getAllWindows().length===0)createWindow()});
