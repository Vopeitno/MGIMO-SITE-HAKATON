import {selectArrowCSS} from './select-arrow';
import {useRef, useState} from 'react';
import type {FormEvent, ReactNode} from 'react';
import {allPages, api} from '../api';
import {studyDate, studyError} from './study-format';
import {campusServiceTitles} from './campus-data';
import type {CampusService} from './campus-data';
import {useCachedQuery} from './useCachedQuery';

type Benefit = {id: string; title: string; description: string; url: string | null; editable: boolean};
type Report = {id: string; kind: 'lost' | 'found'; title: string; description: string; location: string;
    itemID: string | null; itemTitle: string | null; createdAt: string; resolvedAt: string | null; resolution: string | null; studentName: string | null; studentNumber: number | null};
type ResourceKind = 'room' | 'equipment' | 'printing';
type Resource = {id: string; kind: ResourceKind; title: string; location: string};
type Slot = Resource & {resourceID: string; startsAt: string; endsAt: string; reserved: boolean; upcoming: boolean;
    mine: boolean; studentName: string | null; studentNumber: number | null};
const resourceLabels: Record<ResourceKind, string> = {room: 'Помещение', equipment: 'Техника', printing: 'Печать'};

function useServiceList<T>(path: string | null) {
    return useCachedQuery(path, async () => (await allPages<T>(path!)).items);
}

function ServiceForm({children, label, save, reset = true}: {
    children: ReactNode; label: string; save: (data: FormData) => Promise<unknown>; reset?: boolean;
}) {
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [error, setError] = useState('');
    const pending = useRef(false);
    async function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (pending.current) return;
        pending.current = true;
        setBusy(true);
        setMessage('');
        setError('');
        const form = event.currentTarget;
        try {
            await save(new FormData(form));
            if (reset) form.reset();
            setMessage('Сохранено');
        } catch (err) {
            setError(studyError(err));
        } finally {
            pending.current = false;
            setBusy(false);
        }
    }
    return <form className="campus-service-form" onSubmit={submit}>
        <fieldset disabled={busy}>
            {children}
            <button className="primary role-primary" type="submit">{busy ? 'Сохраняем…' : label}</button>
        </fieldset>
        {error && <p className="caption role-sub" role="alert">{error}</p>}
        {message && <p className="caption role-sub" role="status">{message}</p>}
    </form>;
}

function ServiceAction({label, run}: {label: string; run: () => Promise<unknown>}) {
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    async function perform() {
        setBusy(true);
        setError('');
        try { await run(); }
        catch (err) { setError(studyError(err)); }
        finally { setBusy(false); }
    }
    return <div>
        <button type="button" className="secondary role-primary" disabled={busy} onClick={() => void perform()}>
            {busy ? 'Сохраняем…' : label}
        </button>
        {error && <p className="caption role-sub" role="alert">{error}</p>}
    </div>;
}

function ListStatus({data, error, empty}: {data: unknown[] | null; error: string; empty: string}) {
    if (error) return <p className="caption role-sub" role="alert">{error}</p>;
    if (data?.length) return null;
    return <p className="caption role-sub" role="status">{data ? empty : 'Загружаем данные…'}</p>;
}

function field(data: FormData, name: string): string {
    return String(data.get(name) ?? '').trim();
}

function Benefits({manage}: {manage: boolean}) {
    const {data, error, reload} = useServiceList<Benefit>('/campus/benefits');
    const [editing, setEditing] = useState<Benefit | null>(null);
    const path = editing ? '/campus/benefits/' + editing.id : '/campus/benefits';
    return <div className="campus-service-list">
        {manage && <ServiceForm key={editing?.id ?? 'new'} label={editing ? 'Сохранить изменения' : 'Опубликовать льготу'} save={async form => {
            await api(path, editing ? 'PUT' : 'POST', {title: field(form, 'title'), description: field(form, 'description'), url: field(form, 'url')});
            await reload();
            setEditing(null);
        }}>
            <label>Название<input className="input" name="title" maxLength={120} defaultValue={editing?.title} required/></label>
            <label>Условия получения<textarea className="input" name="description" maxLength={2000} defaultValue={editing?.description} required/></label>
            <label>Ссылка на подробности<input className="input" type="url" name="url" maxLength={1000} defaultValue={editing?.url ?? ''}/></label>
            {editing && <button type="button" className="secondary role-primary" onClick={() => setEditing(null)}>Отменить редактирование</button>}
        </ServiceForm>}
        <ListStatus data={data} error={error} empty="Опубликованных льгот пока нет."/>
        {data?.map(item => <article className="campus-service-entry" key={item.id}>
            <strong>{item.title}</strong>
            <p className="campus-service-text">{item.description}</p>
            {item.url && <a href={item.url} target="_blank" rel="noopener noreferrer">Условия на сайте организатора</a>}
            {manage && item.editable && <div className="campus-service-actions">
                <button className="secondary role-primary" type="button" onClick={() => setEditing(item)}>Редактировать</button>
                <ServiceAction label="Снять с публикации" run={async () => {
                    await api('/campus/benefits/' + item.id, 'DELETE');
                    if (editing?.id === item.id) setEditing(null);
                    await reload();
                }}/>
            </div>}
        </article>)}
    </div>;
}

type LostItem = {id: string; kind: 'lost' | 'found'; title: string; location: string; publishedAt: string};

function LostReports({manage}: {manage: boolean}) {
    const {data, error, reload} = useServiceList<Report>('/campus/lost-reports');
    const items = useServiceList<LostItem>('/campus/lost-items');
    const [linked, setLinked] = useState<LostItem | null>(null);
    const [itemQuery, setItemQuery] = useState('');
    const formAnchor = useRef<HTMLDivElement>(null);
    const publicItems = items.data?.filter(item => (item.title+' '+item.location).toLowerCase().includes(itemQuery.trim().toLowerCase())) ?? [];
    const [query, setQuery] = useState('');
    const visible = data?.filter(item => [item.title, item.location, item.description, item.studentName, item.studentNumber]
        .join(' ').toLowerCase().includes(query.trim().toLowerCase())) ?? [];
    return <div className="campus-service-list">
        <strong>Потерянные и найденные вещи</strong>
        {manage && <ServiceForm label="Опубликовать вещь" save={async form => {
            await api('/campus/lost-items', 'POST', {kind: field(form,'kind'),title: field(form,'title'),location: field(form,'location')});
            await items.reload();
        }}>
            <label>Тип карточки<select className="input spaced-select" name="kind"><option value="found">Находка</option><option value="lost">Пропажа</option></select></label>
            <label>Название для списка<input className="input" name="title" maxLength={120} required/></label>
            <label>Место<input className="input" name="location" maxLength={180} required/></label>
            <p className="caption role-sub">Карточку видят студенты. Укажите только общие сведения о вещи и месте.</p>
        </ServiceForm>}
        {!!items.data?.length && <input className="input" aria-label="Поиск по вещам" placeholder="Найти вещь" value={itemQuery} onChange={event=>setItemQuery(event.target.value)}/>}
        <ListStatus data={items.data?publicItems:null} error={items.error} empty={itemQuery?'Подходящих вещей нет.':'Сотрудники бюро пока не опубликовали карточки.'}/>
        {publicItems.map(item=>manage?<article className="campus-service-entry" key={item.id}>
            <strong>{item.kind==='lost'?'Пропажа':'Находка'} · {item.title}</strong>
            <span className="caption role-sub">{item.location} · {studyDate(item.publishedAt)}</span>
            <ServiceAction label="Убрать из списка" run={async()=>{await api('/campus/lost-items/'+item.id,'DELETE');await items.reload()}}/>
        </article>:<button type="button" className="request" key={item.id} aria-pressed={linked?.id===item.id}
            style={{width:'100%',textAlign:'left'}} onClick={()=>{setLinked(item);formAnchor.current?.scrollIntoView({block:'start',behavior:'smooth'})}}>
            <span><strong>{item.kind==='lost'?'Пропажа':'Находка'} · {item.title}</strong>
                <span className="row-sub" style={{display:'block'}}>{item.location} · {studyDate(item.publishedAt)}</span></span>
            <span className="status blue">Обратиться</span>
        </button>)}
        {!manage && <div ref={formAnchor} className="campus-service-list">
        {linked&&<div className="campus-service-actions"><strong>Обращение по вещи «{linked.title}»</strong>
            <button type="button" className="secondary" onClick={()=>setLinked(null)}>Убрать привязку</button></div>}
        <ServiceForm key={linked?.id??'unlinked'} label="Отправить в бюро находок" save={async form => {
            await api('/campus/lost-reports', 'POST', {
                kind: field(form, 'kind'), title: field(form, 'title'), description: field(form, 'description'), location: field(form, 'location'),
                itemID: linked?.id,
            });
            await reload();
        }}>
            <label>Что произошло<select className="input spaced-select" name="kind" defaultValue={linked?.kind==='lost'?'found':'lost'}><option value="lost">Потерял вещь</option><option value="found">Нашёл вещь</option></select></label>
            <label>Предмет<input className="input" name="title" maxLength={120} defaultValue={linked?.title} required/></label>
            <label>Где это произошло<input className="input" name="location" maxLength={180} defaultValue={linked?.location} required/></label>
            <label>Описание<textarea className="input" name="description" maxLength={2000} required/></label>
        </ServiceForm></div>}
        <strong>{manage ? 'Сообщения студентов' : 'Мои сообщения'}</strong>
        {data && data.length > 0 && <input className="input" aria-label="Поиск по сообщениям" placeholder="Найти сообщение"
            value={query} onChange={event => setQuery(event.target.value)}/>}
        <ListStatus data={data} error={error} empty="Сообщений пока нет."/>
        {Boolean(data?.length) && !visible.length && <p className="caption role-sub">По запросу ничего не найдено.</p>}
        {visible.map(item => <article className="campus-service-entry" key={item.id}>
            <strong>{item.kind === 'lost' ? 'Пропажа' : 'Находка'} · {item.title}</strong>
            <div className="caption role-sub">{item.location} · {studyDate(item.createdAt)} · {item.resolvedAt ? 'Закрыто' : 'Ожидает ответа'}</div>
            {manage && <div className="caption role-sub">{item.studentName} · № {item.studentNumber}</div>}
            {item.itemTitle&&<div className="caption role-sub">Карточка вещи · {item.itemTitle}</div>}
            <p className="campus-service-text">{item.description}</p>
            {item.resolution && <div><strong>Ответ поддержки</strong><p className="campus-service-text">{item.resolution}</p></div>}
            {manage && !item.resolvedAt && <ServiceForm label="Ответить и закрыть" save={async form => {
                await api('/campus/lost-reports/' + item.id + '/resolution', 'PUT', {resolution: field(form, 'resolution')});
                await reload();
            }}><label>Ответ студенту<textarea className="input" name="resolution" maxLength={2000} required/></label></ServiceForm>}
        </article>)}
    </div>;
}

function MoscowSlotTime({slot}: {slot: Slot}) {
    const format = new Intl.DateTimeFormat('ru-RU', {
        timeZone: 'Europe/Moscow', day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit',
    });
    return <>{format.formatRange(new Date(slot.startsAt), new Date(slot.endsAt))} мск</>;
}

function Bookings({manage}: {manage: boolean}) {
    const {data, error, reload} = useServiceList<Slot>('/campus/resource-slots');
    const resources = useServiceList<Resource>(manage ? '/campus/resources' : null);
    const [onlyMine, setOnlyMine] = useState(false);
    const [kind, setKind] = useState('');
    const visible = data?.filter(slot => (!kind || slot.kind === kind) && (manage || (onlyMine ? slot.mine : !slot.reserved))) ?? [];
    return <div className="campus-service-list">
        {manage && <>
            <ServiceForm label="Добавить ресурс" save={async form => {
                await api('/campus/resources', 'POST', {kind: field(form, 'kind'), title: field(form, 'title'), location: field(form, 'location')});
                await resources.reload();
            }}>
                <label>Тип ресурса<select className="input spaced-select" name="kind">{Object.entries(resourceLabels).map(([code, name]) =>
                    <option key={code} value={code}>{name}</option>)}</select></label>
                <label>Название<input className="input" name="title" maxLength={120} required/></label>
                <label>Место<input className="input" name="location" maxLength={180} required/></label>
            </ServiceForm>
            <ListStatus data={resources.data} error={resources.error} empty="Добавьте ресурс, чтобы публиковать время бронирования."/>
            {!!resources.data?.length && <ServiceForm label="Опубликовать время" save={async form => {
                await api('/campus/resource-slots', 'POST', {
                    resourceID: field(form, 'resource'), startsAt: field(form, 'start') + ':00+03:00', endsAt: field(form, 'end') + ':00+03:00',
                });
                await reload();
            }}>
                <label>Ресурс<select className="input spaced-select" name="resource" required defaultValue=""><option value="" disabled>Выберите ресурс</option>
                    {resources.data.map(resource => <option key={resource.id} value={resource.id}>{resource.title} · {resource.location}</option>)}
                </select></label>
                <label>Начало по Москве<input className="input" name="start" type="datetime-local" required/></label>
                <label>Окончание по Москве<input className="input" name="end" type="datetime-local" required/></label>
            </ServiceForm>}
        </>}
        <div className="campus-service-actions">
            {!manage && <><button className={onlyMine ? 'secondary' : 'primary'} type="button" aria-pressed={!onlyMine} onClick={() => setOnlyMine(false)}>Доступное время</button>
                <button className={onlyMine ? 'primary' : 'secondary'} type="button" aria-pressed={onlyMine} onClick={() => setOnlyMine(true)}>Мои брони</button></>}
            <select className="input spaced-select" aria-label="Тип ресурса" value={kind} onChange={event => setKind(event.target.value)}>
                <option value="">Все ресурсы</option>{Object.entries(resourceLabels).map(([code, name]) => <option key={code} value={code}>{name}</option>)}
            </select>
        </div>
        <ListStatus data={data ? visible : null} error={error} empty={onlyMine ? 'У вас пока нет активных броней.' : 'Подходящих слотов пока нет.'}/>
        {visible.map(slot => {
            const path = '/campus/resource-slots/' + slot.id;
            return <article className="campus-service-entry" key={slot.id}>
                <strong>{slot.title}</strong>
                <div className="caption role-sub">{resourceLabels[slot.kind]} · {slot.location}</div>
                <div><MoscowSlotTime slot={slot}/></div>
                {manage && slot.reserved && <div className="role-sub">Забронировано · {slot.studentName} · № {slot.studentNumber}</div>}
                {!manage && slot.mine && <span className="status mint">Моя бронь</span>}
                {slot.upcoming && (!manage || !slot.reserved) && <ServiceAction
                    label={manage ? 'Снять слот' : slot.mine ? 'Отменить бронь' : 'Забронировать'}
                    run={async () => {
                        await api(manage ? path : path + '/reservation', manage || slot.mine ? 'DELETE' : 'PUT');
                        await reload();
                    }}/>}
            </article>;
        })}
    </div>;
}

export function CampusServiceContent({service, manage = false}: {service: CampusService; manage?: boolean}) {
    return <div className="campus-service-content">
        <style>{selectArrowCSS+`
            .campus-service-content .campus-service-list,.campus-service-content fieldset{display:grid;gap:12px}
            .campus-service-content fieldset{border:0;padding:0;margin:0;min-width:0}
            .campus-service-content label{display:grid;gap:4px}
            .campus-service-content .input{width:100%;min-width:0}
            .role-app .campus-service-content .input{padding:10px 12px;border:1px solid var(--line);border-radius:12px;font:inherit;background:white;color:inherit}
            .role-app .campus-service-content select.spaced-select.input{padding-right:44px}
            .campus-service-content textarea{min-height:80px;resize:vertical}
            .campus-service-entry{border:1px solid var(--line);border-radius:14px;padding:12px;display:grid;justify-items:start;gap:6px;overflow-wrap:anywhere}
            .campus-service-text{white-space:pre-wrap;margin:0}
            .campus-service-actions{display:flex;flex-wrap:wrap;align-items:center;gap:8px}
            .campus-service-actions select{flex:1 1 145px}
        `}</style>
        {service === 'benefits' ? <Benefits manage={manage}/> : service === 'lost-reports' ? <LostReports manage={manage}/> : <Bookings manage={manage}/>}
    </div>;
}

export function StaffCampusServices({dean}: {dean: boolean}) {
    const [service, setService] = useState<CampusService>(dean ? 'benefits' : 'lost-reports');
    return <section className="role-card">
        <h2 className="role-name">{dean ? campusServiceTitles.benefits : 'Сервисы кампуса'}</h2>
        {!dean && <div className="campus-service-actions" style={{marginBottom: 12, display: 'flex', gap: 8, flexWrap: 'wrap'}}>
            {(['lost-reports', 'resource-slots'] as const).map(key =>
                <button key={key} className="role-primary" type="button" aria-pressed={key === service}
                    onClick={() => setService(key)}>{key === 'resource-slots' ? 'Ресурсы и бронирования' : campusServiceTitles[key]}</button>)}
        </div>}
        <CampusServiceContent key={service} service={service} manage/>
    </section>;
}
