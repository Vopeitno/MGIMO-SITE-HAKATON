import RequestDraftForm from './RequestDraftForm';
import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { api, download, allPages } from '../api';
import type { User, Page } from '../api';
type Appointment = {
    appoinmentID: string;
    appointmentTitle: string;
    appointmentDescription: string;
    appointmentStatus: string;
    created_at: string;
    fileUrlAttachment?: string;
};
type History = {
    appointmentHistoryID: string;
    appointmentStatus: string;
    changedAt: string;
    reason?: string;
};
type Lesson = {
    lessonID: number;
    topic: string;
    startAt: string;
    endAt: string;
    lessonStatus: string;
    remoteLessonLink?: string;
    teacher?: {
        teacherName: string;
    };
    classroom?: {
        classroomNumber: number;
    };
    building?: {
        buildingAddress: string;
    };
    course?: {
        title: string;
    };
};
type Course = {
    courseID: string;
    title: string;
    description?: string;
    hours: number;
    semester: number;
};
type Grade = {
    courseGraduate: string;
    courseID: string;
    studentID: string;
    score: number | null;
    credit: boolean | null;
};
type Notification = {
    notificationID: string;
    notificationTitle: string;
    notificationMessage: string;
    notificationCreatedAt: string;
    notificationReadAt: string | null;
    appoinmentID?: string;
    lessonID?: number;
    sportID?: string;
    courseID?: string;
    newsID?: number;
};
type Sport = {
    sportID: string;
    sportTitle: string;
    sportDescription?: string;
    sportStartAt: string;
    sportCapacity: number;
    enrolledCount?: number;
};
type Registration = {
    sportID: string;
    registrationStatus: string;
};
type Conversation = {
    conversationID: string;
    participants: string[];
    updatedAt: string;
};
type Message = {
    messageID: string;
    conversationID: string;
    senderAccountID: string;
    text: string;
    createdAt: string;
    readAt: string | null;
};
const statuses: Record<string, string> = { creating: 'Создана', proccess: 'В обработке', ready: 'Готова', canceled: 'Отменена', active: 'Активно', rescheduled: 'Перенесено' };
const date = (value?: string) => value ? new Date(value).toLocaleString('ru-RU') : 'Не указано';
const errorText = (error: unknown) => error instanceof Error ? error.message : 'Не удалось выполнить запрос';
function Status({ value }: {
    value: string;
}) { return <span className={`status ${value === 'ready' ? 'mint' : value === 'canceled' ? 'gray' : 'blue'}`}>{statuses[value] || value}</span>; }
function Empty() { return <p className="caption">Записей пока нет.</p>; }
function useRemote<T>(path: string, all = false) {
    const [state, setState] = useState<{
        path: string;
        data?: T;
        error?: string;
    }>({ path });
    const [revision, setRevision] = useState(0);
    useEffect(() => { let live = true; (all ? allPages(path) as Promise<T> : api<T>(path)).then(data => { if (live)
        setState({ path, data }); }).catch(error => { if (live)
        setState({ path, error: errorText(error) }); }); return () => { live = false; }; }, [path, revision, all]);
    return { data: state.path === path ? state.data : undefined, error: state.path === path ? state.error : undefined, reload: () => { setState({ path }); setRevision(v => v + 1); } };
}
function Remote({ data, error, reload, children }: {
    data: unknown;
    error?: string;
    reload: () => void;
    children: ReactNode;
}) { if (error)
    return <section className="card"><p role="alert">{error}</p><button className="secondary" onClick={reload}>Повторить</button></section>; if (data === undefined)
    return <section className="card" role="status">Загрузка…</section>; return <>{children}</>; }
function Pager({ page, offset, setOffset }: {
    page?: Page<unknown>;
    offset: number;
    setOffset: (n: number) => void;
}) { return <div className="filters" style={{ marginTop: 16 }}><button className="secondary" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 20))}>Назад</button><span className="caption">Всего: {page?.total ?? '…'}</span><button className="secondary" disabled={!page || offset + page.items.length >= page.total} onClick={() => setOffset(offset + 20)}>Далее</button></div>; }
function useAction() { const [busy, setBusy] = useState(false); const [error, setError] = useState(''); return { busy, error, run: async (action: () => Promise<void>) => { setBusy(true); setError(''); try {
        await action();
    }
    catch (error) {
        setError(errorText(error));
    }
    finally {
        setBusy(false);
    } } }; }
function ActionError({ error }: {
    error: string;
}) { return error ? <p className="danger" role="alert">{error}</p> : null; }
function Field({ name, label, type = 'text', required = true, maxLength }: {
    name: string;
    label: string;
    type?: string;
    required?: boolean;
    maxLength?: number;
}) { return <div className="field"><label htmlFor={name}>{label}</label><input id={name} name={name} className="input" type={type} required={required} maxLength={maxLength}/></div>; }
function RequestDetails({ id, user, close, changed }: {
    id: string;
    user: User;
    close: () => void;
    changed: () => void;
}) {
    const request = useRemote<Appointment>(`/requests/${id}`);
    const history = useRemote<Page<History>>(`/requests/${id}/history?limit=100&offset=0`, true);
    const action = useAction();
    return <div className="modal-bg"><section className="modal" role="dialog" aria-modal="true" aria-label="Карточка заявки"><div className="card-head"><h2 className="card-title">Заявка</h2><button className="secondary" onClick={close}>Закрыть</button></div><Remote {...request}>{request.data && <><h3>{request.data.appointmentTitle}</h3><Status value={request.data.appointmentStatus}/><p className="caption">№ {id} · {date(request.data.created_at)}</p><p style={{ whiteSpace: 'pre-wrap' }}>{request.data.appointmentDescription}</p>{request.data.fileUrlAttachment && <button className="secondary" disabled={action.busy} onClick={() => action.run(() => download(`/requests/${id}/attachment`, 'вложение'))}>Скачать вложение</button>}<form onSubmit={e => { e.preventDefault(); const file = new FormData(e.currentTarget).get('file'); if (file instanceof File && file.size)
        void action.run(async () => { const body = new FormData(); body.append('file', file); await api(`/requests/${id}/attachment`, 'POST', body); request.reload(); changed(); }); }}><div className="field"><label>Добавить / заменить вложение</label><input name="file" type="file" required accept=".pdf,.png,.jpg,.jpeg,.docx"/></div><button className="secondary" disabled={action.busy}>Загрузить</button></form>{user.role === 'SUPPORT' && ['creating', 'proccess'].includes(request.data.appointmentStatus) && <form onSubmit={e => { e.preventDefault(); const data = new FormData(e.currentTarget); void action.run(async () => { await api(`/requests/${id}/status`, 'PATCH', { appointmentStatus: data.get('status'), reason: data.get('reason') || undefined }); request.reload(); history.reload(); changed(); }); }}><div className="field"><label>Новый статус</label><select name="status" className="select"><option value={request.data.appointmentStatus === 'creating' ? 'proccess' : 'ready'}>{request.data.appointmentStatus === 'creating' ? 'В обработку' : 'Готова'}</option><option value="canceled">Отменить (укажите причину)</option></select></div><Field name="reason" label="Причина" required={false} maxLength={564}/><button className="primary" disabled={action.busy}>Сохранить статус</button></form>}<h3 className="card-title" style={{ marginTop: 20 }}>История</h3><Remote {...history}>{history.data?.items.map(h => <div className="simple-row" key={h.appointmentHistoryID}><Status value={h.appointmentStatus}/><div className="caption">{date(h.changedAt)} {h.reason}</div></div>)}{history.data && !history.data.items.length && <Empty />}</Remote></>}</Remote><ActionError error={action.error}/></section></div>;
}
function Requests({ user }: {
    user: User;
}) {
    const [offset, setOffset] = useState(0);
    const [filter, setFilter] = useState('');
    const [queue, setQueue] = useState('assigned');
    const [selected, setSelected] = useState<string | null>(null);
    const [creating, setCreating] = useState(false);
    const action = useAction();
    const base = user.role === 'SUPPORT' ? `/requests/${queue}` : '/requests';
    const remote = useRemote<Page<Appointment>>(`${base}?limit=20&offset=${offset}${filter && user.role !== 'SUPPORT' ? `&appointmentStatus=${filter}` : ''}`);
    return <div className="content-grid"><section className="card"><div className="card-head"><h3 className="card-title">История заявок</h3>{user.role !== 'SUPPORT' && <button className="primary" onClick={() => setCreating(true)}>＋ Новая заявка</button>}</div>{user.role === 'SUPPORT' && <div className="tabs">{['assigned', 'unassigned'].map(q => <button className={`tab ${queue === q ? 'active' : ''}`} key={q} onClick={() => { setQueue(q); setOffset(0); }}>{q === 'assigned' ? 'Мои обращения' : 'Без исполнителя'}</button>)}</div>}<div className="filters" style={{ marginBottom: 14 }}>{(user.role === 'SUPPORT' ? [] : ['', 'creating', 'proccess', 'ready', 'canceled']).map(s => <button className={`filter ${filter === s ? 'active' : ''}`} key={s} onClick={() => { setFilter(s); setOffset(0); }}>{statuses[s] || 'Все'}</button>)}</div><Remote {...remote}><div className="list">{remote.data?.items.map(r => <div key={r.appoinmentID}><button className="request" style={{ width: '100%' }} onClick={() => setSelected(r.appoinmentID)}><div><div className="row-name">{r.appointmentTitle}</div><div className="request-meta">№ {r.appoinmentID} · {date(r.created_at)}</div></div><Status value={r.appointmentStatus}/></button>{user.role === 'SUPPORT' && queue === 'unassigned' && <button className="secondary" disabled={action.busy} onClick={() => action.run(async () => { await api(`/requests/${r.appoinmentID}/claim`, 'POST'); remote.reload(); })}>Взять в работу</button>}</div>)}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote><ActionError error={action.error}/></section><aside className="stack"><section className="card" style={{ background: 'var(--violet-soft)' }}><h3 className="card-title">Справки и обращения</h3><p className="caption">Статус и история загружаются из деканата. Выберите заявку, чтобы открыть её карточку.</p></section></aside>{creating && <RequestDraftForm close={() => setCreating(false)} created={id => { setCreating(false); setSelected(id); remote.reload(); }}/>}{selected && <RequestDetails id={selected} user={user} close={() => setSelected(null)} changed={remote.reload}/>}</div>;
}
function Profile({ user, logout }: {
    user: User;
    logout: () => Promise<void>;
}) {
    const remote = useRemote<{
        student?: Record<string, unknown>;
        teacher?: Record<string, unknown>;
        employer?: Record<string, unknown>;
        deaneries?: {
            deaneryID: number;
            name: string;
            email?: string;
            phone?: string;
        }[];
    }>('/profile/me');
    const p = remote.data?.student || remote.data?.teacher || remote.data?.employer;
    const prefix = user.role === 'STUDENT' ? 'student' : user.role === 'TEACHER' ? 'teacher' : 'employer';
    return <Remote {...remote}><div className="profile-grid"><section className="card profile-card"><div className="profile-avatar">{user.displayName?.split(' ').map(s => s[0]).slice(0, 2).join('')}</div><h3 className="card-title">{String(p?.[prefix + 'Name'] || user.displayName)}</h3><p className="caption">{user.email}</p><button className="setting danger" onClick={logout}>Выйти из аккаунта</button></section><section className="card"><h3 className="card-title">Личные данные</h3><div className="info-grid" style={{ marginTop: 16 }}>{[["Телефон", p?.[prefix + 'PhoneNumber']], ["Дата рождения", p?.[prefix + 'Birthday'] ? date(String(p[prefix + 'Birthday'])) : undefined], ["Номер", p?.[prefix + 'Number']], ["Группа", p?.groupID], ["Статус", p?.[prefix + 'Status']], ["Должность", p?.teacherPosition]].map(([label, value]) => <div className="info" key={String(label)}><div className="info-label">{String(label)}</div><div className="info-value">{value == null ? 'Не указано' : String(value)}</div></div>)}</div><p className="caption">Изменение профиля и фотографии не предусмотрено API. Для исправления данных обратитесь в деканат.</p>{remote.data?.deaneries?.map(d => <div className="info" key={d.deaneryID}>{d.name} {d.email} {d.phone}</div>)}</section></div></Remote>;
}
function Schedule() { const [day, setDay] = useState(() => { const d = new Date(); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`; }); const [period, setPeriod] = useState(false); const [to, setTo] = useState(day); const remote = useRemote<{
    lessons: Lesson[];
}>(period ? `/schedule/period?from=${day}&to=${to}` : `/schedule?date=${day}`); return <section className="card"><div className="toolbar"><div className="tabs"><button className={`tab ${!period ? 'active' : ''}`} onClick={() => setPeriod(false)}>День</button><button className={`tab ${period ? 'active' : ''}`} onClick={() => setPeriod(true)}>Период</button></div><input aria-label="Дата начала" className="input" type="date" value={day} onChange={e => setDay(e.target.value)}/>{period && <input aria-label="Дата окончания" className="input" type="date" min={day} value={to} onChange={e => setTo(e.target.value)}/>}</div><Remote {...remote}><div className="list">{remote.data?.lessons.map(l => <div className="lesson-row" key={l.lessonID}><div className="row-sub">{date(l.startAt)} — {new Date(l.endAt).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}</div><div><div className="row-name">{l.course?.title || l.topic}</div><div className="row-sub">{l.topic} · {l.teacher?.teacherName} · {l.building?.buildingAddress}</div>{l.remoteLessonLink && /^https?:\/\//i.test(l.remoteLessonLink) && l.lessonStatus !== 'canceled' && <a href={l.remoteLessonLink} target="_blank" rel="noreferrer" className="link-btn">Подключиться</a>}</div><div><span className="room">{l.classroom?.classroomNumber ?? 'Аудитория не указана'}</span><Status value={l.lessonStatus}/></div></div>)}</div>{remote.data && !remote.data.lessons.length && <Empty />}</Remote></section>; }
function Grades({ user }: {
    user: User;
}) { const [offset, setOffset] = useState(0); const courses = useRemote<Page<Course>>('/academic/subjects?limit=100&offset=0', true); const remote = useRemote<Page<Grade>>(`/academic/grades?limit=20&offset=${offset}`); const action = useAction(); return <section className="card"><h3 className="card-title">Успеваемость</h3><Remote {...courses}><Remote {...remote}><div className="subject-table">{remote.data?.items.map(g => <div className="subject-row" key={g.courseGraduate}><div><div className="row-name">{courses.data?.items.find(c => c.courseID === g.courseID)?.title || g.courseID}</div>{user.role !== 'STUDENT' && <div className="row-sub">Студент: {g.studentID}</div>}</div><div className="subject-score">{g.score ?? 'Не выставлено'}<div className="mini-bar"><div className="mini-fill" style={{ width: `${g.score ?? 0}%` }}/></div></div><span className={`status ${g.credit ? 'mint' : 'gray'}`}>{g.credit == null ? 'Нет результата' : g.credit ? 'Зачтено' : 'Не зачтено'}</span>{user.role === 'TEACHER' && <form onSubmit={e => { e.preventDefault(); const value = new FormData(e.currentTarget).get('score'); void action.run(async () => { await api(`/academic/grades/${g.courseGraduate}`, 'PATCH', { score: Number(value) }); remote.reload(); }); }}><input aria-label="Баллы" name="score" type="number" min="0" max="100" required defaultValue={g.score ?? ''}/><button className="secondary" disabled={action.busy}>Сохранить</button></form>}</div>)}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote></Remote><ActionError error={action.error}/></section>; }
function Notifications({ go }: {
    go: (screen: string) => void;
}) { const [offset, setOffset] = useState(0); const remote = useRemote<Page<Notification>>(`/notifications?limit=20&offset=${offset}`); const action = useAction(); return <section className="card"><div className="card-head"><h3 className="card-title">Уведомления</h3><button className="secondary" onClick={remote.reload}>Обновить</button></div><Remote {...remote}><div className="list">{remote.data?.items.map(n => <button className={`notice ${!n.notificationReadAt ? 'unread' : ''}`} key={n.notificationID} disabled={action.busy} onClick={() => action.run(async () => { if (!n.notificationReadAt)
    await api(`/notifications/${n.notificationID}/read`, 'PATCH'); remote.reload(); if (n.appoinmentID)
    go('requests');
else if (n.lessonID)
    go('schedule');
else if (n.sportID)
    go('activities');
else if (n.courseID)
    go('learning');
else if (n.newsID)
    go('news'); })}><span className="notice-icon">●</span><span><div className="row-name">{n.notificationTitle}</div><div className="row-sub">{n.notificationMessage}</div></span><span className="notice-time">{date(n.notificationCreatedAt)}</span></button>)}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote><ActionError error={action.error}/></section>; }
function Sports() { const [offset, setOffset] = useState(0); const remote = useRemote<Page<Sport>>(`/sports?limit=20&offset=${offset}`); const registrations = useRemote<Page<Registration>>('/sports/registrations/me?limit=100&offset=0', true); const action = useAction(); return <div className="stack"><section className="activity-hero"><div><div className="label">Внеучебная активность</div><h2>Спортивные занятия</h2><p className="caption">Запись подтверждается сервером с учётом свободных мест.</p></div></section><Remote {...remote}><Remote {...registrations}><div className="course-grid">{remote.data?.items.map(s => { const enrolled = registrations.data?.items.some(r => r.sportID === s.sportID && r.registrationStatus === 'enrolled'); return <section className="course" key={s.sportID}><div className="course-title">{s.sportTitle}</div><div className="row-sub">{s.sportDescription}</div><div className="task">{date(s.sportStartAt)}<div className="caption">Вместимость: {s.sportCapacity} · записано: {s.enrolledCount ?? 'нет данных'}</div></div><button className={enrolled ? 'secondary' : 'primary'} style={{ marginTop: 14 }} disabled={action.busy || (!enrolled && s.enrolledCount !== undefined && s.enrolledCount >= s.sportCapacity)} onClick={() => action.run(async () => { await api(`/sports/${s.sportID}/registrations${enrolled ? '/me' : ''}`, enrolled ? 'DELETE' : 'POST'); remote.reload(); registrations.reload(); })}>{enrolled ? 'Отменить запись' : 'Записаться'}</button></section>; })}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote></Remote><ActionError error={action.error}/></div>; }
function Messages({ user }: {
    user: User;
}) { const [offset, setOffset] = useState(0); const remote = useRemote<Page<Conversation>>(`/messages/conversations?limit=20&offset=${offset}`); const [selected, setSelected] = useState<string | null>(null); const action = useAction(); return <div className="content-grid"><section className="card"><h3 className="card-title">Диалоги</h3><form onSubmit={e => { e.preventDefault(); const form = e.currentTarget; const participantAccountID = String(new FormData(form).get('participant')); void action.run(async () => { const c = await api<Conversation>('/messages/conversations', 'POST', { participantAccountID }); setSelected(c.conversationID); remote.reload(); form.reset(); }); }}><Field name="participant" label="Идентификатор аккаунта собеседника"/><button className="primary" disabled={action.busy}>Начать диалог</button></form><Remote {...remote}><div className="list" style={{ marginTop: 16 }}>{remote.data?.items.map(c => <button className="request" key={c.conversationID} onClick={() => setSelected(c.conversationID)}><span><span className="row-name">{c.participants.filter(p => p !== user.accountID).join(', ') || c.conversationID}</span><span className="row-sub">{date(c.updatedAt)}</span></span></button>)}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote><ActionError error={action.error}/></section>{selected ? <ConversationMessages key={selected} id={selected} user={user} changed={remote.reload}/> : <section className="card caption">Выберите диалог или создайте новый.</section>}</div>; }
function ConversationMessages({ id, user, changed }: {
    id: string;
    user: User;
    changed: () => void;
}) { const [offset, setOffset] = useState(0); const remote = useRemote<Page<Message>>(`/messages/conversations/${id}/messages?limit=20&offset=${offset}`); const action = useAction(); const [readError, setReadError] = useState(''); useEffect(() => { let live = true; api(`/messages/conversations/${id}/read`, 'POST').catch(error => { if (live)
    setReadError(errorText(error)); }); return () => { live = false; }; }, [id]); return <section className="card"><div className="card-head"><h3 className="card-title">Сообщения</h3><button className="secondary" onClick={remote.reload}>Обновить</button></div><Remote {...remote}><div className="list">{remote.data?.items.map(m => <div className="task" style={{ background: m.senderAccountID === user.accountID ? 'var(--violet-soft)' : undefined }} key={m.messageID}><div className="caption">{m.senderAccountID === user.accountID ? 'Вы' : m.senderAccountID} · {date(m.createdAt)}</div><p style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{m.text}</p>{m.senderAccountID === user.accountID && <div className="caption">{m.readAt ? 'Прочитано' : 'Отправлено'}</div>}</div>)}</div>{remote.data && !remote.data.items.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote><form onSubmit={e => { e.preventDefault(); const form = e.currentTarget; const text = String(new FormData(form).get('text')).trim(); if (text)
    void action.run(async () => { await api(`/messages/conversations/${id}/messages`, 'POST', { text }); form.reset(); setOffset(0); remote.reload(); changed(); }); }}><div className="field"><label htmlFor="message-text">Сообщение</label><textarea id="message-text" className="textarea" name="text" required/></div><button className="primary" disabled={action.busy}>Отправить</button></form><ActionError error={action.error || readError}/></section>; }
type ListItem = {
    id: string;
    title: string;
    description?: string;
    meta?: string;
};
function Directory({ path, kind }: {
    path: string;
    kind: string;
}) {
    const [offset, setOffset] = useState(0);
    const remote = useRemote<Page<Record<string, unknown>>>(`${path}?limit=20&offset=${offset}`);
    const [selected, setSelected] = useState<ListItem | null>(null);
    const rows: ListItem[] = remote.data?.items.map(x => {
        if (kind === 'courses')
            return { id: String(x.courseID), title: String(x.title), description: String(x.description || ''), meta: `${x.hours} ч. · семестр ${x.semester}` };
        if (kind === 'news')
            return { id: String(x.newsID), title: String(x.newsTitle), description: String(x.newsDescription || ''), meta: date(String(x.createdAt || '')) };
        if (kind === 'deadlines')
            return { id: String(x.deadlineID), title: String(x.deadlineTitle), description: String(x.deadlineDescription || ''), meta: date(x.deadlineEndAt as string) };
        if (kind === 'buildings')
            return { id: String(x.buildingID), title: String(x.building), description: String(x.buildingAddress || '') };
        return { id: String(x.studentID || x.teacherID), title: String(x.studentName || x.teacherName), description: String(x.studentEmail || x.teacherEmail || ''), meta: x.groupID ? `Группа ${x.groupID}` : String(x.teacherPosition || '') };
    }) || [];
    return <section className="card"><Remote {...remote}><div className={kind === 'courses' ? 'course-grid' : 'list'}>{rows.map(r => <button className={kind === 'courses' ? 'course' : 'request'} key={r.id} onClick={() => setSelected(r)}><span><span className="row-name">{r.title}</span><span className="row-sub" style={{ display: 'block' }}>{r.meta}</span></span><span className="link-btn">Подробнее →</span></button>)}</div>{!rows.length && <Empty />}<Pager page={remote.data} offset={offset} setOffset={setOffset}/></Remote>{selected && <div className="modal-bg"><section className="modal" role="dialog" aria-modal="true"><div className="card-head"><h2 className="card-title">{selected.title}</h2><button className="secondary" onClick={() => setSelected(null)}>Закрыть</button></div><div className="caption">{selected.meta}</div><p style={{ whiteSpace: 'pre-wrap' }}>{selected.description || 'Описание не указано'}</p></section></div>}</section>;
}
export function ConnectedScreen({ screen, user, logout, go }: {
    screen: string;
    user: User;
    logout: () => Promise<void>;
    go: (screen: string) => void;
}) {
    if (screen === 'users' || screen === 'roles')
        return <Accounts />;
    if (screen === 'profile')
        return <Profile user={user} logout={logout}/>;
    if (screen === 'requests' || screen === 'tickets')
        return <Requests user={user}/>;
    if (screen === 'schedule' || screen === 'online')
        return <Schedule />;
    if (screen === 'grades')
        return <Grades user={user}/>;
    if (screen === 'notifications')
        return <Notifications go={go}/>;
    if (screen === 'chat')
        return <Messages user={user}/>;
    if (screen === 'activities')
        return <Sports />;
    if (screen === 'learning' || screen === 'materials')
        return <Directory path="/courses" kind="courses"/>;
    if (screen === 'news' || screen === 'announcements')
        return <Directory path="/news" kind="news"/>;
    if (screen === 'deadlines')
        return <Directory path="/academic/deadlines" kind="deadlines"/>;
    if (screen === 'campus')
        return <Directory path="/buildings" kind="buildings"/>;
    if (screen === 'students')
        return <Directory path="/faculty/students" kind="students"/>;
    if (screen === 'today' || screen === 'overview')
        return <div className="content-grid"><div className="stack"><section className="hero"><div className="label">Цифровой деканат</div><h2 className="lesson-title">Ваш учебный кабинет</h2><p className="caption">Расписание, обращения и уведомления доступны в разделах кабинета.</p><div className="chips"><button className="primary" onClick={() => go('profile')}>Мой профиль</button><button className="secondary" onClick={() => go('notifications')}>Уведомления</button></div></section>{user.role !== 'ADMIN' && <Schedule />}</div><aside className="stack"><section className="card"><h3 className="card-title">Быстрые действия</h3><div className="quick-grid" style={{ marginTop: 16 }}>{[['requests', 'Заявки'], ['chat', 'Сообщения'], ['schedule', 'Расписание'], ['profile', 'Профиль']].filter(([s]) => user.role !== 'ADMIN' || s === 'profile' || s === 'chat').map(([s, label]) => <button className="quick" key={s} onClick={() => go(s)}><span className="quick-title">{label}</span></button>)}</div></section></aside></div>;
    return <section className="card"><h3 className="card-title">Раздел пока не подключён</h3><p className="caption">Для этого раздела нет подходящего сценария в текущем контракте API. Данные не загружены.</p><button className="secondary" onClick={() => go('profile')}>Открыть профиль</button></section>;
}
function Accounts() { const action = useAction(); const [result, setResult] = useState(''); const [role, setRole] = useState('STUDENT'); return <div className="content-grid"><form className="card" onSubmit={e => { e.preventDefault(); const form = e.currentTarget; const data = new FormData(form); void action.run(async () => { const key = role === 'STUDENT' ? 'studentID' : role === 'TEACHER' ? 'teacherID' : 'employerID'; const account = await api<{
    accountID: string;
    email: string;
}>('/admin/accounts', 'POST', { email: data.get('email'), password: data.get('password'), accountRole: role, [key]: data.get('personID') }); setResult(`Создан аккаунт ${account.email}: ${account.accountID}`); form.reset(); }); }}><h3 className="card-title">Создать учётную запись</h3><p className="caption">Список аккаунтов не предусмотрен контрактом. Создание доступно для существующего студента, преподавателя или сотрудника.</p><Field name="email" label="Электронная почта" type="email"/><div className="field"><label htmlFor="new-password">Пароль (не менее 12 символов)</label><input id="new-password" name="password" className="input" type="password" minLength={12} autoComplete="new-password" required/></div><div className="field"><label>Роль</label><select className="select" value={role} onChange={e => setRole(e.target.value)}>{['STUDENT', 'TEACHER', 'SUPPORT', 'DEAN', 'ADMIN'].map(r => <option key={r}>{r}</option>)}</select></div><Field name="personID" label="Идентификатор существующего человека"/><button className="primary" disabled={action.busy}>Создать</button></form><form className="card" onSubmit={e => { e.preventDefault(); const data = new FormData(e.currentTarget); const id = String(data.get('accountID')); void action.run(async () => { await api(`/admin/accounts/${encodeURIComponent(id)}`, 'PATCH', { isActive: data.get('isActive') === 'true', accountRole: data.get('role') || undefined }); setResult(`Аккаунт ${id} обновлён`); }); }}><h3 className="card-title">Изменить доступ</h3><Field name="accountID" label="Идентификатор аккаунта"/><div className="field"><label>Состояние</label><select name="isActive" className="select"><option value="true">Активен</option><option value="false">Отключён</option></select></div><div className="field"><label>Роль</label><select name="role" className="select"><option value="">Без изменения</option>{['STUDENT', 'TEACHER', 'SUPPORT', 'DEAN', 'ADMIN'].map(r => <option key={r}>{r}</option>)}</select></div><button className="primary" disabled={action.busy}>Сохранить</button><ActionError error={action.error}/>{result && <p role="status">{result}</p>}</form></div>; }
