const {app,BrowserWindow,ipcMain,nativeImage,session,shell}=require('electron');
const path=require('path');
const APP_URL='https://103.96.149.248:802/';
let mainWindow;

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
  mainWindow.loadURL(APP_URL);
}

ipcMain.on('signal-badge',(_event,{count,dataUrl})=>{
  if(!mainWindow||mainWindow.isDestroyed())return;
  if(!count||!dataUrl){mainWindow.setOverlayIcon(null,'无未读消息');return}
  const icon=nativeImage.createFromDataURL(dataUrl).resize({width:32,height:32});
  mainWindow.setOverlayIcon(icon,`${count} 条未读消息`);
});

app.whenReady().then(createWindow);
app.on('window-all-closed',()=>{if(process.platform!=='darwin')app.quit()});
app.on('activate',()=>{if(BrowserWindow.getAllWindows().length===0)createWindow()});
