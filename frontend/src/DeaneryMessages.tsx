import {useCachedQuery} from './useCachedQuery';
import {useEffect,useRef,useState} from 'react';
import {loadDeaneryContacts,loadDialog,loadInbox,markDialogRead,sendDialogMessage} from './deanery-messages-model';
import type {ChatMessage,Person} from './deanery-messages-model';
import {requestDate} from './requests-model';
type Contact=Person&{conversationID?:string};
export default function DeaneryMessages({accountID,staff=false,operator=false}:{accountID:string;staff?:boolean;operator?:boolean}){
 const [preferred,setSelected]=useState('');
 const [text,setText]=useState('');
 const [error,setError]=useState('');
 const [busy,setBusy]=useState(false);
 const sending=useRef(false);
 const end=useRef<HTMLDivElement>(null);
 const contactsQuery=useCachedQuery(`deanery-contacts:${accountID}:${staff}:${operator}`,async():Promise<Contact[]>=>{
    const inbox=await loadInbox(accountID);
    if(staff)return inbox.filter(c=>c.person.role==='STUDENT').map(c=>({...c.person,conversationID:c.conversationID}));
    const people=await loadDeaneryContacts(operator);
    return people.map(p=>({...p,conversationID:inbox.find(c=>c.participants.includes(p.accountID))?.conversationID}));
 });
 const contacts=contactsQuery.data??[];
 const selected=contacts.some(p=>p.accountID===preferred)?preferred:contacts[0]?.accountID??'';
 const conversationID=contacts.find(p=>p.accountID===selected)?.conversationID??'';
 const dialogQuery=useCachedQuery(conversationID?'deanery-dialog:'+conversationID:null,()=>loadDialog(conversationID));
 const messages:ChatMessage[]=dialogQuery.data??[];
 const loading=!contactsQuery.ready&&!contactsQuery.error;
 const readError=contactsQuery.error||dialogQuery.error;
 useEffect(()=>{end.current?.scrollIntoView({block:'nearest'})},[messages.length]);
 const newest=messages.at(-1)?.messageID;
 const hasUnread=messages.some(m=>m.senderAccountID!==accountID&&!m.readAt);
 const reloadDialog=dialogQuery.reload;
 useEffect(()=>{
    if(!conversationID||!hasUnread||!newest)return;
    const mark=()=>{if(document.visibilityState==='visible')void markDialogRead(conversationID,newest).then(reloadDialog).catch(()=>{});};
    mark();
    document.addEventListener('visibilitychange',mark);
    return()=>document.removeEventListener('visibilitychange',mark);
 },[conversationID,hasUnread,newest,reloadDialog]);
 const send=async()=>{
    if(sending.current||!selected||!text.trim())return;
    if(Array.from(text).length>10000){setError('Сообщение не должно превышать 10 000 символов');return}
    sending.current=true;setBusy(true);setError('');
    try{
        const id=await sendDialogMessage(selected,conversationID,text);
        setText('');
        contactsQuery.update(list=>(list??[]).map(p=>p.accountID===selected?{...p,conversationID:id}:p));
        if(conversationID)void dialogQuery.reload();
    }catch(err){
        setError((err instanceof Error?err.message:'Не удалось подтвердить отправку.')+' Нажмите «Обновить», чтобы проверить историю перед повтором.');
    }finally{sending.current=false;setBusy(false)}
 };

 return <div><div style={{display:'flex',gap:10,alignItems:'center',marginBottom:14,flexWrap:'wrap'}}><select aria-label={staff?'Диалог со студентом':operator?'Оператор поддержки':'Сотрудник деканата'} value={selected} onChange={e=>setSelected(e.target.value)} disabled={busy||loading||!contacts.length} style={{flex:1,minWidth:160,minHeight:44,border:'1px solid var(--line)',borderRadius:12,padding:'0 44px 0 14px',font:'inherit',backgroundColor:'#fff',appearance:'none',backgroundImage:"url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")",backgroundRepeat:'no-repeat',backgroundPosition:'right 16px center',backgroundSize:16,color:'inherit'}}>{!contacts.length&&<option value="">{loading?'Загрузка…':staff?'Диалогов пока нет':'Нет доступных сотрудников'}</option>}{contacts.map(p=><option value={p.accountID} key={p.accountID}>{p.displayName}</option>)}</select><button type="button" onClick={()=>{void contactsQuery.reload();void dialogQuery.reload()}} disabled={busy} style={{minHeight:42,padding:'0 14px',border:'1px solid var(--line)',borderRadius:12,background:'#fff',font:'inherit',cursor:'pointer'}}>Обновить</button></div>
 {(error||readError)&&<p role="alert" style={{color:'#a24f58',fontSize:12}}>{error||readError}</p>}
 <div aria-live="polite" style={{display:'flex',flexDirection:'column',gap:10,maxHeight:350,minHeight:160,overflowY:'auto',padding:12,borderRadius:16,background:'var(--bg)'}}>{messages.map(m=><div key={m.messageID} style={{alignSelf:m.senderAccountID===accountID?'flex-end':'flex-start',maxWidth:'88%',padding:'11px 13px',borderRadius:15,background:m.senderAccountID===accountID?'var(--violet-soft)':'#fff',whiteSpace:'pre-wrap',overflowWrap:'anywhere',fontSize:13,lineHeight:1.5}}>{m.text}<div style={{fontSize:10,color:'var(--muted)',marginTop:5}}>{requestDate(m.createdAt)}</div></div>)}{!messages.length&&<p style={{fontSize:12,color:'var(--muted)'}}>{loading?'Загружаем переписку…':selected?'Напишите сообщение, чтобы задать вопрос.':'Сейчас нет доступного диалога.'}</p>}<div ref={end}/></div>
 <form onSubmit={e=>{e.preventDefault();void send()}} style={{display:'flex',gap:10,marginTop:14,flexWrap:'wrap'}}><textarea aria-label={staff?"Ответ студенту":operator?"Сообщение оператору":"Сообщение деканату"} value={text} onChange={e=>setText(e.target.value)} disabled={!selected||busy} placeholder={staff?'Написать ответ':operator?'Задайте вопрос поддержке':'Задайте вопрос деканату'} rows={3} maxLength={10000} style={{flex:'1 1 230px',minWidth:0,padding:12,border:'1px solid var(--line)',borderRadius:14,font:'inherit',fontSize:13,color:'inherit',resize:'vertical'}}/><button type="submit" disabled={!selected||busy||!text.trim()} style={{alignSelf:'flex-end',minHeight:44,padding:'0 18px',border:0,borderRadius:14,background:'var(--violet)',color:'#fff',font:'inherit',fontWeight:800,cursor:'pointer'}}>{busy?'Отправляем…':'Отправить'}</button></form></div>
}
