import {useEffect, useRef, useState} from 'react';
import type {FormEvent, ReactNode} from 'react';
import {api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {studyDate, studyError} from './study-format';

export type SettingsPanel = 'notifications' | 'security' | 'documents';
const categories = {
    schedule_change: 'Изменения расписания',
    request: 'Заявки',
    deadline: 'Дедлайны',
    exam: 'Экзамены',
    sport: 'Физкультура',
    announcement: 'Объявления',
    news: 'Новости',
};
const path = '/profile/me/settings';
const panelTitles = {notifications: 'Уведомления', security: 'Безопасность', documents: 'Документы и согласия'};

function useSettingsData<T>(endpoint: string) {
    const query=useCachedQuery('settings:'+endpoint,()=>api<T>(path+endpoint));
    return {data:query.data,error:query.error,reload:query.reload,setData:query.update};
}

function NotificationsSettings({openInbox}: {openInbox: () => void}) {
    const {data, setData, error, reload} = useSettingsData<{muted: string[]}>('/notifications');
    const [selected, setSelected] = useState<string[]>();
    const [message, setMessage] = useState('');
    const [failed, setFailed] = useState(false);
    const [busy, setBusy] = useState(false);
    const muted = selected ?? data?.muted ?? [];
    async function save() {
        setBusy(true);
        setMessage('');
        try {
            setData(await api<{muted:string[]}>(path + '/notifications', 'PUT', {muted}));
            setSelected(undefined);
            setFailed(false);
            setMessage('Настройки сохранены');
        } catch (err) {
            setFailed(true);
            setMessage(studyError(err));
        } finally {setBusy(false);}
    }
    return <>
        <p className="caption">Выбери, какие категории показывать в ленте уведомлений.</p>
        {!data ? <Loading error={error} reload={reload}/> : <fieldset disabled={busy} style={{padding: 0, margin: 0, border: 0}}>
            {Object.entries(categories).map(([key, label]) => <label key={key} className="setting" style={{justifyContent: 'flex-start'}}>
                <input type="checkbox" checked={!muted.includes(key)} onChange={event => {
                    setSelected(event.target.checked ? muted.filter(type => type !== key) : [...muted, key]);
                    setMessage('');
                }}/>
                <span className="row-name">{label}</span>
            </label>)}
        </fieldset>}
        {message && <p className="caption" role={failed ? 'alert' : 'status'}>{message}</p>}
        <div className="modal-actions">
            <button className="secondary" onClick={openInbox}>Открыть ленту</button>
            <button className="primary" disabled={!data || !selected || busy} onClick={() => void save()}>Сохранить</button>
        </div>
    </>;
}

type ActiveSession = {sessionID: string; createdAt: string; expiresAt: string; current: boolean};

function Loading({error, reload}: {error: string; reload: () => void}) {
    return <><p className="caption" role={error ? 'alert' : 'status'}>{error || 'Загружаем данные…'}</p>
        {error && <button className="secondary" onClick={reload}>Повторить</button>}</>;
}

function SecuritySettings() {
    const {data: sessions, setData, error, reload} = useSettingsData<ActiveSession[]>('/sessions');
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [failed, setFailed] = useState(false);

    async function changePassword(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const form = event.currentTarget;
        const fields = new FormData(form);
        const currentPassword = fields.get('currentPassword');
        const newPassword = fields.get('newPassword');
        if (newPassword !== fields.get('repeat')) {
            setFailed(true);
            setMessage('Новые пароли не совпадают');
            return;
        }
        setBusy(true);
        setMessage('');
        try {
            await api(path + '/password', 'PUT', {currentPassword, newPassword});
            form.reset();
            reload();
            setFailed(false);
            setMessage('Пароль изменён. Остальные сеансы завершены.');
        } catch (err) {
            setFailed(true);
            setMessage(studyError(err));
        } finally {setBusy(false);}
    }

    async function endSession(sessionID: string) {
        setBusy(true);
        setMessage('');
        try {
            await api(path + '/sessions/' + sessionID, 'DELETE');
            setData(items => (items??[]).filter(item => item.sessionID !== sessionID));
            setFailed(false);
            setMessage('Сеанс завершён');
        } catch (err) {
            setFailed(true);
            setMessage(studyError(err));
        } finally {setBusy(false);}
    }

    return <div className="stack">
        <form onSubmit={changePassword}>
            <h3 className="card-title">Смена пароля</h3>
            <label className="field">Текущий пароль<input className="input" name="currentPassword" autoComplete="current-password" type="password" maxLength={1024} required/></label>
            <label className="field">Новый пароль<input className="input" name="newPassword" autoComplete="new-password" type="password" minLength={12} maxLength={1024} required/></label>
            <label className="field">Повтор нового пароля<input className="input" name="repeat" autoComplete="new-password" type="password" minLength={12} maxLength={1024} required/></label>
            <p className="caption">Минимум 12 символов. После смены пароля другие сеансы завершатся.</p>
            <button className="primary" disabled={busy}>Изменить пароль</button>
        </form>
        {message && <p className="caption" role={failed ? 'alert' : 'status'}>{message}</p>}
        <div>
            <h3 className="card-title">Активные сеансы</h3>
            {!sessions ? <Loading error={error} reload={reload}/> : <div className="list">
                {sessions.map(s => <div className="request" key={s.sessionID}>
                    <div><div className="row-name">{s.current ? 'Текущий сеанс' : 'Другой сеанс'}</div>
                        <div className="row-sub">Вход {studyDate(s.createdAt)} · до {studyDate(s.expiresAt)} МСК</div></div>
                    {!s.current && <button className="secondary" disabled={busy} onClick={() => void endSession(s.sessionID)}>Завершить</button>}
                </div>)}
                {!sessions.length && <p className="caption">Активных сеансов нет</p>}
            </div>}
        </div>
    </div>;
}

type StudentDocument = {documentID: string; title: string; version: string; body: string; publishedAt: string;
    requiresConsent: boolean; accepted: boolean | null; changedAt: string | null};

function DocumentsSettings() {
    const {data, error, reload} = useSettingsData<StudentDocument[]>('/documents');
    const [busy, setBusy] = useState('');
    const [message, setMessage] = useState('');
    const [failed, setFailed] = useState(false);
    async function choose(document: StudentDocument, accepted: boolean) {
        setBusy(document.documentID);
        setMessage('');
        try {
            await api(path + '/documents/' + document.documentID, 'PUT', {accepted});
            reload();
            setFailed(false);
            setMessage(accepted ? 'Согласие сохранено' : 'Согласие отозвано');
        } catch (err) {
            setFailed(true);
            setMessage(studyError(err));
        } finally {setBusy('');}
    }
    return <>
        {!data ? <Loading error={error} reload={reload}/> : <div className="list">
            {data.map(doc => <details className="task" key={doc.documentID}>
                <summary className="row-name">{doc.title}</summary>
                <p className="caption">Редакция {doc.version} · {studyDate(doc.publishedAt)} МСК</p>
                <p className="caption" style={{whiteSpace: 'pre-wrap', overflowWrap: 'anywhere'}}>{doc.body}</p>
                {doc.requiresConsent && <>
                    <p className="caption">{doc.accepted === true ? 'Согласие дано' : doc.accepted === false ? 'Согласие отозвано' : 'Решение ещё не принято'}
                        {doc.changedAt && ` · ${studyDate(doc.changedAt)} МСК`}</p>
                    <button className="secondary" disabled={!!busy} onClick={() => void choose(doc, !doc.accepted)}>
                        {doc.accepted ? 'Отозвать согласие' : 'Дать согласие'}
                    </button>
                </>}
            </details>)}
            {!data.length && <p className="caption">Университет пока не опубликовал документы.</p>}
        </div>}
        {message && <p className="caption" role={failed ? 'alert' : 'status'}>{message}</p>}
    </>;
}

export function StudentSettings({panel, close, openInbox}: {panel: SettingsPanel; close: () => void; openInbox: () => void}) {
    const dialog = useRef<HTMLDialogElement>(null);
    useEffect(() => {dialog.current?.showModal();}, []);
    const content: Record<SettingsPanel, ReactNode> = {
        notifications: <NotificationsSettings openInbox={openInbox}/>,
        security: <SecuritySettings/>,
        documents: <DocumentsSettings/>,
    };
    return <dialog className="modal" ref={dialog} aria-labelledby="student-settings-title" onClose={close}
        style={{border: 0, maxWidth: 'calc(100vw - 32px)', maxHeight: 'calc(100dvh - 32px)', color: 'inherit'}}>
        <div className="card-head"><h2 className="card-title" id="student-settings-title">{panelTitles[panel]}</h2>
            <button className="icon-btn" aria-label="Закрыть" onClick={() => dialog.current?.close()}>×</button></div>
        {content[panel]}
    </dialog>;
}
