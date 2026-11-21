import { useEffect, useRef, useState } from 'react';
import { allPages, api, response } from '../api';

type Draft = { appointmentDraftID: string; draftStatus: string; appointmentTypeID?: string | null; appointmentTitle?: string | null; appointmentDescription?: string | null };
type Values = { appointmentTypeID: string; appointmentTitle: string; appointmentDescription: string };
const blank: Values = { appointmentTypeID: '', appointmentTitle: '', appointmentDescription: '' };

export default function RequestDraftForm({ close, created }: { close: () => void; created: (id: string) => void }) {
  const [values, setValues] = useState<Values>(blank);
  const [types, setTypes] = useState<{ appointmentTypeID: string; appointmentTypeTitle: string; isActive: boolean }[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState('');
  const id = useRef<string | null>(null);
  const latest = useRef<Values>(blank);
  const live = useRef(true);
  const dirty = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const chain = useRef<Promise<void>>(Promise.resolve());

  function body(snapshot: Values) { return { ...snapshot, appointmentTypeID: snapshot.appointmentTypeID || null }; }
  function save(snapshot: Values): Promise<void> {
    const operation = chain.current.catch(() => {}).then(async () => {
      const draft = await api<Draft>(id.current ? `/requests/drafts/${id.current}` : '/requests/drafts', id.current ? 'PATCH' : 'POST', body(snapshot));
      id.current = draft.appointmentDraftID;
      if (snapshot === latest.current) dirty.current = false;
      if (live.current) { setSaved('Черновик сохранён'); setError(''); }
    });
    chain.current = operation;
    return operation;
  }

  useEffect(() => {
    live.current = true;
    Promise.all([
      allPages<{ appointmentTypeID: string; appointmentTypeTitle: string; isActive: boolean }>('/requests/types'),
      allPages<Draft>('/requests/drafts'),
    ]).then(([catalog, drafts]) => {
      if (!live.current) return;
      setTypes(catalog.items);
      const draft = drafts.items.find(d => d.draftStatus === 'editing');
      if (draft) {
        id.current = draft.appointmentDraftID;
        latest.current = { appointmentTypeID: draft.appointmentTypeID || '', appointmentTitle: draft.appointmentTitle || '', appointmentDescription: draft.appointmentDescription || '' };
        setValues(latest.current);
        setSaved('Восстановлен сохранённый черновик');
      }
    }).catch(e => { if (live.current) setError(e instanceof Error ? e.message : 'Не удалось загрузить черновик'); })
      .finally(() => { if (live.current) setLoading(false); });
    function pagehide() {
      if (dirty.current && id.current) {
        void response(`/requests/drafts/${id.current}`, { method: 'PATCH', keepalive: true, body: JSON.stringify(body(latest.current)) }, false).catch(() => {});
      }
    }
    window.addEventListener('pagehide', pagehide);
    return () => { live.current = false; if (timer.current) clearTimeout(timer.current); window.removeEventListener('pagehide', pagehide); };
  }, []);

  function change(field: keyof Values, value: string) {
    const snapshot = { ...latest.current, [field]: value };
    latest.current = snapshot; dirty.current = true; setValues(snapshot); setSaved('Сохраняем черновик…');
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => { void save(snapshot).catch(e => { if (live.current) { setSaved(''); setError(e instanceof Error ? e.message : 'Не удалось сохранить черновик'); } }); }, 500);
  }

  async function finish(submit: boolean) {
    setBusy(true); setError('');
    if (timer.current) clearTimeout(timer.current);
    try {
      if (dirty.current || !id.current) await save(latest.current); else await chain.current;
      if (submit) {
        const result = await api<{ appoinmentID: string }>(`/requests/drafts/${id.current}/submit`, 'POST');
        dirty.current = false; created(result.appoinmentID);
      } else close();
    } catch (e) { setError(e instanceof Error ? e.message : 'Не удалось сохранить заявку'); }
    finally { if (live.current) setBusy(false); }
  }

  return <div className="modal-bg"><form className="modal" role="dialog" aria-modal="true" aria-label="Новая заявка" onSubmit={e => { e.preventDefault(); void finish(true); }}>
    <div className="card-head"><h2 className="card-title">Новая заявка</h2><button type="button" className="secondary" disabled={busy || loading} onClick={() => void finish(false)}>Сохранить и закрыть</button></div>
    {loading ? <p className="caption" role="status">Загружаем черновик…</p> : <>
      <div className="field"><label htmlFor="request-type">Тип обращения</label><select id="request-type" className="select" required value={values.appointmentTypeID} onChange={e => change('appointmentTypeID', e.target.value)}><option value="">Выберите тип</option>{types.filter(t => t.isActive).map(t => <option key={t.appointmentTypeID} value={t.appointmentTypeID}>{t.appointmentTypeTitle}</option>)}</select></div>
      <div className="field"><label htmlFor="request-title">Заголовок</label><input id="request-title" className="input" required maxLength={100} value={values.appointmentTitle} onChange={e => change('appointmentTitle', e.target.value)}/></div>
      <div className="field"><label htmlFor="request-description">Описание</label><textarea id="request-description" className="textarea" required value={values.appointmentDescription} onChange={e => change('appointmentDescription', e.target.value)}/></div>
      <p className="caption" role="status">{saved}</p><p className="caption">Вложение можно загрузить в карточке после отправки.</p>
    </>}
    {error && <p role="alert" className="caption">{error}</p>}
    <div className="modal-actions"><button className="primary" disabled={busy || loading || !types.length}>{busy ? 'Сохраняем…' : 'Отправить заявку'}</button></div>
  </form></div>;
}
