import {useEffect,useRef,useState} from 'react';
import type {FormEvent} from 'react';
import {allPages,api,response} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {studyError} from './study-format';
import {selectArrowCSS} from './select-arrow';
import type {ApiNotification,ApiProfile,TodaySummary} from './today-model';

type OfferKind='vacancy'|'internship';
type Offer={id:string;kind:OfferKind;title:string;organization:string;location:string;description:string;url:string;facultyID:string;faculty:string};
type PracticeState='assigned'|'submitted'|'revision'|'accepted';
type Practice={id:string;organization:string;startsOn:string;endsOn:string;reportDueOn:string;state:PracticeState;
    feedback:string|null;hasReport:boolean;studentName:string;studentNumber:number};
type Eligibility={studyYear:number;practiceAvailable:boolean};
type Faculty={facultyID:string;title:string};
const labels:Record<OfferKind,string>={vacancy:'Вакансия',internship:'Стажировка'};
const practiceStatus:Record<PracticeState,string>={assigned:'Назначена',submitted:'Отчёт на проверке',revision:'На доработке',accepted:'Завершена'};
const dateFormat=new Intl.DateTimeFormat('ru-RU',{timeZone:'UTC',day:'numeric',month:'long',year:'numeric'});
const dateText=(value:string)=>dateFormat.format(new Date(value));
const values=(form:FormData, names:string[])=>Object.fromEntries(names.map(name=>[name,String(form.get(name)??'').trim()]));
const list=<T,>(path:string)=>allPages<T>(path).then(page=>page.items);

async function downloadReport(id:string) {
    const result=await response('/career/practices/'+id+'/report');
    const url=URL.createObjectURL(await result.blob());
    const link=document.createElement('a');
    link.href=url;
    link.download=result.headers.get('Content-Disposition')?.includes('.docx')?'Отчёт по практике.docx':'Отчёт по практике.pdf';
    link.click();
    window.setTimeout(()=>URL.revokeObjectURL(url),1000);
}

function ReportDownload({practice}:{practice:Practice}){
    const [error,setError]=useState('');
    const [busy,setBusy]=useState(false);
    return <div>
        <button type="button" className="secondary role-primary" disabled={busy} onClick={async()=>{
            setBusy(true);setError('');
            try{await downloadReport(practice.id)}catch(err){setError(studyError(err))}finally{setBusy(false)}
        }}>{busy?'Скачиваем…':'Скачать отчёт'}</button>
        {error&&<p className="caption role-sub" role="alert">{error}</p>}
    </div>;
}

function ReportUpload({practice,reload}:{practice:Practice;reload:()=>Promise<void>}){
    const input=useRef<HTMLInputElement>(null);
    const [file,setFile]=useState<File|null>(null);
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    const pending=useRef(false);
    async function submit(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(!file||pending.current)return;
        if(file.size>10*1024*1024){setError('Файл должен быть не больше 10 МБ.');return}
        pending.current=true;setBusy(true);setError('');
        try{
            const body=new FormData();
            body.append('file',file);
            await api('/career/practices/'+practice.id+'/report','POST',body);
            setFile(null);
            if(input.current)input.current.value='';
            await reload();
        }catch(err){setError(studyError(err))}
        finally{pending.current=false;setBusy(false)}
    }
    return <form style={{marginTop:18}} onSubmit={submit}>
        <input ref={input} type="file" hidden accept=".pdf,.docx" onChange={event=>{setFile(event.target.files?.[0]??null);setError('')}}/>
        <button className="secondary" type="button" disabled={busy} onClick={()=>input.current?.click()}>Выбрать файл</button>
        <p className="caption" style={{overflowWrap:'anywhere'}}>{file?.name??'PDF или DOCX, до 10 МБ'}</p>
        <button className="primary" type="submit" style={{width:'100%'}} disabled={!file||busy}>
            {busy?'Загружаем…':'Загрузить отчёт'}
        </button>
        {error&&<p className="caption" role="alert">{error}</p>}
    </form>;
}

export function Career({accountID,onSummary}:{accountID:string;onSummary:(summary:TodaySummary)=>void}){
    const page=useCachedQuery('career-page',async()=>{
        const [offers,eligibility,profile,notices]=await Promise.all([
            list<Offer>('/career/offers'),api<Eligibility>('/career/eligibility'),
            api<ApiProfile>('/profile/me').catch(()=>null),allPages<ApiNotification>('/notifications').catch(()=>null),
        ]);
        return {offers,eligibility,summary:{accountID,groupLabel:profile?.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',
            hasUnread:notices?.items.some(n=>!n.notificationReadAt)??false}};
    });
    const practices=useCachedQuery(page.data?.eligibility.practiceAvailable?'career-practices':null,()=>list<Practice>('/career/practices'));
    const [all,setAll]=useState(false);
    const [filter,setFilter]=useState<OfferKind|''>('');
    const [selected,setSelected]=useState<Offer|null>(null);
    const [practiceID,setPracticeID]=useState('');
    const dialog=useRef<HTMLDialogElement>(null);
    const summary=page.data?.summary;
    useEffect(()=>{if(summary)onSummary(summary)},[summary,onSummary]);
    useEffect(()=>{if(selected&&!dialog.current?.open)dialog.current?.showModal()},[selected]);
    const offers=page.data?.offers??[];
    const available=page.data?.eligibility.practiceAvailable;
    const rows=practices.data??[];
    const practice=rows.find(record=>record.id===practiceID)??rows.find(record=>record.state!=='accepted')??rows[0];
    const visible=all?offers.filter(offer=>!filter||offer.kind===filter):offers.slice(0,3);
    return <div className="stack">
        <style>{selectArrowCSS}</style>
        <div className="metric-grid">
            <div className="metric"><strong>{page.data?offers.filter(x=>x.kind==='vacancy').length:'—'}</strong><div className="caption">вакансий факультета</div></div>
            <div className="metric"><strong>{page.data?offers.filter(x=>x.kind==='internship').length:'—'}</strong><div className="caption">стажировок</div></div>
            <div className="metric"><strong>{page.data?.eligibility.studyYear??'—'}</strong><div className="caption">курс обучения</div></div>
            <div className="metric"><strong>{practices.data?rows.filter(x=>x.state!=='accepted').length:available===false?0:'—'}</strong><div className="caption">активных практик</div></div>
        </div>
        <div className="content-grid">
            <section className="card">
                <div className="card-head"><h3 className="card-title">{all?'Вакансии и стажировки':'Рекомендованные возможности'}</h3>
                    <button className="link-btn" type="button" onClick={()=>setAll(!all)}>{all?'Свернуть':'Все предложения →'}</button>
                </div>
                {all&&<select className="select spaced-select" aria-label="Вид предложения" value={filter} onChange={event=>setFilter(event.target.value as OfferKind|'')}>
                    <option value="">Все предложения</option><option value="vacancy">Вакансии</option><option value="internship">Стажировки</option>
                </select>}
                <div className="list">{visible.map(offer=><button className="request" key={offer.id} type="button" onClick={()=>setSelected(offer)}>
                    <div><div className="row-name">{offer.title}</div><div className="row-sub">{offer.organization} · {offer.location}</div></div>
                    <span className="status blue">{labels[offer.kind]}</span>
                </button>)}</div>
                {!visible.length&&<p className="caption" role={page.error?'alert':'status'}>{page.error||(page.data?'В этой категории пока нет предложений.':'Загружаем предложения…')}</p>}
            </section>
            <aside className="card">
                <div className="card-head"><h3 className="card-title">Моя практика</h3>{practice&&<span className="status mint">{practiceStatus[practice.state]}</span>}</div>
                <div className="timeline">
                    {available===false&&<p className="caption">Практика доступна с третьего курса.</p>}
                    {available!==false&&!practice&&<p className="caption" role={page.error||practices.error?'alert':'status'}>
                        {page.error||practices.error||(practices.data?'Деканат пока не назначил практику.':'Загружаем данные о практике…')}
                    </p>}
                    {rows.length>1&&<label>Период практики<select className="select spaced-select" value={practice?.id??''} onChange={event=>setPracticeID(event.target.value)}>
                        {rows.map(item=><option value={item.id} key={item.id}>{dateText(item.startsOn)} · {item.organization}</option>)}
                    </select></label>}
                    {practice&&<>
                        <div className="timeline-item"><span className="timeline-dot"/><div><div className="row-name">Место практики</div><div className="row-sub">{practice.organization}</div></div></div>
                        <div className="timeline-item"><span className="timeline-dot"/><div><div className="row-name">Сроки практики</div>
                            <div className="row-sub">{dateText(practice.startsOn)} — {dateText(practice.endsOn)}</div></div></div>
                        <div className="timeline-item"><span className="timeline-dot"/><div><div className="row-name">Срок сдачи отчёта</div><div className="row-sub">До {dateText(practice.reportDueOn)}</div></div></div>
                        {practice.feedback&&<p className="caption" style={{whiteSpace:'pre-wrap'}}>Комментарий деканата · {practice.feedback}</p>}
                        {practice.hasReport&&<ReportDownload practice={practice}/>}
                        {(practice.state==='assigned'||practice.state==='revision')&&<ReportUpload key={practice.id} practice={practice} reload={practices.reload}/>}
                    </>}
                </div>
            </aside>
        </div>
        <dialog ref={dialog} className="modal" aria-labelledby="career-offer-title" onClose={()=>setSelected(null)}
            style={{border:0,maxWidth:'calc(100vw - 32px)',maxHeight:'calc(100dvh - 32px)',overflowY:'auto',color:'inherit'}}>
            {selected&&<>
                <div className="card-head"><h2 className="card-title" id="career-offer-title">{selected.title}</h2>
                    <button className="icon-btn" type="button" aria-label="Закрыть предложение" onClick={()=>dialog.current?.close()}>×</button></div>
                <p className="caption">{labels[selected.kind]} · {selected.organization} · {selected.location}</p>
                <p style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{selected.description}</p>
                <a className="primary" href={selected.url} target="_blank" rel="noopener noreferrer" style={{display:'inline-flex',textDecoration:'none'}}>Перейти к отклику</a>
            </>}
        </dialog>
    </div>;
}

const fieldStyle={display:'block',width:'100%',minWidth:0,font:'inherit',padding:'10px 12px',
    border:'1px solid var(--line)',borderRadius:12,backgroundColor:'#fff',color:'inherit'};
function DeanForm({label,save,children}:{label:string;save:(form:FormData)=>Promise<void>;children:React.ReactNode}){
    const [busy,setBusy]=useState(false),[error,setError]=useState(''),[saved,setSaved]=useState(false);
    async function submit(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(busy)return;
        const form=event.currentTarget;
        setBusy(true);setError('');setSaved(false);
        try{await save(new FormData(form));form.reset();setSaved(true)}
        catch(err){setError(studyError(err))}
        finally{setBusy(false)}
    }
    return <form onSubmit={submit} style={{display:'grid',gap:10}}>
        <fieldset disabled={busy} style={{border:0,padding:0,margin:0,display:'grid',gap:10}}>
            {children}<button type="submit" className="role-primary">{busy?'Сохраняем…':label}</button>
        </fieldset>
        {saved&&<p className="role-sub" role="status">Сохранено</p>}
        {error&&<p className="role-sub" role="alert">{error}</p>}
    </form>;
}

export function DeanCareer(){
    const [practiceMode,setPracticeMode]=useState(false);
    const faculties=useCachedQuery('career-faculties',()=>list<Faculty>('/career/faculties'));
    const offers=useCachedQuery(practiceMode?null:'career-offers',()=>list<Offer>('/career/offers'));
    const practices=useCachedQuery(practiceMode?'career-practices':null,()=>list<Practice>('/career/practices'));
    const [editing,setEditing]=useState<Offer|null>(null),[error,setError]=useState('');
    async function archive(id:string){
        setError('');
        try{await api('/career/offers/'+id,'DELETE');if(editing?.id===id)setEditing(null);await offers.reload()}
        catch(err){setError(studyError(err))}
    }
    return <section className="role-card">
        <style>{selectArrowCSS}</style>
        <div style={{display:'flex',gap:8,flexWrap:'wrap',marginBottom:12}}>
            <button type="button" className="role-primary" aria-pressed={!practiceMode} onClick={()=>setPracticeMode(false)}>Вакансии и стажировки</button>
            <button type="button" className="role-primary" aria-pressed={practiceMode} onClick={()=>setPracticeMode(true)}>Практика</button>
        </div>
        {(error||offers.error||faculties.error||practices.error)&&<p className="role-sub" role="alert">{error||offers.error||faculties.error||practices.error}</p>}
        {practiceMode?<div style={{display:'grid',gap:16}}>
            <DeanForm label="Назначить практику" save={async form=>{
                await api('/career/practices','POST',{...values(form,['organization','startsOn','endsOn','reportDueOn']),studentNumber:Number(form.get('studentNumber'))});
                await practices.reload();
            }}>
                <p className="role-sub">Практика назначается студентам с третьего курса.</p>
                <label>Номер студента<input style={fieldStyle} name="studentNumber" type="number" min={1} max={2147483647} required/></label>
                <label>Организация<input style={fieldStyle} name="organization" maxLength={160} required/></label>
                <label>Начало<input style={fieldStyle} type="date" name="startsOn" required/></label>
                <label>Окончание<input style={fieldStyle} type="date" name="endsOn" required/></label>
                <label>Срок сдачи отчёта<input style={fieldStyle} type="date" name="reportDueOn" required/></label>
            </DeanForm>
            {practices.data?.map(item=><article key={item.id} style={{border:'1px solid var(--line)',borderRadius:12,padding:12,overflowWrap:'anywhere'}}>
                <strong>{item.studentName} · № {item.studentNumber}</strong>
                <p className="role-sub">{item.organization} · {dateText(item.startsOn)} — {dateText(item.endsOn)} · {practiceStatus[item.state]}</p>
                {item.feedback&&<p className="role-sub" style={{whiteSpace:'pre-wrap'}}>{item.feedback}</p>}
                {item.hasReport&&<ReportDownload practice={item}/>}
                {item.state==='submitted'&&<DeanForm label="Сохранить решение" save={async form=>{
                    await api('/career/practices/'+item.id+'/review','PUT',values(form,['state','feedback']));await practices.reload();
                }}>
                    <label>Решение<select className="select spaced-select" style={{...fieldStyle,paddingRight:44}} name="state">
                        <option value="accepted">Принять отчёт</option><option value="revision">Вернуть на доработку</option>
                    </select></label>
                    <label>Комментарий<textarea style={fieldStyle} name="feedback" maxLength={2000}/></label>
                </DeanForm>}
            </article>)}
            {practices.data?.length===0&&<p className="role-sub">Практика пока никому не назначена.</p>}
        </div>:<div style={{display:'grid',gap:16}}>
            {faculties.data?.length===0&&<p className="role-sub">Нет факультетов для публикации.</p>}
            {!!faculties.data?.length&&<DeanForm key={editing?.id??'new'} label={editing?'Сохранить изменения':'Опубликовать предложение'} save={async form=>{
                await api(editing?'/career/offers/'+editing.id:'/career/offers',editing?'PUT':'POST',values(form,['facultyID','kind','title','organization','location','description','url']));
                await offers.reload();setEditing(null);
            }}>
                <label>Факультет<select className="select spaced-select" style={{...fieldStyle,paddingRight:44}} name="facultyID" defaultValue={editing?.facultyID??faculties.data[0].facultyID}>
                    {faculties.data.map(faculty=><option key={faculty.facultyID} value={faculty.facultyID}>{faculty.title}</option>)}
                </select></label>
                <label>Вид<select className="select spaced-select" style={{...fieldStyle,paddingRight:44}} name="kind" defaultValue={editing?.kind}>
                    <option value="vacancy">Вакансия</option><option value="internship">Стажировка</option>
                </select></label>
                <label>Название<input style={fieldStyle} name="title" maxLength={150} defaultValue={editing?.title} required/></label>
                <label>Работодатель<input style={fieldStyle} name="organization" maxLength={160} defaultValue={editing?.organization} required/></label>
                <label>Город или удалённая работа<input style={fieldStyle} name="location" maxLength={120} defaultValue={editing?.location} required/></label>
                <label>Описание<textarea style={fieldStyle} name="description" maxLength={3000} defaultValue={editing?.description} required/></label>
                <label>Ссылка для отклика<input style={fieldStyle} type="url" name="url" maxLength={1000} defaultValue={editing?.url} required/></label>
                {editing&&<button type="button" className="role-primary" onClick={()=>setEditing(null)}>Отменить редактирование</button>}
            </DeanForm>}
            {offers.data?.map(offer=><article key={offer.id} style={{border:'1px solid var(--line)',borderRadius:12,padding:12,overflowWrap:'anywhere'}}>
                <strong>{offer.title}</strong><p className="role-sub">{labels[offer.kind]} · {offer.faculty} · {offer.organization}</p>
                <div style={{display:'flex',gap:8,flexWrap:'wrap'}}>
                    <button type="button" className="role-primary" onClick={()=>setEditing(offer)}>Редактировать</button>
                    <button type="button" className="role-primary" onClick={()=>void archive(offer.id)}>Снять с публикации</button>
                </div>
            </article>)}
            {offers.data?.length===0&&<p className="role-sub">Предложений пока нет.</p>}
        </div>}
    </section>;
}
