import {useState} from 'react';
import {useCachedQuery} from './useCachedQuery';
import type {FormEvent} from 'react';
import {api} from '../api';
import {moscowDate} from './today-model';
import {studyError} from './study-format';

type Faculty = {facultyID: string; title: string};
type Teacher = {facultyID: string; teacherID: string; name: string};
type Room = {classroomID: number; classroomNumber: number; building: string};
type Slot = {weekday: number; startTime: string; endTime: string; teacherID: string; classroomID: number; capacity: number};
type Plan = {title: string; academicYear: number; term: number; slots: Slot[]};
type Period = {startTime: string; endTime: string};
type Catalog = {periods: Period[]; titles: string[]; plans: Plan[]};
const endpoint = '/study/sport-semesters';
const days = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const fields = {display: 'grid', gap: 10, gridTemplateColumns: 'repeat(auto-fit,minmax(min(100%,145px),1fr))'};
const control = {display: 'block', width: '100%', marginTop: 5, padding: '9px 10px', border: '1px solid var(--line)', borderRadius: 10, font: 'inherit', color: 'inherit', background: '#fff'};
const emptySlot = (weekday: number): Slot => ({weekday, startTime: '', endTime: '', teacherID: '', classroomID: 0, capacity: 20});

function startingTerm() {
    const [year, month] = moscowDate().split('-').map(Number);
    return {year: month >= 9 ? year : year - 1, term: month >= 2 && month < 9 ? 2 : 1};
}
const titleKey = (value: string) => value.trim().replace(/\s+/g, ' ').toLocaleLowerCase('ru');

export function DeanSportSemester({faculties, teachers, rooms}: {faculties: Faculty[]; teachers: Teacher[]; rooms: Room[]}) {
    const [initial] = useState(startingTerm);
    const [faculty, setFaculty] = useState(faculties[0]?.facultyID ?? '');
    const [year, setYear] = useState(initial.year);
    const [term, setTerm] = useState(initial.term);
    const [title, setTitle] = useState('');
    const [activeDay, setActiveDay] = useState(1);
    const [slots, setSlots] = useState<Slot[]>([]);
    const [revision, setRevision] = useState(0);
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');

    const query = useCachedQuery(faculty ? 'sport-semesters:'+faculty : null, () => api<Catalog>(endpoint + '?facultyID=' + encodeURIComponent(faculty)),revision);
    const catalog=query.data;
    const loading=!query.ready && !query.error;

    const key = titleKey(title);
    const previous = catalog?.plans.find(p => titleKey(p.title) === key && p.academicYear === (term === 1 ? year - 1 : year) && p.term === (term === 1 ? 2 : 1));
    const existing = catalog?.plans.find(p => titleKey(p.title) === key && p.academicYear === year && p.term === term);
    const shown = slots.map((slot,index)=>({slot,index})).filter(item=>item.slot.weekday===activeDay);
    const last=shown.at(-1)?.slot;
    const complete=(slot:Slot)=>!!slot.startTime&&!!slot.endTime&&!!slot.teacherID&&!!slot.classroomID&&slot.capacity>0;
    const cells=shown.length<6&&(!last||complete(last))?[...shown,{slot:emptySlot(activeDay),index:-1}]:shown;
    const availableTeachers = teachers.filter(t => t.facultyID === faculty);

    function patch(index: number, update: Partial<Slot>) {
        if (index<0) setSlots(items=>[...items,{...emptySlot(activeDay),...update}]);
        else setSlots(items => items.map((slot, i) => i === index ? {...slot, ...update} : slot));
    }

    async function publish(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (busy || !faculty || !title.trim() || existing) return;
        if (slots.some(slot => !slot.startTime || slot.endTime <= slot.startTime || !slot.classroomID || !slot.teacherID || slot.capacity < 1)) {
            setError('Заполни время, аудиторию и преподавателя во всех выбранных днях.');
            return;
        }
        setBusy(true);
        setMessage('');
        setError('');
        try {
            const result = await api<{sessions: number}>(endpoint, 'POST', {facultyID: faculty, academicYear: year, term, title, slots});
            setMessage(`Опубликовано занятий на семестр: ${result.sessions}.`);
            setRevision(value => value + 1);
        } catch (err) {setError(studyError(err));}
        finally {setBusy(false);}
    }

    return <section className="role-card">
        <div className="role-head"><div><h2>Расписание физкультуры</h2>
            <div className="role-sub">Выбери дни и время занятий. Шаблон повторится каждую неделю до конца семестра.</div></div></div>
        <form className="role-list" onSubmit={publish}>
            <div style={fields}>
                <label className="role-sub">Факультет<select style={control} value={faculty} required onChange={event => {
                    setFaculty(event.target.value); setSlots([]); setMessage('');
                }}>{faculties.map(f => <option value={f.facultyID} key={f.facultyID}>{f.title}</option>)}</select></label>
                <label className="role-sub">Год начала учебного года<input style={control} type="number" value={year} onChange={event => setYear(Number(event.target.value))} min={2000} max={2199} required/></label>
                <label className="role-sub">Семестр<select style={control} value={term} onChange={event => setTerm(Number(event.target.value))}>
                    <option value={1}>Осенний · сентябрь — январь</option><option value={2}>Весенний · февраль — август</option>
                </select></label>
            </div>
            <label className="role-sub">Вид спорта<input required style={control} maxLength={128} list="semester-sport-list" value={title} onChange={event => {setTitle(event.target.value); setMessage('');}} placeholder="Выбрать или ввести название"/></label>
            <datalist id="semester-sport-list">{catalog?.titles.map(option => <option key={option} value={option}/>)}</datalist>
            <div><button className="role-primary" type="button" disabled={!previous || !!existing || busy}
                onClick={() => {
                    if (!previous) return;
                    setSlots(previous.slots.map(slot => catalog?.periods.some(p=>p.startTime===slot.startTime&&p.endTime===slot.endTime)
                        ? {...slot} : {...slot,startTime:'',endTime:''}));
                    setActiveDay(previous.slots[0]?.weekday ?? 1);
                    setMessage('Шаблон загружен. Проверь время, преподавателей и аудитории перед публикацией.');
                    setError('');
                }}>Повторить расписание прошлого семестра</button>
                {!previous && title && !loading && <div className="role-sub" style={{marginTop: 6}}>Для этого вида спорта в прошлом семестре шаблона нет.</div>}
            </div>
            <div role="group" aria-label="Дни недели" style={{display: 'grid', gridTemplateColumns: 'repeat(7,minmax(0,1fr))', gap: 4}}>
                {days.map((name, index) => {
                    const count = (existing?.slots ?? slots).filter(slot => slot.weekday === index + 1).length;
                    return <button type="button" key={name} className="role-status" aria-pressed={activeDay === index + 1}
                        onClick={() => setActiveDay(index + 1)}
                        style={{border: activeDay === index + 1 ? '1px solid var(--violet)' : '1px solid var(--line)', padding: '8px 2px', cursor: 'pointer', minWidth: 0}}>
                        {name}{count ? ` · ${count}` : ''}
                    </button>;
                })}
            </div>
            {existing ? <div className="list">
                <p className="role-sub">На этот семестр расписание уже опубликовано.</p>
                {existing.slots.filter(slot => slot.weekday === activeDay).map((slot, i) => <div className="role-row" key={i}>
                    <span>{slot.startTime}–{slot.endTime} · {rooms.find(r => r.classroomID === slot.classroomID)?.classroomNumber ?? slot.classroomID}</span>
                    <span className="role-status">{availableTeachers.find(t => t.teacherID === slot.teacherID)?.name ?? 'Преподаватель недоступен'}</span>
                </div>)}
            </div> : <>
                <div style={{display:'grid',gap:8,gridTemplateColumns:'repeat(auto-fit,minmax(min(100%,175px),1fr))'}}>
                    {cells.map(({slot,index},position) => <div className="role-row" key={index} style={{display:'grid',gap:8,alignContent:'start',minWidth:0}}>
                        <strong className="role-name">Занятие {position + 1}</strong>
                        <label className="role-sub">Время пары (МСК)<select style={control} value={slot.startTime}
                            onChange={event=>{
                                const match=catalog?.periods.find(p=>p.startTime===event.target.value);
                                if(match)patch(index,match);
                                else if(index>=0)setSlots(items=>items.filter((_,i)=>i!==index));
                            }}>
                            <option value="">Не назначено</option>{catalog?.periods.map(period=><option value={period.startTime} key={period.startTime}>{period.startTime}–{period.endTime}</option>)}
                        </select></label>
                        <label className="role-sub">Аудитория или зал<select style={control} disabled={!slot.startTime} value={slot.classroomID || ''}
                            onChange={event=>patch(index,{classroomID:Number(event.target.value)})}>
                            <option value="">Выбрать</option>{rooms.map(room=><option value={room.classroomID} key={room.classroomID}>{room.building} · {room.classroomNumber}</option>)}
                        </select></label>
                        <label className="role-sub">Преподаватель<select style={control} disabled={!slot.startTime} value={slot.teacherID}
                            onChange={event=>patch(index,{teacherID:event.target.value})}>
                            <option value="">Выбрать</option>{availableTeachers.map(t=><option value={t.teacherID} key={t.teacherID}>{t.name}</option>)}
                        </select></label>
                        <label className="role-sub">Мест<input style={control} disabled={!slot.startTime} type="number" min={1} max={10000} value={slot.capacity}
                            onChange={event=>patch(index,{capacity:Number(event.target.value)})}/></label>
                    </div>)}
                </div>
                <div><button className="role-primary" disabled={busy || loading || !slots.length}>{busy ? 'Проверяем и публикуем…' : 'Опубликовать на семестр'}</button></div>
            </>}
            {(error||query.error) && <div className="role-sub" role="alert">{error||query.error}</div>}
            {message && <div className="role-sub" role="status">{message}</div>}
        </form>
    </section>;
}
