import {useRef,useState} from 'react';
import type {FormEvent,ReactNode} from 'react';
import {allPages,api,response} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {studyError} from './study-format';
import {selectArrowCSS} from './select-arrow';

export function StaffStyles(){
    return <style>{selectArrowCSS+`
    .role-app select{
       appearance:none;-webkit-appearance:none;padding-right:44px!important;
       background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E")!important;
       background-position:right 16px center!important;background-size:16px!important;background-repeat:no-repeat!important
    }
    @media(forced-colors:active){.role-app select{appearance:auto;background-image:none!important;padding-right:12px!important}}
    .staff-panel{display:grid;gap:14px}
    .staff-form{display:grid;gap:10px;max-width:680px}
    .staff-form fieldset{display:grid;gap:10px;border:0;margin:0;padding:0;min-width:0}
    .staff-panel label{display:grid;gap:4px;min-width:0}
    .staff-panel .staff-input{width:100%;min-width:0;padding:10px 12px;border:1px solid var(--line);border-radius:12px;background-color:white;color:inherit;font:inherit}
    .staff-panel select.spaced-select.input{padding-right:44px}
    .staff-panel .staff-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
    .staff-panel .staff-item{padding:12px;border:1px solid var(--line);border-radius:14px;overflow-wrap:anywhere}
    .staff-panel .staff-copy{white-space:pre-wrap;overflow-wrap:anywhere}
    .staff-panel .staff-file{max-width:100%;font:inherit}
    `}</style>;
}

export function StaffForm({children,label,save,reset=true}:{children:ReactNode;label:string;save:(data:FormData)=>Promise<void>;reset?:boolean}){
    const lock=useRef(false);
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    const [saved,setSaved]=useState(false);
    async function submit(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(lock.current)return;
        const form=event.currentTarget;
        const data=new FormData(form);
        lock.current=true;setBusy(true);setError('');setSaved(false);
        try{await save(data);if(reset)form.reset();setSaved(true)}
        catch(error){setError(studyError(error))}
        finally{lock.current=false;setBusy(false)}
    }
    return <form className="staff-form" onSubmit={submit}>
        <fieldset disabled={busy}>{children}
            <button type="submit" className="role-primary">{busy?'Сохраняем…':label}</button>
        </fieldset>
        {error&&<p role="alert" className="role-sub">{error}</p>}
        {saved&&<p role="status" className="role-sub">Сохранено</p>}
    </form>;
}

export function StaffAction({label,run}:{label:string;run:()=>Promise<void>}){
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    async function act(){
        setBusy(true);setError('');
        try{await run()}catch(error){setError(studyError(error))}finally{setBusy(false)}
    }
    return <div><button type="button" className="role-primary" disabled={busy} onClick={()=>void act()}>{busy?'Выполняем…':label}</button>
        {error&&<p role="alert" className="role-sub">{error}</p>}</div>;
}

export function StaffState({data,error,empty='Записей пока нет.'}:{data:unknown;error:string;empty?:string}){
    if(error)return <p role="alert" className="role-sub">{error}</p>;
    if(data==null)return <p role="status" className="role-sub">Загружаем данные…</p>;
    if(Array.isArray(data)&&data.length===0)return <p className="role-sub">{empty}</p>;
    return null;
}

export function StaffRow({title,description,status}:{title:string;description?:ReactNode;status?:ReactNode}){
    return <div className="role-row" style={{cursor:'default',overflowWrap:'anywhere'}}>
        <span><span className="role-name">{title}</span>{description&&<div className="role-sub">{description}</div>}</span>
        {status!=null&&<span className="role-status">{status}</span>}
    </div>;
}

type StaffNotice={notificationID:number;notificationTitle:string;notificationMessage:string;notificationReadAt:string|null};
type StaffUser={displayName:string;email:string;role:string};

export function StaffAccountPanel({notifications}:{notifications:boolean}){
    const data=useCachedQuery<StaffNotice[]|StaffUser>(notifications?'staff-notifications':'staff-profile',()=>{
        if(notifications)return allPages<StaffNotice>('/notifications').then(r=>r.items);
        return api<StaffUser>('/auth/me');
    });
    return <div className="staff-panel">
        <StaffState data={data.data} error={data.error} empty="Уведомлений пока нет."/>
        {Array.isArray(data.data)?data.data.map(item=><div key={item.notificationID} className="staff-item">
            <strong>{item.notificationTitle||'Уведомление'}</strong>
            <p className="staff-copy">{item.notificationMessage}</p>
            {!item.notificationReadAt&&<StaffAction label="Отметить прочитанным" run={async()=>{
                await api('/notifications/'+item.notificationID+'/read','PATCH');await data.reload();
            }}/>}
        </div>):data.data&&<StaffRow title={data.data.displayName??'Сотрудник'} description={data.data.email}/>}
    </div>;
}

export function PrivateFile({path,label='Скачать файл',staff=false}:{path:string;label?:string;staff?:boolean}){
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    async function fetchFile(){
        setBusy(true);setError('');
        try{
            const result=await response(path);
            const extension=result.headers.get('Content-Disposition')?.match(/\.(pdf|docx|png|jpg)"/)?.[1]??'pdf';
            const url=URL.createObjectURL(await result.blob());
            const link=document.createElement('a');
            link.href=url;link.download='Документ.'+extension;link.click();
            window.setTimeout(()=>URL.revokeObjectURL(url),1000);
        }catch(error){setError(studyError(error))}
        finally{setBusy(false)}
    }
    return <div><button className={staff?'role-primary':'secondary'} type="button" disabled={busy} onClick={()=>void fetchFile()}>{busy?'Скачиваем…':label}</button>
        {error&&<p role="alert" className="role-sub caption">{error}</p>}</div>;
}
