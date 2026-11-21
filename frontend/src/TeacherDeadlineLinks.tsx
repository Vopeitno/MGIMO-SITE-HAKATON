import {useState} from 'react';
import {useCachedQuery} from './useCachedQuery';
import type {FormEvent} from 'react';
import {api} from '../api';
import {deadlineSubmissionURL} from './deadlines-model';
import {loadTeacherDeadlines} from './teacher-deadlines-model';
import type {Assignment} from './teacher-deadlines-model';

function LinkForm({item,onSaved,toast}:{item:Assignment;onSaved:(updated:Assignment)=>void;toast:(message:string)=>void}){
 const[draft,setValue]=useState<string>();const value=draft??item.customSubmissionURL??'';const[busy,setBusy]=useState(false);
 const save=async(reset=false)=>{if(busy)return;const link=reset?null:value.trim();if(link!==null&&!deadlineSubmissionURL(link)){toast('Укажите полную HTTPS-ссылку без логина и пароля');return}setBusy(true);try{const result=await api<{submissionURL:string;customSubmissionURL:string|null}>(`/academic/deadlines/${item.deadlineID}/submission-link`,'PATCH',{submissionURL:link});setValue(undefined);onSaved({...item,...result});toast(reset?'Восстановлен общий переход в Moodle':'Ссылка сохранена и доступна студентам')}catch(error){toast(error instanceof Error?error.message:'Не удалось сохранить ссылку')}finally{setBusy(false)}};
 const submit=(event:FormEvent<HTMLFormElement>)=>{event.preventDefault();void save()};
 const current=deadlineSubmissionURL(item.submissionURL);
 return <form className="role-row" style={{display:'block',cursor:'default'}} onSubmit={submit}>
  <div className="role-name">{item.deadlineTitle}</div><div className="role-sub" style={{marginTop:4}}>{item.courseTitle}</div>
  <label style={{display:'block',marginTop:12}}><span className="role-sub">Ссылка на сдачу задания</span><input type="url" required maxLength={2048} value={value} onChange={e=>setValue(e.target.value)} disabled={busy} placeholder={current||'https://…'} style={{display:'block',width:'100%',marginTop:6,padding:'11px 12px',border:'1px solid var(--line)',borderRadius:12,font:'inherit',color:'inherit',background:'#fff'}}/></label>
  <div className="role-sub" style={{marginTop:7}}>Если отдельная ссылка не задана, студенты переходят на общую страницу Moodle.</div>
  <div style={{display:'flex',gap:10,flexWrap:'wrap',alignItems:'center',marginTop:12}}>
   <button className="role-primary" type="submit" disabled={busy}>{busy?'Сохраняем…':'Сохранить ссылку'}</button>
   <button className="role-primary" type="button" disabled={busy||!item.customSubmissionURL} onClick={()=>void save(true)} style={{background:'var(--violet-soft)',color:'var(--text)'}}>Вернуть общий Moodle</button>
   {current&&<a className="role-status" href={current} target="_blank" rel="noopener noreferrer" style={{textDecoration:'none'}}>Открыть ссылку</a>}
  </div>
 </form>
}
export default function TeacherDeadlineLinks({toast}:{toast:(message:string)=>void}){
 const query=useCachedQuery('teacher-deadlines',loadTeacherDeadlines);
 const {data:items,error}=query;
 return <section className="role-card"><div className="role-head"><div><h2>Ссылки на задания</h2><div className="role-sub">Дедлайны ваших дисциплин. Студенты открывают эти ссылки из своего списка заданий.</div></div><button className="role-primary" onClick={()=>void query.reload()}>Обновить</button></div>
 <div className="role-list">{items?.map(item=><LinkForm key={item.deadlineID} item={item} toast={toast} onSaved={updated=>query.update(list=>(list??[]).map(d=>d.deadlineID===updated.deadlineID?updated:d))}/>)}{!items?.length&&<div className="role-sub">{error||(!items?'Загружаем задания…':'В ваших дисциплинах пока нет дедлайнов')}</div>}</div></section>
}
