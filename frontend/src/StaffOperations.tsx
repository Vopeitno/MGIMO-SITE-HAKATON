import {useState} from 'react';
import {StaffOwnRequests} from './StaffForms';
import {api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {PrivateFile,StaffAction,StaffForm,StaffRow,StaffState} from './StaffCommon';
import {formString,staffInput,staffSelect,useStaffList} from './staff-data';
import type {Directory} from './staff-data';
import {requestDate} from './requests-model';
import type {RequestRecord} from './requests-model';
import {appointmentStatus,moscowDate,safeLessonLink} from './today-model';
import type {ApiLesson} from './today-model';

export type SupportStats={open:number;urgent:number;closed:number;averageFirstReplyMinutes:number|null};
type ScopedStudent={studentID:string;studentName:string;studentNumber:number;studentStatus:string;groupID:number};
type Lesson=ApiLesson&{groupID:number;classroomID:number;teacherID:string};
type RequestRow=RequestRecord&{studentName?:string;studentNumber?:number};

const lessonStates={active:'По расписанию',rescheduled:'Перенесено',canceled:'Отменено'};
const pupilStates:Record<string,string>={active:'Обучается',expelled:'Отчислен',academic_leave:'Академический отпуск',graduated:'Выпустился'};
const dateSpan=(days:number)=>{
    const day=new Date();
    day.setUTCDate(day.getUTCDate()+days);
    return moscowDate(day);
};
const localTime=(instant:string)=>new Date(Date.parse(instant)+3*60*60*1000).toISOString().slice(0,16);

export function StaffSchedule({online=false,editable=false,todayOnly=false}:{online?:boolean;editable?:boolean;todayOnly?:boolean}){
    const [from,setFrom]=useState(()=>dateSpan(0));
    const [to,setTo]=useState(()=>dateSpan(todayOnly?0:6));
    const [selected,setSelected]=useState<number|null>(null);
    const query=useCachedQuery(from&&to&&from<=to?'staff-schedule:'+from+':'+to:null,
        ()=>api<{lessons:Lesson[]}>('/schedule/period?from='+from+'&to='+to));
    const directory=useCachedQuery(editable?'staff-directories':null,()=>api<Directory>('/staff/directories'));
    const lessons=query.data?.lessons.filter(item=>!online||item.lessonStatus!=='canceled'&&!!safeLessonLink(item.remoteLessonLink))??null;
    return <section className="role-card staff-panel">
        {todayOnly?<h2 className="role-name">Занятия сегодня</h2>:<div className="staff-actions">
            <label>С<input className={staffInput} type="date" value={from} onChange={event=>{setFrom(event.target.value);setSelected(null)}}/></label>
            <label>По<input className={staffInput} type="date" min={from} value={to} onChange={event=>{setTo(event.target.value);setSelected(null)}}/></label>
        </div>}
        {from>to?<p className="role-sub" role="alert">Окончание периода раньше начала.</p>:<StaffState data={lessons} error={query.error} empty="На эти даты занятий нет."/>}
        {lessons?.map(lesson=>{
            const link=lesson.lessonStatus!=='canceled'?safeLessonLink(lesson.remoteLessonLink):null;
            return <article key={lesson.lessonID} className="staff-item staff-panel">
                <StaffRow title={lesson.course?.title||lesson.topic} description={`${requestDate(lesson.startAt)} — ${requestDate(lesson.endAt)} · группа ${lesson.groupID}`}
                    status={lessonStates[lesson.lessonStatus as keyof typeof lessonStates]??lesson.lessonStatus}/>
                <div className="role-sub">{lesson.teacher?.teacherName}{link?' · Онлайн':lesson.classroom?` · Аудитория ${lesson.classroom.classroomNumber}`:''}</div>
                {link&&<a href={link} target="_blank" rel="noopener noreferrer">Подключиться к занятию</a>}
                {editable&&<button className="role-primary" type="button" onClick={()=>setSelected(selected===lesson.lessonID?null:lesson.lessonID)}>
                    {selected===lesson.lessonID?'Закрыть форму':'Изменить занятие'}
                </button>}
                {selected===lesson.lessonID&&<StaffForm label="Сохранить изменения" save={async form=>{
                    const remoteLessonLink=formString(form,'link');
                    await api('/lessons/'+lesson.lessonID,'PATCH',{
                        startAt:formString(form,'start')+':00+03:00',endAt:formString(form,'end')+':00+03:00',
                        classroomID:Number(form.get('room')),teacherID:formString(form,'teacher'),remoteLessonLink:remoteLessonLink||null,
                        lessonStatus:formString(form,'status'),reason:formString(form,'reason'),
                    });
                    await query.reload();setSelected(null);
                }}>
                    <label>Начало по Москве<input className={staffInput} type="datetime-local" name="start" defaultValue={localTime(lesson.startAt)} required/></label>
                    <label>Окончание по Москве<input className={staffInput} type="datetime-local" name="end" defaultValue={localTime(lesson.endAt)} required/></label>
                    <label>Преподаватель<select className={staffSelect} name="teacher" defaultValue={lesson.teacherID} required>
                        {!directory.data?.teachers.some(t=>t.teacherID===lesson.teacherID)&&<option value={lesson.teacherID}>{lesson.teacher?.teacherName||'Текущий преподаватель'}</option>}
                        {directory.data?.teachers.map(t=><option key={t.teacherID} value={t.teacherID}>{t.teacherName}</option>)}
                    </select></label>
                    <label>Аудитория<select className={staffSelect} name="room" defaultValue={lesson.classroomID} required>
                        {!directory.data?.classrooms.some(c=>c.classroomID===lesson.classroomID)&&<option value={lesson.classroomID}>{lesson.classroom?.classroomNumber??'Текущая аудитория'}</option>}
                        {directory.data?.classrooms.map(c=><option key={c.classroomID} value={c.classroomID}>
                            {directory.data?.buildings.find(b=>b.buildingID===c.buildingID)?.building} · {c.classroomNumber}
                        </option>)}
                    </select></label>
                    <label>Состояние<select className={staffSelect} name="status" defaultValue={lesson.lessonStatus}>{Object.entries(lessonStates).map(([value,label])=>
                        <option key={value} value={value}>{label}</option>)}</select></label>
                    <label>Ссылка на онлайн-занятие<input className={staffInput} type="url" name="link" defaultValue={lesson.remoteLessonLink??''} maxLength={1024}/></label>
                    <label>Причина изменения<textarea className={staffInput} name="reason" required maxLength={564}/></label>
                    {directory.error&&<p role="alert" className="role-sub">{directory.error}</p>}
                </StaffForm>}
            </article>;
        })}
    </section>;
}

export function DeanStudents(){
    const query=useStaffList<ScopedStudent>('/faculty/students');
    const [search,setSearch]=useState('');
    const values=query.data?.filter(row=>`${row.studentName} ${row.studentNumber}`.toLowerCase().includes(search.trim().toLowerCase()))??null;
    return <section className="role-card staff-panel">
        <label>Поиск по имени или номеру<input className={staffInput} value={search} onChange={event=>setSearch(event.target.value)}/></label>
        <StaffState data={values} error={query.error} empty="Студенты не найдены."/>
        {values?.map(row=><StaffRow key={row.studentID} title={row.studentName} description={`№ ${row.studentNumber} · группа ${row.groupID}`}
            status={pupilStates[row.studentStatus]??row.studentStatus}/>)}
    </section>;
}

type Reports={contingent:{title:string;total:number;active:number}[];
    debts:{studentName:string;studentNumber:number;title:string;score:number}[];
    attendance:{studentName:string;studentNumber:number;visits:number}[]};

export function DeanReports({overview=false}:{overview?:boolean}){
    const query=useCachedQuery('dean-reports',()=>api<Reports>('/staff/dean/reports'));
    const data=query.data;
    return <section className="role-card staff-panel">
        <StaffState data={data} error={query.error}/>
        {data&&<>
            <h2 className="role-name">Контингент</h2>
            <StaffState data={data.contingent} error="" empty="За деканатом не закреплены факультеты."/>
            {data.contingent.map(item=><StaffRow key={item.title} title={item.title} description={`Всего: ${item.total}`} status={`Обучаются: ${item.active}`}/>)}
            {overview?<>
                <StaffRow title="Незачтённые дисциплины" status={data.debts.length}/>
                <StaffRow title="Подтверждённые посещения физкультуры" status={data.attendance.reduce((sum,r)=>sum+r.visits,0)}/>
            </>:<>
                <h2 className="role-name">Академические задолженности</h2>
                <StaffState data={data.debts} error="" empty="Выставленных незачётов нет."/>
                {data.debts.map(item=><StaffRow key={item.studentNumber+':'+item.title} title={item.studentName}
                    description={`№ ${item.studentNumber} · ${item.title}`} status={`${item.score} / 100`}/>)}
                <h2 className="role-name">Учёт посещений физкультуры</h2>
                <StaffState data={data.attendance} error="" empty="Преподаватели пока не подтвердили посещения."/>
                {data.attendance.map(item=><StaffRow key={item.studentNumber} title={item.studentName} description={`№ ${item.studentNumber}`} status={`Посещений: ${item.visits}`}/>)}
            </>}
        </>}
    </section>;
}

export function SupportAnalytics(){
    const query=useCachedQuery('support-analytics',()=>api<SupportStats>('/staff/support/analytics'));
    const data=query.data;
    return <section className="role-card staff-panel">
        <StaffState data={data} error={query.error}/>
        {data&&<>
            <StaffRow title="Открытые обращения" status={data.open}/>
            <StaffRow title="Срочные среди открытых" status={data.urgent}/>
            <StaffRow title="Закрытые обращения" status={data.closed}/>
            <StaffRow title="Среднее время первого ответа" status={data.averageFirstReplyMinutes==null?'Нет ответов':new Intl.NumberFormat('ru-RU').format(data.averageFirstReplyMinutes)+' мин'}/>
        </>}
    </section>;
}

export function StaffRequests({dean=false,teacher=false}:{dean?:boolean;teacher?:boolean}){
    const assigned=useStaffList<RequestRow>(teacher?null:dean?'/staff/dean/requests':'/requests/assigned');
    const unassigned=useStaffList<RequestRow>(dean||teacher?null:'/requests/unassigned');
    const colleagues=useStaffList<{employerID:string;employerName:string}>(dean||teacher?null:'/staff/support/colleagues');
    const [waiting,setWaiting]=useState(false);
    const [own,setOwn]=useState(false);
    const [selected,setSelected]=useState('');
    const query=waiting&&!dean?unassigned:assigned;
    if(teacher)return <section className="role-card staff-panel"><StaffOwnRequests/></section>;
    return <section className="role-card staff-panel">
        {dean&&<div className="staff-actions">
            <button className="role-primary" type="button" aria-pressed={!own} onClick={()=>setOwn(false)}>Заявки факультетов</button>
            <button className="role-primary" type="button" aria-pressed={own} onClick={()=>setOwn(true)}>Мои заявки</button>
        </div>}
        {dean&&own?<StaffOwnRequests/>:<>
        {!dean&&<div className="staff-actions">
            <button className="role-primary" type="button" aria-pressed={!waiting} onClick={()=>{setWaiting(false);setSelected('')}}>Мои заявки</button>
            <button className="role-primary" type="button" aria-pressed={waiting} onClick={()=>{setWaiting(true);setSelected('')}}>Без исполнителя</button>
        </div>}
        <StaffState data={query.data} error={query.error} empty="Заявок пока нет."/>
        {query.data?.map(item=><article className="staff-item staff-panel" key={item.appoinmentID}>
            <StaffRow title={item.appointmentTitle} description={[item.studentName,requestDate(item.created_at)].filter(Boolean).join(' · ')}
                status={appointmentStatus(item.appointmentStatus)}/>
            <p className="staff-copy" style={{margin:0}}>{item.appointmentDescription}</p>
            {!dean&&waiting&&<StaffAction label="Взять в работу" run={async()=>{
                await api('/requests/'+item.appoinmentID+'/claim','POST');await Promise.all([assigned.reload(),unassigned.reload()]);setWaiting(false);
            }}/>}
            {!dean&&!waiting&&<>
                {item.fileUrlAttachment&&<PrivateFile path={'/requests/'+item.appoinmentID+'/attachment'} staff/>}
                {item.appointmentStatus==='proccess'&&!!colleagues.data?.length&&<StaffForm label="Передать заявку" save={async form=>{
                    await api('/requests/'+item.appoinmentID+'/transfer','POST',{employerID:formString(form,'operator')});
                    await assigned.reload();
                }}>
                    <label>Другой сотрудник поддержки<select className={staffSelect} name="operator" required defaultValue="">
                        <option value="" disabled>Выберите сотрудника</option>{colleagues.data.map(item=><option key={item.employerID} value={item.employerID}>{item.employerName}</option>)}
                    </select></label>
                </StaffForm>}
                {(item.appointmentStatus==='creating'||item.appointmentStatus==='proccess')&&<>
                    <button className="role-primary" type="button" onClick={()=>setSelected(selected===item.appoinmentID?'':item.appoinmentID)}>Изменить статус</button>
                    {selected===item.appoinmentID&&<StaffForm label="Сохранить статус" save={async form=>{
                        const appointmentStatus=formString(form,'status');
                        const reason=formString(form,'reason');
                        await api('/requests/'+item.appoinmentID+'/status','PATCH',{appointmentStatus,reason:reason||null});
                        await assigned.reload();setSelected('');
                    }}>
                        <label>Новый статус<select className={staffSelect} name="status">
                            {item.appointmentStatus==='creating'&&<option value="proccess">В обработке</option>}
                            <option value="ready">Готова</option><option value="canceled">Отклонена</option>
                        </select></label>
                        <label>Причина отклонения<textarea className={staffInput} name="reason" maxLength={564}/></label>
                    </StaffForm>}
                </>}
            </>}
        </article>)}
        </>}
    </section>;
}
