import {useState} from 'react';
import {useCachedQuery} from './useCachedQuery';
import {api} from '../api';
import {SportCalendar} from './SportCalendar';
import {studyDate, studyError} from './study-format';

type Session = {startAt: string; endAt: string};
type Stream = {streamID: string; teacher: string; remaining: number; selected: boolean; open: boolean; lessons: Session[]};
type Plan = {periodID: string; semester: number; opensAt: string; closesAt: string; open: boolean; requirements: {courseID: string; title: string; kind: string; streams: Stream[]}[]};
type Physical = {semester: number | null; points: number; credit: boolean; weeklyVisits: number; visits: {title: string; attendedAt: string}[]};

const moscow = 'Europe/Moscow';
const weekday = new Intl.DateTimeFormat('ru-RU', {weekday: 'long', timeZone: moscow});
const clock = new Intl.DateTimeFormat('ru-RU', {hour: '2-digit', minute: '2-digit', timeZone: moscow});
export function LearningEnrollment({onSaved}: {onSaved: () => void}) {
    const [choicesDraft, setChoices] = useState<{period:string;values:Record<string,string>}>();
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [saved, setSaved] = useState(false);
    const [revision, setRevision] = useState(0);

    const query=useCachedQuery('study-selection',()=>api<Plan|null>('/study/selection'),revision);
    const plan=query.data;
    const persisted=Object.fromEntries((plan?.requirements||[]).flatMap(course=>{
        const selected=course.streams.find(stream=>stream.selected);
        return selected?[[course.courseID,selected.streamID]]:[];
    }));
    const choices=choicesDraft&&choicesDraft.period===plan?.periodID?choicesDraft.values:persisted;

    async function confirm() {
        if (!plan || busy) return;
        setBusy(true);
        setError('');
        try {
            await api('/study/selection', 'PUT', {periodID: plan.periodID, streamIDs: plan.requirements.map(c => choices[c.courseID])});
            query.update({...plan,requirements:plan.requirements.map(course=>({...course,streams:course.streams.map(stream=>({...stream,selected:choices[course.courseID]===stream.streamID}))}))});
            setChoices(undefined);
            setSaved(true);
            setRevision(value => value + 1);
            onSaved();
        } catch (error) {
            setError(studyError(error));
        } finally {
            setBusy(false);
        }
    }

    if (!plan) return <section className="card"><div className="caption" role="status">{error || query.error || (query.ready ? 'Деканат ещё не опубликовал набор на следующий семестр' : 'Загружаем учебный план…')}</div>{(error||query.error) && <button className="secondary" onClick={() => {setError(''); setRevision(x => x + 1);}}>Обновить</button>}</section>;
    const total = plan.requirements.length;
    const selected = plan.requirements.filter(c => choices[c.courseID]).length;
    const complete = total > 0 && selected === total;
    return <div className="content-grid">
        <section className="card">
            <div className="card-head"><div><h3 className="card-title">Индивидуальная траектория</h3><div className="caption">{plan.semester}-й семестр · По одному потоку на каждую дисциплину</div></div><span className="status blue">{selected}/{total} выбрано</span></div>
            <div className="list">{plan.requirements.map(course => <div className="list" role="group" aria-label={course.title} key={course.courseID}>
                <div><div className="row-name">{course.title}</div><div className="row-sub">{course.kind === 'profile' ? 'Профильная дисциплина' : 'Общая дисциплина'}</div></div>
                {course.streams.map(stream => {
                    const active = choices[course.courseID] === stream.streamID;
                    const lesson = stream.lessons[0];
                    const time = lesson ? `${weekday.format(new Date(lesson.startAt))} · ${clock.format(new Date(lesson.startAt))}–${clock.format(new Date(lesson.endAt))}` : 'Нет занятий';
                    return <button className="request" key={stream.streamID} aria-pressed={active}
                        disabled={!plan.open || busy || (!stream.selected && (!stream.open || stream.remaining === 0)) || !lesson}
                        onClick={() => {setSaved(false); setChoices({period:plan.periodID,values:{...choices,[course.courseID]:stream.streamID}});}}>
                        <div><div className="row-name">{time}</div><div className="row-sub">{stream.teacher} · свободных мест: {stream.remaining}</div>
                            {lesson && <div className="row-sub">{stream.lessons.length} занятий · первое {studyDate(lesson.startAt)} МСК</div>}</div>
                        <span className={`status ${active ? 'mint' : 'gray'}`}>{active ? 'Выбрано' : stream.remaining && stream.open ? 'Выбрать' : 'Недоступно'}</span>
                    </button>;
                })}
                {!course.streams.length && <div className="caption">Доступных потоков пока нет</div>}
            </div>)}
            {!total && <div className="caption">Для вашего направления пока нет дисциплин с выбором времени</div>}</div>
        </section>
        <aside className="card" style={{background: 'var(--violet-soft)'}}>
            <h3 className="card-title">Моя траектория</h3>
            <p className="caption">Выбери удобное время. При подтверждении сервер проверит места и пересечения занятий.</p>
            <p className="caption">{plan.open ? `Выбор до ${studyDate(plan.closesAt)} МСК` : `Окно выбора закрыто. Период: ${studyDate(plan.opensAt)} — ${studyDate(plan.closesAt)} МСК`}</p>
            <div className="bar"><div className="bar-fill" style={{width: `${total ? 100 * selected / total : 0}%`}}/></div>
            <button className="primary" disabled={!complete || !plan.open || busy} style={{width: '100%', marginTop: 18, opacity: complete && plan.open && !busy ? 1 : .5}} onClick={() => void confirm()}>{busy ? 'Сохраняем…' : 'Подтвердить выбор'}</button>
            {saved && <p className="caption" role="status">Выбор сохранён. Курсы и расписание обновлены.</p>}
            {error && <><p className="caption" role="alert">{error}</p><button className="secondary" onClick={() => {setError(''); setRevision(x => x + 1);}}>Обновить потоки</button></>}
        </aside>
    </div>;
}

export function LearningPhysical() {
    const {data,error}=useCachedQuery('physical-education',()=>api<Physical>('/study/physical-education'));
    return <div className="stack">
        <SportCalendar/>
        <aside className="card">
            {!data ? <div className="caption" role="status">{error || 'Загружаем посещения…'}</div> : <>
                <div className="card-head"><h3 className="card-title">Посещения за семестр</h3><span className={`status ${data.credit ? 'mint' : 'blue'}`}>{data.points} / 100 баллов</span></div>
                <p className="caption">{data.credit ? 'Зачёт получен' : 'Для зачёта нужно 100 баллов'}. Преподаватель подтверждает посещение, за него начисляется 5 баллов.</p>
                <p className="caption">На этой неделе: {data.weeklyVisits} из 2 посещений</p>
                <div className="list">{data.visits.map((v, index) => <div key={v.attendedAt + v.title + index}>
                    <div className="row-name">{v.title}</div><div className="row-sub">{studyDate(v.attendedAt)} МСК · +5 баллов</div>
                </div>)}
                {!data.visits.length && <div className="caption">Подтверждённых посещений пока нет</div>}</div>
            </>}
        </aside>
    </div>;
}
