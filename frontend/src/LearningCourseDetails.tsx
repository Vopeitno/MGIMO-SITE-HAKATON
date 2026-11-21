import {StudentCourseMaterials} from './StaffTeaching';
import {useCachedQuery} from './useCachedQuery';
import {loadLearningDetails} from './useLearningData';
import {deadlineSubmissionURL} from './deadlines-model';
import {requestDate} from './requests-model';
function DetailInfo({label,value}:{label:string;value:string}){return <div><div className="info-label">{label}</div><div className="info-value">{value}</div></div>}
export default function LearningCourseDetails({courseID,close}:{courseID:string;close:()=>void}){
 const {data,error}=useCachedQuery('learning-course:'+courseID,()=>loadLearningDetails(courseID));
 const grade=data?.scores[0];
 return <div className="modal-bg" onMouseDown={close}><section className="modal" role="dialog" aria-modal="true" aria-label="Дисциплина" onMouseDown={e=>e.stopPropagation()}><div className="card-head"><h2 className="card-title" style={{fontSize:21}}>{data?.course.title||'Дисциплина'}</h2><button className="icon-btn" onClick={close} aria-label="Закрыть дисциплину">×</button></div>
 {!data?<p className="caption" role={error?'alert':undefined}>{error||'Загружаем дисциплину…'}</p>:<><div className="info-grid"><DetailInfo label="Семестр" value={String(data.course.semester)}/><DetailInfo label="Часы" value={String(data.course.hours)}/><DetailInfo label="Преподаватели" value={data.course.teachers?.map(t=>t.teacherName).join(', ')||'Не назначены'}/><DetailInfo label="Баллы из 100" value={grade?.score==null?'Не выставлены':String(grade.score)}/></div><div className="task"><div className="row-name">Описание</div><div className="row-sub" style={{whiteSpace:'pre-wrap'}}>{data.course.description||'Описание пока не опубликовано'}</div></div>
 <StudentCourseMaterials courseID={courseID}/>
 <h3 className="card-title" style={{marginTop:20,marginBottom:12}}>Задания</h3>{data.deadlines.map(d=>{const link=deadlineSubmissionURL(d.submissionURL);return <div className="task" key={d.deadlineID}><div className="row-name">{d.deadlineTitle}</div><div className="row-sub" style={{whiteSpace:'pre-wrap'}}>{d.deadlineDescription}</div><div className="row-sub">{d.deadlineEndAt?`Срок: ${requestDate(d.deadlineEndAt)}`:'Срок не указан'}</div>{link&&<a className="secondary" href={link} target="_blank" rel="noopener noreferrer" style={{display:'inline-flex',marginTop:10,textDecoration:'none'}}>Открыть задание</a>}</div>})}{!data.deadlines.length&&<p className="caption">Задания пока не опубликованы</p>}
 <h3 className="card-title" style={{marginTop:20,marginBottom:12}}>Экзамены</h3>{data.exams.map(e=><div className="task" key={e.examID}><div className="row-name">{e.examTitle}</div><div className="row-sub">{requestDate(e.examStartAt)}{e.examStatus==='canceled'?' · Отменён':e.examStatus==='rescheduled'?' · Перенесён':''}</div></div>)}{!data.exams.length&&<p className="caption">Экзамены пока не назначены</p>}</>}
 <div className="modal-actions"><button className="secondary" onClick={close}>Закрыть</button></div></section></div>
}
