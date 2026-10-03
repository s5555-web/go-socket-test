(function(){
  'use strict';
  const script=document.currentScript;
  if(!script||script.dataset.signalSupportLoaded)return;
  script.dataset.signalSupportLoaded='1';
  const siteKey=(script.dataset.siteKey||'').trim();
  if(!siteKey){console.error('Signal Support: data-site-key is required');return}
  const base=new URL(script.src,location.href).origin;
  const title=script.dataset.title||'在线客服';
  const position=script.dataset.position==='left'?'left':'right';
  const storageKey=`signal-support:${siteKey}`;
  let visitorToken=localStorage.getItem(storageKey)||'',lastID=0,pollTimer=null,syncing=false,sending=false,sessionPromise=null,closed=false;

  const host=document.createElement('div');host.style.cssText='position:fixed;z-index:2147483000;bottom:20px;'+position+':20px';
  const root=host.attachShadow({mode:'open'});
  root.innerHTML=`<style>
    *{box-sizing:border-box}.launcher{width:58px;height:58px;border:0;border-radius:50%;background:#2c6bed;color:#fff;box-shadow:0 8px 26px #173b8260;cursor:pointer;font:26px system-ui}.panel{position:absolute;bottom:72px;${position}:0;width:min(370px,calc(100vw - 24px));height:min(570px,calc(100vh - 110px));background:#fff;border-radius:18px;box-shadow:0 18px 60px #0003;overflow:hidden;display:grid;grid-template-rows:64px minmax(0,1fr) auto;font-family:system-ui,-apple-system,sans-serif;color:#20232a}.panel[hidden]{display:none}.head{background:#2c6bed;color:#fff;padding:12px 16px;display:flex;align-items:center;justify-content:space-between}.head b{display:block}.head small{opacity:.84}.close{border:0;background:transparent;color:#fff;font-size:26px;cursor:pointer}.messages{padding:14px;overflow:auto;background:#f5f7fb;display:flex;flex-direction:column;gap:9px}.bubble{align-self:flex-start;max-width:84%;background:#fff;border-radius:14px 14px 14px 4px;padding:9px 12px;box-shadow:0 2px 8px #0000000d;white-space:pre-wrap;word-break:break-word;font-size:14px}.bubble.mine{align-self:flex-end;background:#2c6bed;color:#fff;border-radius:14px 14px 4px 14px}.bubble.system{align-self:center;background:#e9edf5;color:#5b6473;border-radius:12px;text-align:center;font-size:13px}.meta{font-size:10px;opacity:.68;margin-bottom:3px}.composer{padding:10px;background:#fff;border-top:1px solid #e8eaf0;display:flex;gap:8px}.composer textarea{flex:1;resize:none;min-width:0;height:42px;max-height:96px;border:1px solid #dce0e8;border-radius:12px;padding:10px;font:14px system-ui;outline:none}.composer button,.restart{border:0;border-radius:10px;background:#2c6bed;color:#fff;padding:0 15px;cursor:pointer}.notice{text-align:center;color:#7a8190;font-size:12px}.restart{padding:9px 14px;margin-top:8px}@media(max-width:480px){.panel{position:fixed;inset:12px;width:auto;height:auto}.launcher{width:54px;height:54px}}
  </style><button class="launcher" aria-label="打开在线客服">💬</button><section class="panel" hidden><div class="head"><div><b></b><small>等待连接</small></div><button class="close" aria-label="关闭">×</button></div><div class="messages"><div class="notice connecting">正在连接客服…</div></div><form class="composer"><textarea maxlength="2000" placeholder="请输入消息" aria-label="消息"></textarea><button>发送</button></form></section>`;
  document.body.appendChild(host);
  const panel=root.querySelector('.panel'),messages=root.querySelector('.messages'),input=root.querySelector('textarea'),form=root.querySelector('.composer'),status=root.querySelector('.head small');
  root.querySelector('.head b').textContent=title;

  function setStatus(text){status.textContent=text}

  async function request(path,opt={}){
    opt.headers={...(opt.headers||{}),'Content-Type':'application/json'};
    if(visitorToken)opt.headers.Authorization='Bearer '+visitorToken;
    const response=await fetch(base+'/api/widget'+path,opt),data=await response.json().catch(()=>({}));
    if(!response.ok)throw Object.assign(Error(data.error||'客服连接失败'),{status:response.status});
    return data;
  }
  function append(message){
    if(root.querySelector(`[data-message-id="${message.id}"]`))return;
    root.querySelector('.connecting')?.remove();
    const bubble=document.createElement('div');bubble.className='bubble '+(message.sender_type==='visitor'?'mine':message.sender_type==='system'?'system':'');bubble.dataset.messageId=message.id;
    if(message.sender_type!=='system'){const meta=document.createElement('div');meta.className='meta';meta.textContent=message.sender_type==='visitor'?'我':(message.sender_name||'客服');bubble.appendChild(meta)}
    const body=document.createElement('div');body.textContent=message.body;bubble.appendChild(body);messages.appendChild(bubble);lastID=Math.max(lastID,Number(message.id)||0);messages.scrollTop=messages.scrollHeight;
  }
  async function createSession(){
    const session=await request('/sessions',{method:'POST',body:JSON.stringify({site_key:siteKey})});visitorToken=session.visitor_token;localStorage.setItem(storageKey,visitorToken);closed=false;lastID=0;messages.innerHTML='';
  }
  async function ensureSession(){
    if(visitorToken)return;
    if(!sessionPromise)sessionPromise=createSession().finally(()=>{sessionPromise=null});
    await sessionPromise;
  }
  async function sync(){
    if(syncing||panel.hidden)return;syncing=true;
    try{
      setStatus('正在连接…');
      await ensureSession();
      const result=await request('/messages?after='+lastID);result.messages.forEach(append);closed=result.thread.status==='closed';
      root.querySelector('.connecting')?.remove();setStatus('已连接客服');
      if(closed){form.hidden=true;if(!root.querySelector('.restart')){const box=document.createElement('div');box.className='notice restart-box';box.textContent='本次咨询已结束';const button=document.createElement('button');button.className='restart';button.textContent='发起新咨询';button.onclick=async()=>{localStorage.removeItem(storageKey);visitorToken='';lastID=0;box.remove();form.hidden=false;await sync()};box.appendChild(document.createElement('br'));box.appendChild(button);messages.appendChild(box)}}
      else form.hidden=false;
    }catch(error){
      setStatus('连接失败，正在重试');
      if(error.status===401){localStorage.removeItem(storageKey);visitorToken='';lastID=0;messages.innerHTML='';setTimeout(sync,100);return}
      if(!messages.children.length||messages.querySelector('.notice'))messages.innerHTML=`<div class="notice">${error.message}</div>`;
    }finally{syncing=false}
  }
  function startPolling(){clearInterval(pollTimer);sync();pollTimer=setInterval(sync,2500)}
  root.querySelector('.launcher').onclick=()=>{panel.hidden=!panel.hidden;if(!panel.hidden){startPolling();setTimeout(()=>input.focus(),50)}else clearInterval(pollTimer)};
  root.querySelector('.close').onclick=()=>{panel.hidden=true;clearInterval(pollTimer)};
  form.onsubmit=async event=>{event.preventDefault();const body=input.value.trim();if(!body||sending||closed)return;sending=true;try{setStatus('正在发送…');await ensureSession();const sent=await request('/messages',{method:'POST',body:JSON.stringify({body})});append(sent);input.value='';setStatus('已连接客服')}catch(error){setStatus('发送失败');const notice=document.createElement('div');notice.className='notice';notice.textContent=error.message;messages.appendChild(notice)}finally{sending=false;input.focus()}};
  input.onkeydown=event=>{if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();form.requestSubmit()}};
  request('/config?site_key='+encodeURIComponent(siteKey)).catch(error=>{console.error('Signal Support:',error.message);host.remove()});
})();
