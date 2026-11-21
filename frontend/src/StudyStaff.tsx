import {ActivityScanner} from "./ActivityScanner";
import {DeanSportSemester} from "./DeanSportSemester";
import {useRef,useState} from 'react';
import {useCachedQuery} from './useCachedQuery';
import type {FormEvent, ReactNode} from 'react';
import {allPages, api} from '../api';
import {studyDate, studyError} from './study-format';

type Faculty = {facultyID: string; title: string};
type Requirement = {requirementID: string; courseID: string; title: string; kind: string; direction?: string};
type Period = {periodID: string; facultyID: string; academicYear: number; semester: number; opensAt: string; closesAt: string; isPublished: boolean; closed: boolean; requirements: Requirement[]; streams: {streamID: string; title: string; teacher: string; startAt: string; capacity: number; enrolled: number}[]};
type Management = {
    faculties: Faculty[];
    periods: Period[];
    courses: {courseID: string; facultyID: string; semester: number; title: string}[];
    directions: {directionID: string; facultyID: string; directionName: string}[];
    teachers: {teacherID: string; facultyID: string; name: string}[];
    rooms: {classroomID: number; classroomNumber: number; building: string}[];
    buildings: {buildingID: string; building: string}[];
};
type Sport = {sportID: string; title: string; startAt: string; building?: string};
type Roster = {started: boolean; students: {studentID: string; name: string; number: number; present: boolean}[]};
type FormDataValue = string | number;
const inputStyle = {display: 'block', width: '100%', padding: '11px 12px', border: '1px solid var(--line)', borderRadius: 12, font: 'inherit', color: 'inherit', background: '#fff', marginTop: 6};
const formStyle = {display: 'grid', gap: 12, cursor: 'default'};

function Field({label, children}: {label: string; children: ReactNode}) {
    return <label><span className="role-sub">{label}</span>{children}</label>;
}
function Input({name, label, type = 'text', min, max, placeholder}: {name: string; label: string; type?: string; min?: number; max?: number; placeholder?: string}) {
    return <Field label={label}><input style={inputStyle} name={name} required type={type} min={min} max={max} maxLength={type === 'text' ? 128 : undefined} placeholder={placeholder}/></Field>;
}
function Select({name, label, options, onChange, empty = 'Выберите вариант', optional = false, value}: {name: string; label: string; options: [string | number, string][]; onChange?: (value: string) => void; empty?: string; optional?: boolean; value?: string}) {
    return <Field label={label}><select style={inputStyle} name={name} required={!optional} value={value} defaultValue={value === undefined ? '' : undefined} onChange={event => onChange?.(event.target.value)}>
        <option value="">{empty}</option>{options.map(([id, title]) => <option key={id} value={id}>{title}</option>)}
    </select></Field>;
}
function DateFields({start = 'Начало', end = 'Окончание', startName = 'startAt', endName = 'endAt'}) {
    return <><Input name={startName} label={start + ' (МСК)'} type="datetime-local"/><Input name={endName} label={end + ' (МСК)'} type="datetime-local"/></>;
}
function StudyForm({title, action, children, submit, method = 'POST', onSaved}: {title: string; action: string; children?: ReactNode; submit: string; method?: string; onSaved: () => void}) {
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [failed, setFailed] = useState(false);
    async function save(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (busy) return;
        const form = event.currentTarget;
        const values: Record<string, FormDataValue> = {};
        for (const input of form.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('input[name], select[name], textarea[name]')) {
            values[input.name] = input.type === 'number' || input.name === 'classroomID' ? Number(input.value)
                : input.type === 'datetime-local' ? input.value + ':00+03:00' : input.value;
        }
        setBusy(true);
        setMessage('');
        try {
            await api(action, method, values);
            setFailed(false);
            setMessage('Сохранено');
            onSaved();
        } catch (error) {
            setFailed(true);
            setMessage(studyError(error));
        } finally {
            setBusy(false);
        }
    }
    return <form className="role-row" style={formStyle} onSubmit={save}>
        <h3 className="role-name" style={{margin: 0}}>{title}</h3>
        {children}
        <div><button className="role-primary" disabled={busy}>{busy ? 'Сохраняем…' : submit}</button></div>
        {message && <div className="role-sub" role={failed ? 'alert' : 'status'}>{message}</div>}
    </form>;
}

function RequirementForm({period, data, path, reload}: {period: Period; data: Management; path: string; reload: () => void}) {
    const [kind, setKind] = useState('');
    const [course, setCourse] = useState('');
    return <StudyForm title="Дисциплина учебного плана" action={path + '/requirements'} submit="Добавить дисциплину" onSaved={reload}>
        <Select name="courseID" label="Дисциплина" empty="Создать новую дисциплину" optional onChange={setCourse}
            options={data.courses.filter(c => c.facultyID === period.facultyID && c.semester === period.semester).map(c => [c.courseID, c.title])}/>
        {!course && <><Input name="title" label="Название новой дисциплины"/><Input name="hours" label="Часы" type="number" min={1} max={10000}/></>}
        <Select name="kind" label="Категория" options={ [['common', 'Общая'], ['profile', 'Профильная'], ['physical_education', 'Физкультура']] } onChange={setKind}/>
        {kind === 'profile' && <Select name="directionID" label="Направление" options={data.directions.filter(d => d.facultyID === period.facultyID).map(d => [d.directionID, d.directionName])}/>}
    </StudyForm>;
}

function PeriodCard({period, data, reload}: {period: Period; data: Management; reload: () => void}) {
    const path = '/study/periods/' + period.periodID;
    const courses = new Map(period.requirements.filter(r => r.kind !== 'physical_education').map(r => [r.courseID, r.title]));
    const teachers = data.teachers.filter(t => t.facultyID === period.facultyID);
    return <section className="role-card">
        <div className="role-head"><div><h2>{period.semester}-й семестр · {period.academicYear}/{period.academicYear + 1}</h2>
            <div className="role-sub">{data.faculties.find(f => f.facultyID === period.facultyID)?.title} · {studyDate(period.opensAt)} — {studyDate(period.closesAt)} МСК</div></div>
            <span className="role-status">{!period.isPublished ? 'Черновик' : period.closed ? 'Набор закрыт' : 'Опубликован'}</span></div>
        <div className="role-list">
            {period.requirements.map(r => <div className="role-row" key={r.requirementID}><div className="role-name">{r.title}</div><span className="role-status">{r.direction || (r.kind === 'physical_education' ? 'Свободное посещение' : 'Общая')}</span></div>)}
            {period.streams.map(s => <div className="role-row" key={s.streamID}><div><div className="role-name">{s.title}</div><div className="role-sub">{s.teacher}{s.startAt ? ` · с ${studyDate(s.startAt)} МСК` : ''}</div></div><span className="role-status">{s.enrolled}/{s.capacity} мест занято</span></div>)}
            {!period.isPublished && <RequirementForm period={period} data={data} path={path} reload={reload}/>}
            {!!courses.size && <StudyForm title="Учебный поток" action={path + '/streams'} submit="Добавить поток" onSaved={reload}>
                <Select name="courseID" label="Дисциплина" options={[...courses]}/>
                <Select name="teacherID" label="Преподаватель" options={teachers.map(t => [t.teacherID, t.name])}/>
                <Select name="classroomID" label="Аудитория" options={data.rooms.map(r => [r.classroomID, `${r.building} · ${r.classroomNumber}`])}/>
                <DateFields start="Начало первого занятия" end="Конец первого занятия"/>
                <Input name="weeks" label="Число еженедельных занятий" type="number" min={1} max={40}/>
                <Input name="capacity" label="Мест в потоке" type="number" min={1} max={10000}/>
            </StudyForm>}
            {(!period.isPublished || !period.closed) && <StudyForm
                key={period.isPublished ? 'close' : 'publish'} title={period.isPublished ? 'Закрытие набора' : 'Публикация набора'}
                action={path} method="PATCH" submit={period.isPublished ? 'Закрыть набор' : 'Опубликовать'} onSaved={reload}>
                <input type="hidden" name="action" value={period.isPublished ? 'close' : 'publish'}/>
            </StudyForm>}
        </div>
    </section>;
}

export function DeanStudy() {
    const [period, setPeriod] = useState('');
    const [revision,setRevision]=useState(0);
    const reload=()=>setRevision(v=>v+1);
    const {data,error}=useCachedQuery('study-management',()=>api<Management>('/study/management'),revision);
    if (!data) return <section className="role-card"><p className="role-sub" role="status">{error || 'Загружаем учебные планы…'}</p>{error && <button className="role-primary" onClick={reload}>Обновить</button>}</section>;
    const faculties: [string, string][] = data.faculties.map(f => [f.facultyID, f.title]);
    const current = data.periods.find(p => p.periodID === period);
    return <>
        <section className="role-card">
            <div className="role-head"><h2>Наборы по семестрам</h2><button className="role-primary" onClick={reload}>Обновить</button></div>
            <div className="role-list">
                {error && <p className="role-sub" role="alert">{error}</p>}
                {!faculties.length && <p className="role-sub">За вашим деканатом пока не закреплён факультет</p>}
                {!!data.periods.length && <Select name="period" label="Учебный план" value={period} onChange={setPeriod} options={data.periods.map(p => [p.periodID, `${data.faculties.find(f => f.facultyID === p.facultyID)?.title} · ${p.academicYear}/${p.academicYear+1} · семестр ${p.semester}`])}/>}
                {!!faculties.length && <StudyForm title="Новый набор" action="/study/periods" submit="Создать набор" onSaved={reload}>
                    <Select name="facultyID" label="Факультет" options={faculties}/>
                    <Input name="academicYear" label="Год начала учебного года" type="number" min={1990} max={2200}/>
                    <Input name="semester" label="Номер семестра по учебному плану" type="number" min={1} max={20}/>
                    <DateFields start="Открытие выбора" end="Закрытие выбора" startName="opensAt" endName="closesAt"/>
                </StudyForm>}
            </div>
        </section>
        {current && <PeriodCard key={current.periodID} period={current} data={data} reload={reload}/>}
        {!!faculties.length && <DeanSportSemester faculties={data.faculties} teachers={data.teachers} rooms={data.rooms}/>}
    </>;
}

export function TeacherAttendance() {
    const [sport, setSport] = useState('');
    const [search, setSearch] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const {data:sessionList,error:sessionsError}=useCachedQuery('study-attendance',()=>allPages<Sport>('/study/attendance'));
    const sessions=sessionList?.items;
    const [target,setTarget]=useState({sport:'',search:''});
    const rosterQuery=useCachedQuery(target.sport?'study-roster:'+target.sport+':'+target.search:null,
        ()=>api<Roster>(`/study/attendance/${target.sport}?search=${encodeURIComponent(target.search)}`));
    const roster=rosterQuery.data;
    function load(id:string,searchTerm:string){
        setTarget({sport:id,search:searchTerm.trim()});
        setError('');
        if(id===target.sport&&searchTerm.trim()===target.search)void rosterQuery.reload();
    }
    async function mark(studentID: string, present: boolean) {
        setBusy(true);
        setError('');
        try {
            await api('/study/attendance/' + sport, 'PUT', {studentID, present});
            if(roster)rosterQuery.update({...roster,students:roster.students.map(st=>st.studentID===studentID?{...st,present}:st)});
            void rosterQuery.reload();
        } catch (err) {
            setError(studyError(err));
        } finally {
            setBusy(false);
        }
    }
    return <section className="role-card">
        <div className="role-head"><div><h2>Посещаемость физкультуры</h2><div className="role-sub">Выберите своё занятие и найдите студента по имени или номеру</div></div></div>
        <div className="role-list">
            {(error||sessionsError||rosterQuery.error) && <p className="role-sub" role="alert">{error||sessionsError||rosterQuery.error}</p>}
            {!sessions?.length && <p className="role-sub">{sessions ? 'Назначенных занятий пока нет' : error ? '' : 'Загружаем занятия…'}</p>}
            {!!sessions?.length && <fieldset disabled={busy} style={{border: 0, padding: 0, margin: 0, minWidth: 0}}>
                <Select name="sport" label="Занятие" value={sport} options={sessions.map(s => [s.sportID, `${s.title} · ${studyDate(s.startAt)} МСК`])}
                    onChange={id => {setSport(id); load(id, ''); setSearch('');}}/>
                {sport && <form style={{...formStyle, marginTop: 12}} onSubmit={event => {event.preventDefault(); void load(sport, search);}}>
                    <Field label="Имя или номер студента"><input name="search" style={inputStyle} maxLength={100} value={search} onChange={event => setSearch(event.target.value)}/></Field>
                    <div><button className="role-primary">Найти</button></div>
                </form>}
            </fieldset>}
            {roster && !roster.started && <p className="role-sub">Отметить посещение можно после начала занятия</p>}
            {roster?.students.map(st => <div className="role-row" key={st.studentID}><div><div className="role-name">{st.name}</div><div className="role-sub">№ {st.number} · {st.present ? 'Присутствовал' : 'Посещение не отмечено'}</div></div>
                <button className="role-primary" disabled={busy || !roster.started} onClick={() => void mark(st.studentID, !st.present)}>{st.present ? 'Снять отметку' : 'Подтвердить посещение'}</button></div>)}
            {roster && !roster.students.length && <p className="role-sub">Студенты не найдены. Введите имя или номер для поиска.</p>}
        </div>
    </section>;
}

type PublishedActivity = {eventID: string; title: string; category: string; building: string; startAt: string; remaining: number; facultyID:string; teacherID?:string|null};
type ActivityManagement = {faculties: Faculty[]; buildings: {buildingID: string; building: string}[]; events: PublishedActivity[];
    teachers:{facultyID:string;teacherID:string;name:string}[]};

export function DeanActivities() {
    const [revision,setRevision]=useState(0);
    const {data,error,reload}=useCachedQuery('campus-activities-management',()=>api<ActivityManagement>('/activities/events'),revision);
    const [faculty,setFaculty]=useState('');
    const [selected,setSelected]=useState('');
    const event=data?.events.find(item=>item.eventID===selected);
    const teacherOptions=(facultyID:string):[string,string][]=>data?.teachers.filter(t=>t.facultyID===facultyID).map(t=>[t.teacherID,t.name])??[];

    return <section className="role-card">
        <div className="role-head"><div><h2>Афиша кампуса</h2><div className="role-sub">Публикация мероприятий и назначение ответственных</div></div>
            <button className="role-primary" onClick={()=>void reload()}>Обновить</button></div>
        {!data?<p className="role-sub" role={error?'alert':'status'}>{error||'Загружаем афишу…'}</p>:<div className="role-list">
            {data.faculties.length>0&&<StudyForm title="Новое мероприятие" action="/activities/events" submit="Опубликовать" onSaved={()=>setRevision(value=>value+1)}>
                <Select name="facultyID" label="Факультет организатора" options={data.faculties.map(f=>[f.facultyID,f.title])} onChange={setFaculty}/>
                <Select key={faculty} name="teacherID" label="Ответственный преподаватель" options={teacherOptions(faculty)}/>
                <Select name="category" label="Категория" options={ [['community','Сообщества'],['career','Карьера'],['volunteering','Волонтёрство']] }/>
                <Input name="title" label="Название"/>
                <Field label="Описание"><textarea name="description" style={inputStyle} rows={4} maxLength={2000}/></Field>
                <Select name="buildingID" label="Корпус" options={data.buildings.map(b=>[b.buildingID,b.building])}/>
                <DateFields/>
                <Input name="capacity" label="Количество мест" type="number" min={1} max={10000}/>
            </StudyForm>}
            {data.events.map(item=><button className="role-row" type="button" key={item.eventID} onClick={()=>setSelected(item.eventID)}>
                <span><span className="role-name">{item.title}</span><span className="role-sub" style={{display:'block'}}>{studyDate(item.startAt)} МСК · {item.building}</span></span>
                <span className="role-status">Мест: {item.remaining}</span>
            </button>)}
            {!data.events.length&&<p className="role-sub">Мероприятия пока не опубликованы</p>}
            {event&&<StudyForm key={event.eventID} title={event.title} action={'/activities/events/'+event.eventID+'/host'} method="PUT"
                submit="Сохранить ответственного" onSaved={()=>setRevision(value=>value+1)}>
                <div className="role-sub">Текущий ответственный: {teacherOptions(event.facultyID).find(t=>t[0]===event.teacherID)?.[1]||'Не назначен'}</div>
                <Select name="teacherID" label="Назначить преподавателя" options={teacherOptions(event.facultyID)}/>
            </StudyForm>}
        </div>}
    </section>;
}

export function ActivityCheckIn({manual}: {manual:boolean}) {
    const [selected,setSelected]=useState('');
    const [message,setMessage]=useState('');
    const [failed,setFailed]=useState(false);
    const [busy,setBusy]=useState(false);
    const pending=useRef(false);
    const events=useCachedQuery('campus-check-in',()=>api<PublishedActivity[]>('/activities/check-in'));
    const roster=useCachedQuery(manual&&selected?'campus-roster:'+selected:null,
        ()=>api<Roster>('/activities/events/'+selected+'/registrations'));
    const event=events.data?.find(item=>item.eventID===selected);

    async function scan(code:string){
        if(!event||pending.current)return;
        pending.current=true;setBusy(true);setMessage('');
        try{
            const result=await api<{name:string;already:boolean}>('/activities/events/'+event.eventID+'/check-in','POST',{code});
            setFailed(false);
            setMessage(result.already?`${result.name} · посещение уже подтверждено`:`${result.name} · посещение засчитано`);
        }catch(err){setFailed(true);setMessage(studyError(err));}
        finally{pending.current=false;setBusy(false);}
    }

    async function mark(studentID:string,present:boolean){
        if(!event||pending.current)return;
        pending.current=true;setBusy(true);setMessage('');
        try{
            await api('/activities/events/'+event.eventID+'/registrations/'+studentID,'PUT',{present});
            if(roster.data)roster.update({...roster.data,students:roster.data.students.map(st=>st.studentID===studentID?{...st,present}:st)});
            setFailed(false);setMessage(present?'Посещение подтверждено':'Отметка снята');
        }catch(err){setFailed(true);setMessage(studyError(err));}
        finally{pending.current=false;setBusy(false);}
    }

    return <section className="role-card">
        <div className="role-head"><div><h2>Посещения мероприятий</h2><div className="role-sub">{manual?'Выберите мероприятие и проверьте список записавшихся.':'Выберите назначенное вам мероприятие и отсканируйте билет студента во время события.'}</div></div></div>
        <div className="role-list">
            {!events.data&&<p className="role-sub" role={events.error?'alert':'status'}>{events.error||'Загружаем мероприятия…'}</p>}
            {events.data?.length===0&&<p className="role-sub">Доступных мероприятий пока нет.</p>}
            {!!events.data?.length&&<fieldset disabled={busy} style={{border:0,padding:0,margin:0}}>
                <Select name="event" label="Мероприятие" value={selected} onChange={value=>{setSelected(value);setMessage('')}}
                    options={events.data.map(item=>[item.eventID,`${item.title} · ${studyDate(item.startAt)} МСК`])}/>
            </fieldset>}
            {event&&!manual&&<ActivityScanner key={event.eventID} onCode={code=>void scan(code)} disabled={busy}/>}
            {event&&manual&&<>
                {!roster.data&&<p className="role-sub" role={roster.error?'alert':'status'}>{roster.error||'Загружаем записи…'}</p>}
                {roster.data?.students.length===0&&<p className="role-sub">Студенты ещё не записались.</p>}
                {roster.data&&!roster.data.started&&<p className="role-sub">Отметки доступны после начала мероприятия.</p>}
                {roster.data?.students.map(st=><div className="role-row" key={st.studentID}>
                    <span>{st.name} · № {st.number}</span>
                    <button className="role-primary" disabled={busy||!roster.data?.started}
                        onClick={()=>void mark(st.studentID,!st.present)}>{st.present?'Снять отметку':'Подтвердить посещение'}</button>
                </div>)}
            </>}
            {message&&<p role={failed?'alert':'status'} className="role-sub">{message}</p>}
        </div>
    </section>;
}
