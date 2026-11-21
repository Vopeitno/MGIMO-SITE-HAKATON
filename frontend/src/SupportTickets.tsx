import {useRef, useState} from 'react';
import type {FormEvent} from 'react';
import {allPages, api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {requestDate} from './requests-model';
import {studyError} from './study-format';
import {ticketCategories, useSupportArticles} from './support-data';
import type {SupportArticle, Ticket, TicketThread} from './support-data';

const fieldStyle = {display: 'block', width: '100%', minWidth: 0, font: 'inherit', padding: '10px 12px',
    border: '1px solid var(--line)', borderRadius: 12, background: '#fff', color: 'inherit'};
const formStyle = {display: 'grid', gap: 10};
const detailStyle = {display: 'grid', gap: 12, minWidth: 0, overflowWrap: 'anywhere' as const};

function ticketStatus(ticket: Ticket) {
    return ticket.closedAt ? 'Закрыто' : ticket.assigned ? 'В работе' : 'Ожидает ответа';
}

function TicketMessages({ticket, manage, afterReply}: {ticket: TicketThread; manage: boolean; afterReply: () => Promise<void>}) {
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const sending = useRef(false);
    async function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (sending.current) return;
        const form = event.currentTarget;
        const values = new FormData(form);
        const message = String(values.get('message') ?? '').trim();
        if (!message) {
            setError('Введите сообщение.');
            return;
        }
        sending.current = true;
        setBusy(true);
        setError('');
        try {
            await api('/support/tickets/' + ticket.id + '/messages', 'POST', {message, close: values.has('close')});
            form.reset();
            await afterReply();
        } catch (err) {
            setError(studyError(err));
            await afterReply();
        } finally {
            sending.current = false;
            setBusy(false);
        }
    }
    return <div style={detailStyle}>
        <div className="card-head">
            <strong>{ticketCategories[ticket.category]}</strong><span className="status gray">{ticketStatus(ticket)}</span>
        </div>
        {ticket.messages.map(entry => <div key={entry.id} style={{padding: 12, background: 'var(--bg)', borderRadius: 12}}>
            <div className="caption role-sub">{entry.fromStudent ? 'Студент' : 'Поддержка'} · {requestDate(entry.createdAt)}</div>
            <div style={{whiteSpace: 'pre-wrap'}}>{entry.message}</div>
        </div>)}
        {manage && !ticket.closedAt && ticket.assigned && !ticket.assignedToMe &&
            <p className="role-sub">Обращение ведёт другой сотрудник.</p>}
        {!ticket.closedAt && (!manage || !ticket.assigned || ticket.assignedToMe) &&
            <form onSubmit={submit} style={formStyle}>
                <label>{manage ? 'Ответ студенту' : 'Сообщение'}
                    <textarea name="message" style={{...fieldStyle, minHeight: 75}} maxLength={5000} required/>
                </label>
                {manage && <label><input name="close" type="checkbox"/> Закрыть обращение после ответа</label>}
                <button type="submit" className="primary role-primary" disabled={busy}>{busy ? 'Отправляем…' : 'Отправить'}</button>
                {error && <p className="caption role-sub" role="alert">{error}</p>}
            </form>}
    </div>;
}

export function SupportTickets({manage = false, initialID = ''}: {manage?: boolean; initialID?: string}) {
    const [selected, setSelected] = useState(initialID);
    const [query, setQuery] = useState('');
    const [status, setStatus] = useState('active');
    const list = useCachedQuery('support-tickets', () => allPages<Ticket>('/support/tickets').then(page => page.items));
    const detailPath = selected ? '/support/tickets/' + selected : null;
    const thread = useCachedQuery(detailPath, () => api<TicketThread>(detailPath!));
    const entries = (list.data ?? []).filter(ticket => !manage ||
        (status === 'all' || (status === 'closed' ? ticket.closedAt : !ticket.closedAt)) &&
        [ticketCategories[ticket.category], ticket.studentName, ticket.studentNumber].join(' ').toLowerCase().includes(query.trim().toLowerCase()));
    return <section className={manage ? 'role-card' : 'card'}>
        <div className="card-head"><h2 className="card-title role-name">{manage ? 'Очередь обращений' : 'Мои обращения'}</h2></div>
        {manage && <div style={{display: 'flex', gap: 10, marginBottom: 12, flexWrap: 'wrap'}}>
            <input placeholder="Тема, имя или номер студента" aria-label="Поиск по обращениям"
                style={{...fieldStyle, flex: '1 1 220px'}} value={query} onChange={event => setQuery(event.target.value)}/>
            <select aria-label="Статус обращения" style={{...fieldStyle, flex: '1 1 130px'}} value={status} onChange={event => setStatus(event.target.value)}>
                <option value="active">Открытые</option><option value="closed">Закрытые</option><option value="all">Все</option>
            </select>
        </div>}
        <div style={detailStyle}>
            {list.error && <p className="caption role-sub" role="alert">{list.error}</p>}
            {!list.error && !entries.length && <p className="caption role-sub">{list.data ? 'Обращений пока нет.' : 'Загружаем обращения…'}</p>}
            {entries.map(ticket => <button type="button" key={ticket.id} className="setting role-row"
                style={{width: '100%', textAlign: 'left'}} aria-expanded={selected === ticket.id}
                onClick={() => setSelected(selected === ticket.id ? '' : ticket.id)}>
                <span><strong>{ticketCategories[ticket.category]}</strong>
                    <div className="row-sub role-sub">{requestDate(ticket.updatedAt)}{manage && <> · {ticket.studentName} · № {ticket.studentNumber}</>}
                        {ticket.priority === 'urgent' && ' · Срочное'}
                    </div>
                </span>
                <span className="status gray">{ticketStatus(ticket)}</span>
            </button>)}
            {selected && (thread.data ? <TicketMessages key={selected} ticket={thread.data} manage={manage}
                afterReply={async () => {await Promise.all([list.reload(), thread.reload()]);}}/>
                : <p className="caption role-sub" role={thread.error ? 'alert' : 'status'}>{thread.error || 'Загружаем переписку…'}</p>)}
        </div>
    </section>;
}

export function SupportKnowledgeEditor() {
    const articles = useSupportArticles();
    const [editing, setEditing] = useState<SupportArticle | null>(null);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [saved, setSaved] = useState('');
    async function change(action: () => Promise<unknown>, message: string) {
        if (busy) return;
        setBusy(true);
        setError('');
        setSaved('');
        try {
            await action();
            await articles.reload();
            setSaved(message);
        } catch (err) {
            setError(studyError(err));
        } finally {
            setBusy(false);
        }
    }
    function submit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const form = event.currentTarget;
        const fields = new FormData(form);
        void change(async () => {
            await api(editing ? '/support/articles/' + editing.id : '/support/articles', editing ? 'PUT' : 'POST',
                {question: fields.get('question'), answer: fields.get('answer')});
            form.reset();
            setEditing(null);
        }, 'Статья сохранена.');
    }
    return <section className="role-card">
        <h2 className="role-name">{editing ? 'Редактировать статью' : 'Новая статья'}</h2>
        <form key={editing?.id ?? 'new'} onSubmit={submit} style={formStyle}>
            <label>Вопрос<input name="question" required maxLength={180} defaultValue={editing?.question} style={fieldStyle}/></label>
            <label>Ответ<textarea name="answer" required maxLength={5000} defaultValue={editing?.answer} style={{...fieldStyle, minHeight: 80}}/></label>
            <button className="role-primary" disabled={busy} type="submit">{busy ? 'Сохраняем…' : 'Сохранить статью'}</button>
            {editing && <button className="role-primary" type="button" onClick={() => setEditing(null)}>Отменить редактирование</button>}
        </form>
        {(error || articles.error) && <p className="role-sub" role="alert">{error || articles.error}</p>}
        {saved && <p className="role-sub" role="status">{saved}</p>}
        <div style={{...detailStyle, marginTop: 16}}>
            {articles.data?.map(item => <div key={item.id} style={{border: '1px solid var(--line)', borderRadius: 12, padding: 12}}>
                <strong>{item.question}</strong><p style={{whiteSpace: 'pre-wrap'}}>{item.answer}</p>
                <div style={{display: 'flex', gap: 8, flexWrap: 'wrap'}}>
                    <button className="role-primary" type="button" disabled={busy} onClick={() => setEditing(item)}>Редактировать</button>
                    <button className="role-primary" type="button" disabled={busy} onClick={() => void change(async () => {
                        await api('/support/articles/' + item.id, 'DELETE');
                        if (editing?.id === item.id) setEditing(null);
                    }, 'Статья снята с публикации.')}>Снять с публикации</button>
                </div>
            </div>)}
            {articles.data?.length === 0 && <p className="role-sub">Опубликованных статей пока нет.</p>}
        </div>
    </section>;
}
