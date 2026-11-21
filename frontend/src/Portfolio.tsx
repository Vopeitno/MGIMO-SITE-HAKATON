import {useEffect,useRef,useState} from 'react';
import type {FormEvent,ReactNode} from 'react';
import {allPages,api,download} from '../api';
import './resume.css';
import {selectArrowCSS} from './select-arrow';
import {studyError} from './study-format';
import type {ApiNotification,ApiProfile,TodaySummary} from './today-model';
import {useCachedQuery} from './useCachedQuery';

const labels={project:'Проект',certificate:'Сертификат',achievement:'Достижение',publication:'Публикация',
    volunteering:'Волонтёрство',skill:'Навык'};
type Kind=keyof typeof labels;
const resumeSections:Record<Kind,string>={project:'Проекты',certificate:'Сертификаты',achievement:'Достижения',
    publication:'Публикации',volunteering:'Волонтёрство',skill:'Навыки'};
export type PortfolioItem={id:string;kind:Kind;title:string;description:string;url:string|null};
export type PortfolioView={displayName:string;bio:string;items:PortfolioItem[]};
type PrivatePortfolio=PortfolioView&{shareID:string|null};
type Panel={type:'editor';item?:PortfolioItem}|{type:'bio'}|{type:'sharing'};

const field=(data:FormData,key:string)=>String(data.get(key)??'').trim();
const preview=(value:string)=>value.length>140?value.slice(0,137).trimEnd()+'…':value;
const shareURL=(id:string)=>window.location.origin+window.location.pathname+'#portfolio/'+id;

function EditorForm({children,label,onSave}:{children:ReactNode;label:string;onSave:(data:FormData)=>Promise<void>}){
    const lock=useRef(false);
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    async function submit(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(lock.current)return;
        const data=new FormData(event.currentTarget);
        lock.current=true;setBusy(true);setError('');
        try{await onSave(data)}
        catch(err){setError(studyError(err))}
        finally{lock.current=false;setBusy(false)}
    }
    return <form onSubmit={submit}>
        <fieldset disabled={busy} style={{border:0,margin:0,padding:0,minWidth:0}}>
            {children}
            <button type="submit" className="primary">{busy?'Сохраняем…':label}</button>
        </fieldset>
        {error&&<p role="alert" className="caption">{error}</p>}
    </form>;
}

function SharePortfolio({shareID,onChange}:{shareID:string|null;onChange:(id:string|null)=>void}){
    const [busy,setBusy]=useState(false);
    const [status,setStatus]=useState('');
    const [error,setError]=useState('');
    async function change(published:boolean){
        setBusy(true);setError('');setStatus('');
        try{
            const result=await api<{shareID:string|null}>('/portfolio/sharing','PUT',{published});
            onChange(result.shareID);
            setStatus(result.shareID?'Ссылка создана.':'Ссылка отключена.');
        }catch(err){setError(studyError(err))}
        finally{setBusy(false)}
    }
    const url=shareID?shareURL(shareID):null;
    return <div className="stack">
        <p className="caption">По ссылке доступны ФИО, описание и все карточки портфолио. Изменения будут видны по той же ссылке, пока она включена.</p>
        {url?<div className="stack">
            <label>Ссылка на портфолио<input readOnly className="input" style={{width:'100%'}} value={url} onFocus={event=>event.currentTarget.select()}/></label>
            <div className="filters">
                <button type="button" className="primary" disabled={busy} onClick={async()=>{
                    setStatus('');setError('');
                    try{await navigator.clipboard.writeText(url);setStatus('Ссылка скопирована.')}
                    catch{setError('Выделите ссылку в поле и скопируйте вручную.')}
                }}>Скопировать ссылку</button>
                <a className="secondary" href={url} target="_blank" rel="noopener noreferrer">Открыть</a>
                <button className="secondary" type="button" disabled={busy} onClick={()=>void change(false)}>Отключить ссылку</button>
            </div>
        </div>:<button className="primary" type="button" disabled={busy} onClick={()=>void change(true)}>Создать публичную ссылку</button>}
        {status&&<p className="caption" role="status">{status}</p>}
        {error&&<p className="caption" role="alert">{error}</p>}
    </div>;
}

function DeleteEntry({item,onDelete}:{item:PortfolioItem;onDelete:()=>Promise<void>}){
    const [confirm,setConfirm]=useState(false);
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    return <div style={{marginTop:12}}>
        {confirm?<div className="filters">
            <button type="button" className="secondary" disabled={busy} onClick={async()=>{
                setBusy(true);setError('');
                try{await onDelete()}
                catch(err){setError(studyError(err));setBusy(false)}
            }}>Удалить запись</button>
            <button type="button" className="secondary" disabled={busy} onClick={()=>{setConfirm(false);setError('')}}>Отмена</button>
        </div>:<button type="button" className="link-btn" onClick={()=>setConfirm(true)}>Удалить «{item.title}»</button>}
        {error&&<p className="caption" role="alert">{error}</p>}
    </div>;
}

export function Portfolio({accountID,onSummary,addIcon}:{accountID:string;onSummary:(summary:TodaySummary)=>void;addIcon:ReactNode}){
    const query=useCachedQuery('portfolio',()=>api<PrivatePortfolio>('/portfolio'));
    const summaryQuery=useCachedQuery('portfolio-summary',async()=>{
        const [profile,notices]=await Promise.all([api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications')]);
        return {accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',
            hasUnread:notices.items.some(item=>!item.notificationReadAt)};
    });
    const summary=summaryQuery.data;
    const [panel,setPanel]=useState<Panel|null>(null);
    const [exporting,setExporting]=useState(false);
    const [exportError,setExportError]=useState('');
    const dialog=useRef<HTMLDialogElement>(null);
    useEffect(()=>{if(summary)onSummary(summary)},[summary,onSummary]);
    useEffect(()=>{if(panel&&!dialog.current?.open)dialog.current?.showModal()},[panel]);
    const data=query.data;
    const completeness=data?(Number(!!data.bio)+Number(!!data.items.length))*50:0;
    const item=panel?.type==='editor'?panel.item:undefined;
    const dialogTitle=panel?.type==='sharing'?'Поделиться портфолио':panel?.type==='bio'?'Описание портфолио':item?'Редактировать запись':'Добавить запись';
    return <div className="stack">
        <style>{selectArrowCSS}</style>
        <section className="card">
            <div className="card-head">
                <div><h3 className="card-title">Публичное портфолио</h3>
                    <div className="caption">Соберите достижения в один профиль для преподавателей и работодателей.</div>
                </div>
                <div className="resume-actions">
                    <button className="secondary" type="button" disabled={!data||exporting} onClick={async()=>{
                        setExporting(true);setExportError('');
                        try{await download('/portfolio/resume.pdf','Резюме студента.pdf')}
                        catch(err){setExportError(studyError(err))}
                        finally{setExporting(false)}
                    }}>{exporting?'Формируем PDF…':'Скачать резюме'}</button>
                    <button className="primary" type="button" disabled={!data} onClick={()=>setPanel({type:'sharing'})}>Поделиться</button>
                </div>
            </div>
            {exportError&&<p className="caption" role="alert">{exportError}</p>}
            <div className="bar" role="progressbar" aria-label="Заполненность портфолио" aria-valuenow={completeness}>
                <div className="bar-fill" style={{width:completeness+'%'}}/>
            </div>
            {query.error?<p className="caption" role="alert">{query.error} <button type="button" className="link-btn" onClick={()=>void query.reload()}>Повторить</button></p>
            :<p className="caption">{data?`Профиль заполнен на ${completeness}% · ${data.shareID?'доступен по ссылке':'виден только вам'}`:'Загружаем портфолио…'}</p>}
            {data?.bio&&<p className="caption" style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{data.bio}</p>}
            <button className="link-btn" type="button" disabled={!data} onClick={()=>setPanel({type:'bio'})}>{data?.bio?'Изменить описание':'Добавить описание'}</button>
        </section>
        <div className="portfolio-grid">
            {data?.items.map(entry=><button className="portfolio-card" type="button" key={entry.id} onClick={()=>setPanel({type:'editor',item:entry})}>
                <span className="status blue">{labels[entry.kind]}</span>
                <div className="event-title" style={{marginTop:14,overflowWrap:'anywhere'}}>{entry.title}</div>
                <div className="row-sub" style={{marginTop:6,overflowWrap:'anywhere'}}>{preview(entry.description)}</div>
            </button>)}
            <button className="portfolio-card" type="button" style={{display:'grid',placeItems:'center',minHeight:145,border:'2px dashed var(--line)',boxShadow:'none'}}
                disabled={!data} onClick={()=>setPanel({type:'editor'})}>
                <span className="service-icon" style={{margin:0}}>{addIcon}</span><b>Добавить</b>
            </button>
        </div>
        <dialog className="modal" ref={dialog} aria-labelledby="portfolio-dialog-title" onClose={()=>setPanel(null)}
            style={{border:0,color:'inherit',maxWidth:'calc(100vw - 32px)',maxHeight:'calc(100dvh - 32px)',overflowY:'auto'}}>
            {panel&&data&&<>
                <div className="card-head"><h2 className="card-title" id="portfolio-dialog-title">{dialogTitle}</h2>
                    <button className="icon-btn" type="button" aria-label="Закрыть" onClick={()=>dialog.current?.close()}>×</button></div>
                {panel.type==='sharing'&&<SharePortfolio shareID={data.shareID} onChange={shareID=>query.update(old=>({...(old??data),shareID}))}/>}
                {panel.type==='bio'&&<EditorForm label="Сохранить описание" onSave={async fields=>{
                    const bio=field(fields,'bio');
                    await api('/portfolio','PUT',{bio});
                    query.update(old=>({...(old??data),bio}));
                    dialog.current?.close();
                }}><div className="field"><label htmlFor="portfolio-bio">О себе</label>
                    <textarea id="portfolio-bio" className="textarea" name="bio" maxLength={1500} defaultValue={data.bio}/></div>
                </EditorForm>}
                {panel.type==='editor'&&<>
                    <EditorForm key={item?.id??'new'} label={item?'Сохранить изменения':'Добавить запись'} onSave={async fields=>{
                        const body={kind:field(fields,'kind') as Kind,title:field(fields,'title'),description:field(fields,'description'),url:field(fields,'url')};
                        const path='/portfolio/items'+(item?'/'+item.id:'');
                        const result=await api<{id:string}|undefined>(path,item?'PUT':'POST',body);
                        const entry={...body,id:item?.id??result!.id,url:body.url||null};
                        query.update(old=>{
                            const current=old??data;
                            return {...current,items:item?current.items.map(record=>record.id===item.id?entry:record):[entry,...current.items]};
                        });
                        dialog.current?.close();
                    }}>
                        <div className="field"><label htmlFor="portfolio-kind">Тип</label><select id="portfolio-kind" className="select spaced-select" name="kind" defaultValue={item?.kind}>
                            {Object.entries(labels).map(([key,label])=><option key={key} value={key}>{label}</option>)}
                        </select></div>
                        <div className="field"><label htmlFor="portfolio-title">Название</label><input id="portfolio-title" className="input" name="title" required maxLength={150} defaultValue={item?.title}/></div>
                        <div className="field"><label htmlFor="portfolio-description">Описание</label><textarea id="portfolio-description" className="textarea" name="description" required maxLength={3000} defaultValue={item?.description}/></div>
                        <div className="field"><label htmlFor="portfolio-url">Ссылка на работу или подтверждение</label><input id="portfolio-url" className="input" name="url" type="url" maxLength={1000} defaultValue={item?.url??''}/></div>
                    </EditorForm>
                    {item?.url&&<a className="link-btn" href={item.url} target="_blank" rel="noopener noreferrer">Открыть ссылку</a>}
                    {item&&<DeleteEntry key={item.id} item={item} onDelete={async()=>{
                        await api('/portfolio/items/'+item.id,'DELETE');
                        query.update(old=>{const current=old??data;return {...current,items:current.items.filter(record=>record.id!==item.id)}});
                        dialog.current?.close();
                    }}/>}
                </>}
            </>}
        </dialog>
    </div>;
}

export function PublicPortfolio({id}:{id:string}){
    const [data,setData]=useState<PortfolioView|null>(null);
    const [error,setError]=useState('');
    useEffect(()=>{
        const controller=new AbortController();
        const meta=document.createElement('meta');
        meta.name='robots';meta.content='noindex, nofollow';
        document.head.append(meta);
        void fetch('/api/v1/MGIMO/public/portfolios/'+encodeURIComponent(id),{credentials:'omit',signal:controller.signal})
            .then(result=>{
                if(result.status===404)throw new Error('Портфолио по этой ссылке недоступно.');
                if(!result.ok)throw new Error('Не удалось открыть портфолио. Попробуйте обновить страницу.');
                return result.json() as Promise<PortfolioView>;
            }).then(setData).catch(err=>{if(!controller.signal.aborted)setError(studyError(err))});
        return ()=>{controller.abort();meta.remove()};
    },[id]);
    return <main className="resume-page">
        <div className="resume-document">
            <header className="resume-header">
                <div className="resume-heading-row">
                    <p className="resume-label">РЕЗЮМЕ СТУДЕНТА</p>
                    {data&&<a className="resume-download" download href={'/api/v1/MGIMO/public/portfolios/'+encodeURIComponent(id)+'/resume.pdf'}>Скачать PDF</a>}
                </div>
                {data&&<h1>{data.displayName}</h1>}
            </header>
            {!data&&<p role={error?'alert':'status'}>{error||'Загружаем портфолио…'}</p>}
            {data&&<>
                {data.bio&&<section className="resume-section"><h2>О себе</h2><p>{data.bio}</p></section>}
                {Object.entries(resumeSections).map(([kind,title])=>{
                    const items=data.items.filter(item=>item.kind===kind);
                    if(!items.length)return null;
                    return <section className="resume-section" key={kind}><h2>{title}</h2>{items.map(item=><article key={item.id}>
                        <h3>{item.title}</h3>
                        <p>{item.description}</p>
                        {item.url&&<a href={item.url} target="_blank" rel="noopener noreferrer">Открыть материал</a>}
                    </article>)}</section>;
                })}
                {!data.items.length&&!data.bio&&<p>В портфолио пока нет записей.</p>}
            </>}
        </div>
    </main>;
}
