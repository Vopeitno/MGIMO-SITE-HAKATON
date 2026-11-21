import {useState} from 'react';
import {api} from '../api';
import {moscowDate} from './today-model';
import {shiftDate} from './schedule-model';
import {studyError} from './study-format';
import {useCachedQuery} from './useCachedQuery';

type SportSession = {
    sportID: string; title: string; teacher: string; buildingID: string; building: string; room: string | null;
    startAt: string; endAt: string; remaining: number; reservable: boolean; cancellable: boolean; reserved: boolean;
};
type Period = {startTime: string; endTime: string};
type Calendar = {
    week: string; used: number; periods: Period[]; defaultBuildingID: string | null;
    buildings: {buildingID: string; building: string}[]; types: string[]; sessions: SportSession[];
};

const zone = 'Europe/Moscow';
const clock = new Intl.DateTimeFormat('ru-RU', {timeZone: zone, hour: '2-digit', minute: '2-digit'});
const dayFormat = new Intl.DateTimeFormat('ru-RU', {timeZone: zone, weekday: 'short', day: 'numeric', month: 'short'});
const weekdayFormat = new Intl.DateTimeFormat('ru-RU', {timeZone: zone, weekday: 'short'});
const weekFormat = new Intl.DateTimeFormat('ru-RU', {timeZone: zone, day: 'numeric', month: 'long'});
const minutes = (time: string) => {const [hour, minute] = time.split(':').map(Number); return hour * 60 + minute;};
const localDate = (day: string) => new Date(day + 'T12:00:00+03:00');
const eventTime = (s: SportSession) => `${clock.format(new Date(s.startAt))}–${clock.format(new Date(s.endAt))}`;
const periodMatches = (s: SportSession, p: Period) => eventTime(s) === `${p.startTime}–${p.endTime}`;

const calendarStyle = `
.sport-calendar-scroll{overflow:auto;border:1px solid var(--line);border-radius:16px}
.sport-calendar-grid{display:grid;grid-template-columns:76px repeat(7,minmax(132px,1fr));min-width:1000px}
.sport-calendar-day,.sport-calendar-timehead{position:sticky;top:0;background:var(--soft);padding:11px 6px;text-align:center;font-size:12px;font-weight:750;z-index:2;border-bottom:1px solid var(--line)}
.sport-calendar-timehead{left:0;z-index:4}
.sport-calendar-hour,.sport-calendar-break-label{position:sticky;left:0;z-index:3;background:var(--soft);display:grid;place-content:center;text-align:center;color:var(--muted);font-size:11px}
.sport-calendar-hour{border-bottom:1px solid var(--line);white-space:nowrap}
.sport-calendar-break,.sport-calendar-break-label{min-height:22px;background:var(--soft);border-bottom:1px solid var(--line)}
.sport-calendar-slot{display:grid;grid-template-columns:repeat(auto-fit,minmax(58px,1fr));align-content:start;gap:4px;min-height:90px;padding:4px;border-left:1px solid var(--line);border-bottom:1px solid var(--line)}
.sport-calendar-event{display:grid;align-content:center;justify-items:start;gap:3px;min-width:0;min-height:82px;overflow:hidden;text-align:left;padding:6px;border:1px solid var(--line);border-radius:8px;background:var(--violet-soft);color:var(--text);font-size:10px;line-height:1.16;cursor:pointer}
.sport-calendar-event>*{display:block;min-width:0;width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.sport-calendar-event>b,.sport-calendar-event>span:first-of-type{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;white-space:normal;overflow-wrap:anywhere}
.sport-calendar-event[aria-pressed="true"]{background:var(--mint);border-color:#6ca994}
.sport-calendar-event:disabled{cursor:default;opacity:.68}
.sport-calendar-event:focus-visible{outline:2px solid #6759d9;outline-offset:-2px}
.sport-calendar-filters{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,190px),1fr));gap:12px}
.sport-calendar-filters .select{appearance:none;-webkit-appearance:none;padding-right:44px;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");background-repeat:no-repeat;background-position:right 16px center;background-size:16px}

.sport-calendar-mobile{display:none}
@media(max-width:680px), (max-width:950px) and (pointer:coarse){
    .sport-calendar-card.card{padding:14px;gap:12px}
    .sport-calendar-card>.card-head{margin-bottom:0;gap:8px}
    .sport-calendar-card>.card-head .secondary{min-height:36px;min-width:36px;padding:0 9px}
    .sport-calendar-scroll,.sport-calendar-off-grid{display:none}
    .sport-calendar-filters{grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}
    .sport-calendar-filters .select{height:40px;padding:0 28px 0 8px;background-position:right 8px center;font-size:14px}
    .sport-calendar-mobile{display:grid;gap:10px}
    .sport-calendar-days{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:3px}
    .app .sport-calendar-day-button{display:grid;justify-items:center;place-content:center;gap:2px;min-height:48px;padding:3px 0;border:1px solid var(--line);border-radius:10px;background:var(--soft);font-size:13px;line-height:1.1;cursor:pointer}
    .sport-calendar-day-button>span{font-size:10px;color:var(--muted);text-transform:capitalize}
    .app .sport-calendar-day-button[aria-pressed=true]{background:var(--violet-soft);border-color:var(--violet)}
    .sport-calendar-day-button::after{content:"";width:4px;height:4px;border-radius:50%}
    .sport-calendar-day-button[data-available=true]::after{background:var(--violet-dark)}
    .sport-calendar-mobile-group{display:grid;gap:5px}
    .sport-calendar-mobile-time{font-size:12px;font-weight:750;color:var(--muted)}
    .app .sport-calendar-mobile .sport-calendar-event{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:start;min-height:0;width:100%;padding:9px 10px;gap:4px 10px;font-size:12px}
    .sport-calendar-mobile .sport-calendar-event>b,.sport-calendar-mobile .sport-calendar-event>span{display:block;white-space:normal;overflow-wrap:anywhere}
    .sport-calendar-mobile .sport-calendar-event>b,.sport-calendar-mobile .sport-calendar-event>span:first-of-type{grid-column:1/-1}
    .sport-calendar-mobile .sport-calendar-event>b{font-size:14px}
    .sport-calendar-mobile .sport-calendar-event>span:last-child{white-space:nowrap}
    .sport-calendar-card .list .request{grid-template-columns:minmax(0,1fr) auto;gap:8px;padding:10px}
    .sport-calendar-card .list .secondary{min-height:36px;padding:4px 9px}
    .sport-calendar-card .list .primary{width:100%}
}
`;

export function SportCalendar() {
    const [desiredWeek, setDesiredWeek] = useState('');
    const query = useCachedQuery('sport-calendar:' + desiredWeek, () => api<Calendar>('/study/sport-calendar'
        + (desiredWeek ? `?week=${encodeURIComponent(desiredWeek)}` : '')));
    const calendar = query.data;
    const [filter, setFilter] = useState('');
    const [building, setBuilding] = useState('');
    const [mobileSelection, setMobileSelection] = useState<{week: string; date: string}>();
    const [draft, setDraft] = useState<{week: string; selected: string[]}>();
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [message, setMessage] = useState('');

    if (!calendar) return <section className="card"><p className="caption" role={query.error ? 'alert' : 'status'}>{query.error || 'Загружаем спортивный календарь…'}</p>
        {query.error && <button className="secondary" onClick={() => void query.reload()}>Повторить</button>}</section>;
    const view = calendar;

    const dates = Array.from({length: 7}, (_, i) => shiftDate(view.week, i));
    const buildingID = view.buildings.some(b => b.buildingID === building) ? building : '';
    const shown = view.sessions.filter(s => (!filter || s.title === filter) && (!buildingID || s.buildingID === buildingID));
    const sessionsByDate = new Map(dates.map(date => [date, [] as SportSession[]]));
    for (const session of shown) sessionsByDate.get(moscowDate(new Date(session.startAt)))?.push(session);
    const today = moscowDate();
    const mobileDate = mobileSelection?.week === view.week ? mobileSelection.date
        : dates.find(date => date >= today && sessionsByDate.get(date)?.length)
            ?? dates.find(date => sessionsByDate.get(date)?.length)
            ?? (dates.includes(today) ? today : view.week);
    const mobileSessionsByTime = new Map<string, SportSession[]>();
    const mobileSessions = [...(sessionsByDate.get(mobileDate) ?? [])].sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt));
    for (const session of mobileSessions) {
        const time = eventTime(session);
        const group = mobileSessionsByTime.get(time) ?? [];
        group.push(session);
        mobileSessionsByTime.set(time, group);
    }
    const reserved = view.sessions.filter(s => s.reserved && s.cancellable).map(s => s.sportID);
    const selected = draft?.week === view.week ? draft.selected : reserved;
    const selectedSet = new Set(selected);
    const changed = reserved.length !== selected.length || reserved.some(id => !selectedSet.has(id));
    const maximum = Math.max(0, 2 - view.used);
    const limit = selected.length >= maximum;

    function toggle(id: string) {
        if (busy || (!selectedSet.has(id) && limit)) return;
        setMessage('');
        setError('');
        setDraft({week: view.week, selected: selectedSet.has(id) ? selected.filter(item => item !== id) : [...selected, id]});
    }

    function navigate(week: string) {
        setDesiredWeek(week);
        setDraft(undefined);
        setError('');
        setMessage('');
    }

    async function confirm() {
        setBusy(true);
        setMessage('');
        setError('');
        try {
            await api('/study/sport-calendar', 'PUT', {week: view.week, sportIDs: selected});
            query.update({
                ...view, sessions: view.sessions.map(s => {
                    if (!s.cancellable) return s;
                    const nowReserved = selectedSet.has(s.sportID);
                    return {...s, reserved: nowReserved, remaining: Math.max(0, s.remaining + Number(s.reserved) - Number(nowReserved))};
                }),
            });
            setDraft(undefined);
            setMessage(selected.length ? 'Выбор подтверждён. Места забронированы.' : 'Будущие записи на эту неделю отменены.');
            void query.reload();
        } catch (err) {setError(studyError(err));} finally {setBusy(false);}
    }

    const renderSession = (session: SportSession, mobile = false) => {
        const picked = selectedSet.has(session.sportID);
        const enabled = session.cancellable && (picked || (!limit && (session.reserved || (session.reservable && session.remaining > 0))));
        const info = [session.title, session.teacher, session.building, session.room, `мест: ${session.remaining}`].filter(Boolean).join(' · ');
        return <button key={session.sportID} type="button" className="sport-calendar-event"
            aria-pressed={picked || (session.reserved && !session.cancellable)} aria-label={info}
            title={info} disabled={!enabled || busy} onClick={() => toggle(session.sportID)}>
            <b>{session.title}</b><span>{session.teacher || 'Преподаватель не назначен'}</span>
            <span>{mobile && session.building ? session.building + ' · ' : ''}{session.room || 'Зал не указан'}</span><span>Мест: {session.remaining}</span>
        </button>;
    };
    const offGrid = shown.filter(s => !view.periods.some(p => periodMatches(s,p)));

    return <section className="card stack sport-calendar-card">
        <style>{calendarStyle}</style>
        <div className="card-head">
            <div><h3 className="card-title">Расписание физкультуры</h3><div className="caption">{weekFormat.format(localDate(dates[0]))} — {weekFormat.format(localDate(dates[6]))} · МСК</div></div>
            <div className="chips">
                <button className="secondary" aria-label="Предыдущая неделя" disabled={busy} onClick={() => navigate(shiftDate(view.week, -7))}>←</button>
                <button className="secondary" aria-label="Следующая неделя" disabled={busy} onClick={() => navigate(shiftDate(view.week, 7))}>→</button>
            </div>
        </div>
        <div className="sport-calendar-filters">
            <label><span className="caption">Вид спорта</span><select className="select" value={filter} onChange={event => setFilter(event.target.value)}>
                <option value="">Все виды</option>{view.types.map(type => <option key={type}>{type}</option>)}
            </select></label>
            <label><span className="caption">Корпус</span><select className="select" value={buildingID} onChange={event => setBuilding(event.target.value)}>
                <option value="">Все корпуса</option>{view.buildings.map(item => <option key={item.buildingID} value={item.buildingID}>{item.building}</option>)}
            </select></label>
        </div>
        {!shown.length && <div className="caption sport-calendar-off-grid">На этой неделе нет занятий по выбранным фильтрам.</div>}
        <div className="sport-calendar-mobile">
            <div className="sport-calendar-days" role="group" aria-label="День недели">
                {dates.map(date => <button key={date} type="button" className="sport-calendar-day-button"
                    aria-pressed={mobileDate === date} aria-current={today === date ? 'date' : undefined}
                    aria-label={dayFormat.format(localDate(date))} data-available={!!sessionsByDate.get(date)?.length}
                    onClick={() => setMobileSelection({week: view.week, date})}>
                    <span>{weekdayFormat.format(localDate(date))}</span><b>{Number(date.slice(-2))}</b>
                </button>)}
            </div>
            {mobileSessions.length ? Array.from(mobileSessionsByTime, ([time, sessions]) => <div className="sport-calendar-mobile-group" key={time}>
                <div className="sport-calendar-mobile-time">{time}</div>
                {sessions.map(session => renderSession(session, true))}
            </div>) : <p className="caption">На этот день нет занятий по выбранным фильтрам.</p>}
        </div>
        <div className="sport-calendar-scroll" tabIndex={0} aria-label="Недельная сетка физкультуры">
            <div className="sport-calendar-grid">
                <div className="sport-calendar-timehead">МСК</div>
                {dates.map(date => <div className="sport-calendar-day" key={date}>{dayFormat.format(localDate(date))}</div>)}
                {view.periods.map((period, index) => <CalendarRow key={period.startTime} period={period}
                    pause={index ? minutes(period.startTime) - minutes(view.periods[index-1].endTime) : 0}
                    dates={dates} shown={shown} renderSession={renderSession}/>)}
            </div>
        </div>
        {!!offGrid.length && <details className="task sport-calendar-off-grid"><summary className="row-name">Ранее назначенные занятия вне сетки</summary>
            {offGrid.map(s => <div key={s.sportID}><span className="caption">{dayFormat.format(new Date(s.startAt))} · {eventTime(s)}</span>{renderSession(s)}</div>)}
        </details>}
        <div className="list">
            <h3 className="card-title">Мой выбор · {view.used + selected.length}/2</h3>
            {view.sessions.filter(s => selectedSet.has(s.sportID)).map(session => <div className="request" key={session.sportID}>
                <div><div className="row-name">{session.title}</div><div className="row-sub">{dayFormat.format(new Date(session.startAt))} · {eventTime(session)} · {session.building}{session.room ? ' · ' + session.room : ''}</div></div>
                <button className="secondary" disabled={busy} onClick={() => toggle(session.sportID)}>Убрать</button>
            </div>)}
            <p className="caption">{limit ? 'Достигнут лимит двух занятий на неделю.' : 'Нажми на занятия, чтобы выбрать их.'}</p>
            {error && <div role="alert" className="caption">{error} <button className="secondary" onClick={() => void query.reload()}>Обновить</button></div>}
            {message && <p role="status" className="caption">{message}</p>}
            <div><button className="primary" disabled={!changed || busy || selected.length > maximum} onClick={() => void confirm()}>{busy ? 'Сохраняем…' : 'Подтвердить выбор'}</button></div>
        </div>
    </section>;
}

function CalendarRow({period,pause,dates,shown,renderSession}: {period: Period; pause: number; dates: string[]; shown: SportSession[]; renderSession: (session: SportSession) => React.ReactNode}) {
    return <>
        {pause > 0 && <><div className="sport-calendar-break-label">{pause} мин</div>{dates.map(day => <div key={day} className="sport-calendar-break"/>)}</>}
        <div className="sport-calendar-hour"><b>{period.startTime}</b><span>{period.endTime}</span></div>
        {dates.map(date => <div className="sport-calendar-slot" key={date}>
            {shown.filter(s => moscowDate(new Date(s.startAt)) === date && periodMatches(s,period)).map(session => renderSession(session))}
        </div>)}
    </>;
}
