const params=new URLSearchParams(location.search);
if(params.get('failed')==='1'){
  document.getElementById('status').textContent='暂时无法打开聊天服务';
  document.getElementById('detail').textContent='请检查网络后重新连接。如仍有问题，可提供诊断日志以定位原因。';
  document.getElementById('actions').hidden=false;
}
document.getElementById('retry').onclick=()=>window.signalDesktop.retryLoad();
document.getElementById('logs').onclick=()=>window.signalDesktop.openDiagnostics();
