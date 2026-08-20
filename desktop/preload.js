const{contextBridge,ipcRenderer}=require('electron');
contextBridge.exposeInMainWorld('signalDesktop',{setBadge:(count,dataUrl)=>ipcRenderer.send('signal-badge',{count,dataUrl})});
