import {useEffect,useRef,useState} from 'react';
import type {FormEvent,ReactNode} from 'react';
import {allPages,api} from '../api';
import {groupItem,roleNames,useChatInbox} from './chat-inbox';
import type {Group,InboxItem} from './chat-inbox';
import type {Conversation,Person} from './deanery-messages-model';
import {studyDate,studyError} from './study-format';
import {useCachedQuery} from './useCachedQuery';

type Message={messageID:string;senderAccountID:string;senderName?:string;text:string;createdAt:string;readAt?:string|null;read?:boolean};
const initials=(value:string)=>value.trim().split(/\s+/).slice(0,2).map(word=>Array.from(word)[0]??'').join('').toLocaleUpperCase('ru');
const paths={private:'conversations',group:'groups'} as const;
const chatPath=(item:InboxItem)=>`/messages/${paths[item.kind]}/${item.id}`;

type Icons={add:ReactNode;info:ReactNode};

export function ChatPanel({accountID,icons}: {accountID:string;icons:Icons}){
    const inbox=useChatInbox(accountID,0);
    const [selected,setSelected]=useState('');
    const [composer,setComposer]=useState('');
    const [notice,setNotice]=useState('');
    const [sending,setSending]=useState(false);
    const [newChat,setNewChat]=useState(false);
    const [info,setInfo]=useState(false);
    const lock=useRef(false);
    const pending=useRef<{id:string;text:string;messageID:string}|null>(null);
    const acknowledged=useRef('');
    const scroll=useRef<HTMLDivElement>(null);
    const items=inbox.data?.items??[];
    const current=items.find(item=>item.id===selected)??items[0];
    const endpoint=current?chatPath(current):null;
    const thread=useCachedQuery(endpoint?'chat-thread:'+endpoint:null,async()=>{
        if(!endpoint) return [];
        return (await allPages<Message>(endpoint+'/messages')).items;
    });
    const messages=thread.data;
    const updateInbox=inbox.update;
    const latest=messages?.at(-1)?.messageID;
    const hasUnread=messages?.some(message=>message.senderAccountID!==accountID&&
        (current?.kind==='group'?!message.read:!message.readAt));
    const threadReload=thread.reload;

    useEffect(()=>{
        if(!endpoint||!hasUnread||!latest||!current)return;
        const scope=endpoint+'/'+latest;
        const selectedID=current.id;
        const read=()=>{
            if(document.visibilityState!=='visible'||acknowledged.current===scope)return;
            acknowledged.current=scope;
            void api(endpoint+'/read?through='+encodeURIComponent(latest),'POST').then(()=>{
                updateInbox(view=>({...view,warning:view?.warning??'',items:(view?.items??[]).map(item=>item.id===selectedID?{...item,unread:0}:item)}));
                void threadReload();
            }).catch(()=>{if(acknowledged.current===scope)acknowledged.current='';});
        };
        read();
        document.addEventListener('visibilitychange',read);
        return()=>document.removeEventListener('visibilitychange',read);
    },[accountID,current,endpoint,hasUnread,latest,threadReload,updateInbox]);
    useEffect(()=>{if(scroll.current)scroll.current.scrollTop=scroll.current.scrollHeight;},[endpoint,latest]);

    function choose(id:string){
        setSelected(id);setComposer('');setNotice('');setInfo(false);
    }

    async function send(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        const text=composer.trim();
        if(!current||!endpoint||!text||lock.current)return;
        if(Array.from(text).length>10000){setNotice('Сообщение не должно превышать 10 000 символов');return}
        const intended=pending.current;
        const messageID=intended?.id===current.id&&intended.text===text?intended.messageID:crypto.randomUUID();
        pending.current={id:current.id,text,messageID};
        lock.current=true;setSending(true);setNotice('');
        try{
            const result=await api<Message>(endpoint+'/messages','POST',{text,messageID});
            thread.update(list=>[...(list??[]).filter(message=>message.messageID!==result.messageID),result]
                .sort((a,b)=>a.createdAt.localeCompare(b.createdAt)||a.messageID.localeCompare(b.messageID)));
            inbox.update(view=>({warning:view?.warning??'',items:(view?.items??[]).map(item=>item.id===current.id?{...item,when:result.createdAt}:item)
                .sort((a,b)=>b.when.localeCompare(a.when))}));
            pending.current=null;
            setComposer('');
            void thread.reload();
        }catch(error){setNotice(studyError(error))}
        finally{lock.current=false;setSending(false)}
    }

    function added(item:InboxItem){
        inbox.update(view=>({warning:view?.warning??'',items:[item,...(view?.items??[]).filter(existing=>existing.id!==item.id)]}));
        choose(item.id);
        setNewChat(false);
        void inbox.reload();
    }

    return <div className="chat-layout">
        <aside className="card chat-list">
            <div className="card-head"><h3 className="card-title">Сообщения</h3>
                <button className="icon-btn" aria-label="Новый чат" disabled={sending} onClick={()=>setNewChat(true)}>{icons.add}</button></div>
            {items.map(item=><button key={item.id} className={`chat-person ${current?.id===item.id?'active':''}`} disabled={sending} onClick={()=>choose(item.id)}>
                <span className="mini-avatar">{initials(item.title)}</span><span><div className="row-name">{item.title}</div><div className="row-sub">{item.subtitle}</div></span>
                {item.unread>0&&<span className="unread-count" aria-label={`Непрочитанных: ${item.unread}`}>{item.unread>99?'99+':item.unread}</span>}
            </button>)}
            {!items.length&&<p className="caption" role={inbox.error?'alert':'status'}>{inbox.error||(!inbox.ready?'Загружаем чаты…':'Диалогов пока нет. Нажмите «+», чтобы начать.')}</p>}
            {inbox.data?.warning&&<p className="caption" role="status">{inbox.data.warning}</p>}
            {inbox.error&&<button className="secondary" onClick={()=>void inbox.reload()}>Повторить</button>}
        </aside>
        <section className="card conversation">
            <div className="chat-head"><span className="mini-avatar">{current?initials(current.title):'—'}</span>
                <div><div className="row-name">{current?.title??'Выберите чат'}</div><div className="row-sub">{current?.subtitle??'Переписка доступна её участникам'}</div></div>
                <button className="icon-btn" style={{marginLeft:'auto'}} aria-label="Информация о чате" aria-expanded={info} disabled={!current} onClick={()=>setInfo(value=>!value)}>{icons.info}</button>
            </div>
            {info&&current&&<div className="task"><div className="row-name">{current.title}</div><div className="row-sub">{current.subtitle}</div></div>}
            <div className="messages" ref={scroll} style={{maxHeight:540,overflowY:'auto'}} role="log" aria-label="История переписки">
                {messages?.map(message=>{
                    const mine=message.senderAccountID===accountID;
                    return <div key={message.messageID} className={`bubble ${mine?'mine':''}`} style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>
                        {!mine&&message.senderName&&<strong className="row-sub">{message.senderName}</strong>}{!mine&&message.senderName&&<br/>}
                        {message.text}<div className="row-sub" style={{marginTop:4,fontSize:10}}>
                            {studyDate(message.createdAt)}{mine&&current?.kind==='private'&&message.readAt?' · Прочитано':''}
                        </div>
                    </div>;
                })}
                {!messages?.length&&<div className="caption" role={thread.error?'alert':'status'}>{thread.error||(!current?'История появится после выбора чата':!thread.ready?'Загружаем сообщения…':'Сообщений пока нет')}</div>}
            </div>
            <form className="chat-compose" onSubmit={send}>
                <input className="input" value={composer} disabled={!current||sending} onChange={event=>setComposer(event.target.value)} maxLength={10000} placeholder="Написать сообщение…" aria-label="Сообщение"/>
                <button className="primary" type="submit" disabled={!current||sending||!composer.trim()}>{sending?'Отправляем…':'Отправить'}</button>
            </form>
            {notice&&<p className="caption" role="alert">{notice}</p>}
        </section>
        {newChat&&<NewChatDialog close={()=>setNewChat(false)} added={added}/>}
    </div>;
}

function NewChatDialog({close,added}: {close:()=>void;added:(item:InboxItem)=>void}){
    const dialog=useRef<HTMLDialogElement>(null);
    const contacts=useCachedQuery('chat-contacts',()=>allPages<Person>('/messages/contacts'));
    const [mode,setMode]=useState<'private'|'group'>('private');
    const [search,setSearch]=useState('');
    const [title,setTitle]=useState('');
    const [picked,setPicked]=useState<{person:Person; number?:number}[]>([]);
    const [lookup,setLookup]=useState('');
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    const term=search.trim();
    const numeric=/^\d+$/.test(term);
    const number=numeric?Number(term):0;
    const validNumber=number>0&&number<=2147483647;
    useEffect(()=>{
        const timer=window.setTimeout(()=>setLookup(validNumber?String(number):''),250);
        return()=>window.clearTimeout(timer);
    },[number,validNumber]);
    const waiting=validNumber&&lookup!==String(number);
    const found=useCachedQuery(lookup?'chat-contact-number:'+lookup:null,
        ()=>api<{items:Person[]}>(`/messages/contacts?number=${encodeURIComponent(lookup)}`),0,'Не удалось найти контакт',0);
    const shown=numeric?(waiting||!validNumber?[]:found.data?.items??[]):
        (contacts.data?.items??[]).filter(person=>person.displayName.toLocaleLowerCase('ru').includes(term.toLocaleLowerCase('ru')));
    const searchError=numeric?(waiting||!validNumber?'':found.error):contacts.error;
    const searching=numeric?waiting||(validNumber&&!found.ready&&!found.error):!contacts.ready&&!contacts.error;
    const selectedIDs=picked.map(item=>item.person.accountID);
    const maximum=mode==='private'?1:31;
    const confirmed=mode==='private'?picked.length===1:picked.length>=2&&!!title.trim();
    useEffect(()=>{dialog.current?.showModal()},[]);
    async function create(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(busy||!confirmed)return;
        setBusy(true);setError('');
        try{
            if(mode==='group'){
                const contactNumbers=Object.fromEntries(picked.flatMap(item=>item.number!==undefined?[[item.person.accountID,item.number]]:[]));
                const created=await api<Group>('/messages/groups','POST',{title,memberAccountIDs:selectedIDs,contactNumbers});
                added(groupItem(created));
            }else{
                const person=picked[0]?.person;
                if(!person)return;
                const created=await api<Conversation>('/messages/conversations','POST',{participantAccountID:person.accountID});
                added({id:created.conversationID,title:person.displayName,subtitle:roleNames[person.role]??'Личный диалог',
                    kind:'private',unread:0,when:created.updatedAt});
            }
        }catch(err){setError(studyError(err))}
        finally{setBusy(false)}
    }
    return <dialog ref={dialog} className="modal" aria-labelledby="new-chat-title" onClose={close}
        style={{border:'1px solid var(--line)',borderRadius:16,padding:20,width:540,maxWidth:'calc(100vw - 32px)',maxHeight:'calc(100dvh - 32px)',color:'inherit'}}>
        <form onSubmit={create} className="stack" style={{display:'grid',gap:12}}>
            <div className="card-head"><h2 className="card-title" id="new-chat-title">Новый чат</h2>
                <button type="button" className="icon-btn" aria-label="Закрыть" onClick={()=>dialog.current?.close()}>×</button></div>
            <div className="tabs">
                <button type="button" className={`tab ${mode==='private'?'active':''}`} onClick={()=>{setMode('private');setPicked([])}}>Личный</button>
                <button type="button" className={`tab ${mode==='group'?'active':''}`} onClick={()=>{setMode('group');setPicked([])}}>Групповой</button>
            </div>
            {mode==='group'&&<input className="input" aria-label="Название группы" placeholder="Название группы" value={title} onChange={e=>setTitle(e.target.value)} maxLength={80} required/>}
            <input className="input" aria-label="Поиск контактов" placeholder="Имя или уникальный номер" value={search} onChange={e=>setSearch(e.target.value)}/>
            <div className="list" style={{maxHeight:250,overflowY:'auto'}}>
                {shown.map(person=><label className="chat-person" key={person.accountID}>
                    <input type={mode==='private'?'radio':'checkbox'} name={mode==='private'?'recipient':undefined} checked={selectedIDs.includes(person.accountID)} disabled={busy||(!selectedIDs.includes(person.accountID)&&mode==='group'&&picked.length>=maximum)}
                        onChange={event=>{
                            const contact={person,number:numeric?number:undefined};
                            setPicked(items=>mode==='private'?[contact]:event.target.checked?[...items,contact]:
                                items.filter(item=>item.person.accountID!==person.accountID));
                        }}/>
                    <span><span className="row-name">{person.displayName}</span><span className="row-sub" style={{display:'block'}}>{roleNames[person.role]??person.role}</span></span>
                </label>)}
                {!shown.length&&<p className="caption" role={searchError?'alert':'status'}>{searchError||(searching?'Ищем контакты…':'Контакты не найдены')}</p>}
            </div>
            {mode==='group'&&<p className="caption">Выберите от 2 до 31 участника · выбрано {picked.length}</p>}
            {!!picked.length&&<div style={{display:'flex',flexWrap:'wrap',gap:6}}>{picked.map(item=><button type="button" className="secondary" key={item.person.accountID}
                disabled={busy} aria-label={`Убрать: ${item.person.displayName}`} onClick={()=>setPicked(items=>items.filter(selected=>selected.person.accountID!==item.person.accountID))}>
                {item.person.displayName} ×</button>)}</div>}
            {error&&<p className="caption" role="alert">{error}</p>}
            <button className="primary" disabled={busy||!confirmed}>{busy?'Создаём…':mode==='group'?'Создать группу':'Открыть чат'}</button>
        </form>
    </dialog>;
}
