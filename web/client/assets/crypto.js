const E2EE=(()=>{
  const enc=new TextEncoder(),dec=new TextDecoder();
  const PQ=globalThis.SignalPostQuantum;
  const PQ_ALG='P256-MLKEM768-HKDF-SHA256-A256GCM';
  let identity=null,currentAccount='';

  const b64=value=>{let out='';for(const byte of new Uint8Array(value))out+=String.fromCharCode(byte);return btoa(out)};
  const un64=value=>Uint8Array.from(atob(value),char=>char.charCodeAt(0));
  const equalBytes=(first,second)=>first.length===second.length&&first.every((value,index)=>value===second[index]);
  const sameClassicKey=(first,second)=>{
    try{const a=typeof first==='string'?JSON.parse(first):first,b=typeof second==='string'?JSON.parse(second):second;return a?.kty==='EC'&&a?.crv==='P-256'&&a.x===b?.x&&a.y===b?.y}catch{return false}
  };
  const db=()=>new Promise((ok,no)=>{const request=indexedDB.open('signal-web-e2ee',1);request.onupgradeneeded=()=>request.result.createObjectStore('keys');request.onsuccess=()=>ok(request.result);request.onerror=()=>no(request.error)});
  async function get(name){const database=await db();return new Promise((ok,no)=>{const request=database.transaction('keys').objectStore('keys').get(name);request.onsuccess=()=>ok(request.result);request.onerror=()=>no(request.error)})}
  async function put(name,value){const database=await db();return new Promise((ok,no)=>{const request=database.transaction('keys','readwrite').objectStore('keys').put(value,name);request.onsuccess=()=>ok();request.onerror=()=>no(request.error)})}
  async function backupKey(password,salt,iterations=310000){const material=await crypto.subtle.importKey('raw',enc.encode(password),'PBKDF2',false,['deriveKey']);return crypto.subtle.deriveKey({name:'PBKDF2',hash:'SHA-256',salt,iterations},material,{name:'AES-GCM',length:256},false,['encrypt','decrypt'])}
  async function encryptBackup(clear,password,aad){const salt=crypto.getRandomValues(new Uint8Array(16)),iv=crypto.getRandomValues(new Uint8Array(12)),key=await backupKey(password,salt),ct=await crypto.subtle.encrypt({name:'AES-GCM',iv,additionalData:enc.encode(aad)},key,clear);return JSON.stringify({v:1,kdf:'PBKDF2-SHA256',iterations:310000,salt:b64(salt),iv:b64(iv),ct:b64(ct)})}
  async function decryptBackup(value,password,aad){const box=JSON.parse(value);if(box.v!==1||box.kdf!=='PBKDF2-SHA256'||box.iterations!==310000)throw Error('不支持的密钥备份格式');const key=await backupKey(password,un64(box.salt),box.iterations);return crypto.subtle.decrypt({name:'AES-GCM',iv:un64(box.iv),additionalData:enc.encode(aad)},key,un64(box.ct))}
  async function makeClassicBackup(pkcs8,password){return encryptBackup(pkcs8,password,'signal-web-key-backup-v1')}
  async function restoreClassicBackup(value,password,publicJwk){
    const pkcs8=await decryptBackup(value,password,'signal-web-key-backup-v1');
    const extractable=await crypto.subtle.importKey('pkcs8',pkcs8,{name:'ECDH',namedCurve:'P-256'},true,['deriveBits']);
    const privateJwk=await crypto.subtle.exportKey('jwk',extractable);
    if(!sameClassicKey(privateJwk,publicJwk))throw Error('服务器公钥与备份私钥不匹配');
    const privateKey=await crypto.subtle.importKey('pkcs8',pkcs8,{name:'ECDH',namedCurve:'P-256'},false,['deriveBits']);
    const publicKey=await crypto.subtle.importKey('jwk',JSON.parse(publicJwk),{name:'ECDH',namedCurve:'P-256'},true,[]);
    return {privateKey,publicKey};
  }
  async function createClassicIdentity(password){const raw=await crypto.subtle.generateKey({name:'ECDH',namedCurve:'P-256'},true,['deriveBits']),pkcs8=await crypto.subtle.exportKey('pkcs8',raw.privateKey),privateKey=await crypto.subtle.importKey('pkcs8',pkcs8,{name:'ECDH',namedCurve:'P-256'},false,['deriveBits']);return {pair:{privateKey,publicKey:raw.publicKey},backup:await makeClassicBackup(pkcs8,password)}}
  async function makePQBackup(pair,password){const clear=enc.encode(JSON.stringify({public_key:b64(pair.publicKey),secret_key:b64(pair.secretKey)}));return encryptBackup(clear,password,'signal-web-mlkem768-key-backup-v1')}
  async function restorePQBackup(value,password,serverPublicKey){
    if(!PQ)throw Error('当前客户端缺少后量子加密组件，请刷新页面');
    const clear=await decryptBackup(value,password,'signal-web-mlkem768-key-backup-v1'),saved=JSON.parse(dec.decode(clear)),publicKey=un64(saved.public_key),secretKey=un64(saved.secret_key),serverKey=un64(serverPublicKey);
    if(publicKey.length!==PQ.parameters.publicKeyBytes||secretKey.length!==PQ.parameters.secretKeyBytes||!equalBytes(publicKey,serverKey))throw Error('服务器后量子公钥与备份私钥不匹配');
    return {publicKey,secretKey};
  }
  async function init(username,serverPublicKey,serverBackup,serverPQPublicKey='',serverPQBackup='',password=''){
    if(!PQ)throw Error('后量子加密组件加载失败，请清除缓存后重试');
    currentAccount=username;
    const stored=await get(username);
    let classic=null,classicResult={created:false};
    if(stored?.privateKey&&stored?.publicKey&&serverPublicKey){const local=await crypto.subtle.exportKey('jwk',stored.publicKey);if(sameClassicKey(local,serverPublicKey))classic={privateKey:stored.privateKey,publicKey:stored.publicKey}}
    if(!classic&&serverBackup){if(!password)throw Error('此账号的加密私钥不在当前设备，请重新登录安全恢复');try{classic=await restoreClassicBackup(serverBackup,password,serverPublicKey)}catch{throw Error('加密密钥恢复失败，请确认登录密码')}}
    if(!classic){if(!password)throw Error('请重新登录以设置此设备的加密密钥');const made=await createClassicIdentity(password);classic=made.pair;classicResult={created:true,keyBackup:made.backup,replace:!!serverPublicKey}}
    const publicKey=JSON.stringify(await crypto.subtle.exportKey('jwk',classic.publicKey));

    let pq=null,pqResult={created:false,needsPassword:false};
    if(stored?.pq?.publicKey&&stored?.pq?.secretKey&&serverPQPublicKey){
      const localPublic=new Uint8Array(stored.pq.publicKey),localSecret=new Uint8Array(stored.pq.secretKey);
      if(localPublic.length===PQ.parameters.publicKeyBytes&&localSecret.length===PQ.parameters.secretKeyBytes&&equalBytes(localPublic,un64(serverPQPublicKey)))pq={publicKey:localPublic,secretKey:localSecret};
    }
    if(!pq&&serverPQPublicKey){
      if(!serverPQBackup)throw Error('此账号的后量子私钥没有安全备份，请回到原设备完成密钥迁移');
      if(!password)throw Error('此账号的后量子私钥不在当前设备，请重新登录安全恢复');
      try{pq=await restorePQBackup(serverPQBackup,password,serverPQPublicKey)}catch{throw Error('后量子密钥恢复失败，请确认登录密码')}
    }
    if(!pq&&!serverPQPublicKey&&password){const saved=stored?.pq?.publicKey&&stored?.pq?.secretKey?{publicKey:new Uint8Array(stored.pq.publicKey),secretKey:new Uint8Array(stored.pq.secretKey)}:null;pq=saved?.publicKey.length===PQ.parameters.publicKeyBytes&&saved?.secretKey.length===PQ.parameters.secretKeyBytes?saved:PQ.keygen();pqResult={created:true,keyBackup:await makePQBackup(pq,password),needsPassword:false}}
    if(!pq&&!serverPQPublicKey&&!password)pqResult.needsPassword=true;

    identity={...classic,pq};
    await put(username,identity);
    return {publicKey,keyBackup:classicResult.keyBackup,created:classicResult.created,replace:classicResult.replace,pqPublicKey:pq?b64(pq.publicKey):'',pqKeyBackup:pqResult.keyBackup,pqCreated:pqResult.created,pqNeedsPassword:pqResult.needsPassword};
  }
  async function classicSecret(publicJwk){const pub=await crypto.subtle.importKey('jwk',JSON.parse(publicJwk),{name:'ECDH',namedCurve:'P-256'},false,[]);return new Uint8Array(await crypto.subtle.deriveBits({name:'ECDH',public:pub},identity.privateKey,256))}
  async function aesForV1(publicJwk,conversationID){const bits=await classicSecret(publicJwk),base=await crypto.subtle.importKey('raw',bits,'HKDF',false,['deriveKey']);bits.fill(0);return crypto.subtle.deriveKey({name:'HKDF',hash:'SHA-256',salt:enc.encode('signal-web:'+conversationID),info:enc.encode('message-envelope-v1')},base,{name:'AES-GCM',length:256},false,['encrypt','decrypt'])}
  async function aesForV2(publicJwk,pqSecret,conversationID){const ecdh=await classicSecret(publicJwk),combined=new Uint8Array(ecdh.length+pqSecret.length);combined.set(ecdh);combined.set(pqSecret,ecdh.length);const base=await crypto.subtle.importKey('raw',combined,'HKDF',false,['deriveKey']);ecdh.fill(0);combined.fill(0);return crypto.subtle.deriveKey({name:'HKDF',hash:'SHA-256',salt:enc.encode('signal-web:'+conversationID),info:enc.encode('message-envelope-v2-hybrid-mlkem768')},base,{name:'AES-GCM',length:256},false,['encrypt','decrypt'])}
  function paddedContent(body,attachment,reply){const content={body,attachment,reply},size=enc.encode(JSON.stringify(content)).length,target=Math.ceil((size+48)/128)*128+crypto.getRandomValues(new Uint8Array(1))[0]%96,pad=b64(crypto.getRandomValues(new Uint8Array(Math.max(16,target-size))));return enc.encode(JSON.stringify({...content,pad}))}
  async function encrypt(body,conversationID,members,attachment=null,reply=null){
    const missing=members.filter(member=>!member.public_key);if(missing.length)throw Error(`${missing.map(member=>member.display_name).join('、')} 尚未启用端到端加密`);
    const clear=paddedContent(body,attachment,reply),recipients={},useHybrid=!!identity?.pq&&members.every(member=>member.pq_public_key);
    for(const member of members){
      const iv=crypto.getRandomValues(new Uint8Array(12));
      if(useHybrid){
        const pqPublic=un64(member.pq_public_key);if(pqPublic.length!==PQ.parameters.publicKeyBytes)throw Error(`${member.display_name} 的后量子公钥无效`);
        const encapsulated=PQ.encapsulate(pqPublic),key=await aesForV2(member.public_key,encapsulated.sharedSecret,conversationID),aad=enc.encode(`v2:${conversationID}:${member.id}`),ct=await crypto.subtle.encrypt({name:'AES-GCM',iv,additionalData:aad},key,clear);encapsulated.sharedSecret.fill(0);recipients[member.id]={iv:b64(iv),ct:b64(ct),pq_ct:b64(encapsulated.cipherText)};
      }else{
        const key=await aesForV1(member.public_key,conversationID),aad=enc.encode(`v1:${conversationID}:${member.id}`),ct=await crypto.subtle.encrypt({name:'AES-GCM',iv,additionalData:aad},key,clear);recipients[member.id]={iv:b64(iv),ct:b64(ct)};
      }
    }
    return JSON.stringify({v:useHybrid?2:1,alg:useHybrid?PQ_ALG:'P256-HKDF-A256GCM',attachment_id:attachment?.id||undefined,recipients});
  }
  async function decryptPayload(value,conversationID,userID,members,senderID){
    let env;try{env=JSON.parse(value)}catch{return {body:'⚠ 历史未加密消息：'+value,attachment:null,reply:null}}
    if(![1,2].includes(env.v)||!env.recipients)return {body:'⚠ 无法识别的加密消息',attachment:null,reply:null};
    const box=env.recipients[userID],sender=members.find(member=>member.id===senderID);if(!box||!sender?.public_key)return {body:'🔒 无法解密此消息',attachment:null,reply:null};
    try{
      let key,aad;
      if(env.v===2&&env.alg===PQ_ALG){if(!identity?.pq?.secretKey||!box.pq_ct)throw Error('missing post-quantum key');const pqSecret=PQ.decapsulate(un64(box.pq_ct),new Uint8Array(identity.pq.secretKey));key=await aesForV2(sender.public_key,pqSecret,conversationID);pqSecret.fill(0);aad=enc.encode(`v2:${conversationID}:${userID}`)}
      else if(env.v===1&&env.alg==='P256-HKDF-A256GCM'){key=await aesForV1(sender.public_key,conversationID);aad=enc.encode(`v1:${conversationID}:${userID}`)}
      else throw Error('unsupported envelope');
      const clear=await crypto.subtle.decrypt({name:'AES-GCM',iv:un64(box.iv),additionalData:aad},key,un64(box.ct)),payload=JSON.parse(dec.decode(clear));return {body:payload.body||'',attachment:payload.attachment||null,reply:payload.reply||null};
    }catch{return {body:'🔒 解密失败（密钥可能已变更）',attachment:null,reply:null}}
  }
  async function memberKeyHash(member){const classic=member.public_key?JSON.stringify(JSON.parse(member.public_key)):'';const canonical=`${member.id}|${classic}|${member.pq_public_key||''}`;return b64(await crypto.subtle.digest('SHA-256',enc.encode(canonical)))}
  async function changedMemberKeys(members){const changed=[];for(const member of members){const key=`trust:${currentAccount}:${member.id}`,hash=await memberKeyHash(member),saved=await get(key);if(saved&&saved!==hash)changed.push(member);else if(!saved)await put(key,hash)}return changed}
  async function trustMemberKeys(members){for(const member of members)await put(`trust:${currentAccount}:${member.id}`,await memberKeyHash(member))}
  async function encryptAttachment(file){const key=await crypto.subtle.generateKey({name:'AES-GCM',length:256},true,['encrypt','decrypt']),raw=await crypto.subtle.exportKey('raw',key),iv=crypto.getRandomValues(new Uint8Array(12)),cipher=await crypto.subtle.encrypt({name:'AES-GCM',iv,additionalData:enc.encode('signal-web-image-v1')},key,await file.arrayBuffer());return {cipher,metadata:{v:1,alg:'A256GCM',key:b64(raw),iv:b64(iv),mime:file.type,name:file.name.slice(0,120),size:file.size}}}
  async function decryptAttachment(cipher,metadata){if(metadata?.v!==1||metadata.alg!=='A256GCM')throw Error('不支持的图片加密格式');const key=await crypto.subtle.importKey('raw',un64(metadata.key),'AES-GCM',false,['decrypt']);return crypto.subtle.decrypt({name:'AES-GCM',iv:un64(metadata.iv),additionalData:enc.encode('signal-web-image-v1')},key,cipher)}
  async function decryptEnvelope(...args){return (await decryptPayload(...args)).body}
  async function fingerprint(members){const canonical=await Promise.all(members.map(member=>memberKeyHash(member)));const hash=await crypto.subtle.digest('SHA-256',enc.encode(canonical.sort().join('|')));return [...new Uint8Array(hash)].slice(0,12).map(value=>value.toString(16).padStart(2,'0')).join(' ').toUpperCase()}
  function isPostQuantumReady(members){return !!identity?.pq&&members.length>0&&members.every(member=>member.public_key&&member.pq_public_key)}
  return {init,encrypt,decrypt:decryptEnvelope,decryptPayload,encryptAttachment,decryptAttachment,fingerprint,changedMemberKeys,trustMemberKeys,isPostQuantumReady};
})();
