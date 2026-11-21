import {useEffect, useRef, useState} from 'react';
import {response} from '../api';
import {useCachedQuery} from './useCachedQuery';

async function imageURL(result: Response): Promise<string> {
    if (result.status === 204) return '';
    const blob = await result.blob();
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result));
        reader.onerror = () => reject(new Error('Не удалось прочитать изображение'));
        reader.readAsDataURL(blob);
    });
}

export function ProfileAvatar({initials}: {initials: string}) {
    const input = useRef<HTMLInputElement>(null);
    const query=useCachedQuery('profile-photo',()=>response('/profile/me/photo').then(imageURL),0,'Не удалось загрузить фото',0);
    const photo=query.data||'';
    const [uploading, setUploading] = useState(false);
    const [message, setMessage] = useState('');
    const [failed, setFailed] = useState(false);

    async function upload(file: File) {
        setMessage('');
        if (file.size > 2 * 1024 * 1024) {
            setFailed(true);
            setMessage('Фото должно быть не больше 2 МБ');
            return;
        }
        setUploading(true);
        const form = new FormData();
        form.set('file', file);
        try {
            query.update(await imageURL(await response('/profile/me/photo', {method: 'PUT', body: form})));
            setFailed(false);
            setMessage('Фото сохранено');
        } catch (err) {
            setFailed(true);
            setMessage(err instanceof Error ? err.message : 'Не удалось сохранить фото');
        } finally {
            setUploading(false);
        }
    }

    const label = photo ? 'Заменить фото профиля' : 'Добавить фото профиля';
    return <>
        <button type="button" className="profile-avatar" aria-label={label} title={label}
            disabled={(!query.ready && !query.error) || uploading} style={{border: 0, padding: 0, cursor: 'pointer'}}
            onClick={() => input.current?.click()}>
            {photo ? <img src={photo} alt=""/> : initials}
        </button>
        <input hidden ref={input} type="file" accept="image/jpeg,image/png"
            onChange={event => {
                const file = event.currentTarget.files?.[0];
                event.currentTarget.value = '';
                if (file) void upload(file);
            }}/>
        {uploading && <p className="caption" role="status">Сохраняем фото…</p>}
        {query.error&&!message&&<p className="caption" role="alert">{query.error}</p>}
        {message && <p className="caption" role={failed ? 'alert' : 'status'}>{message}</p>}
    </>;
}

export function ProfileQRDialog({close}: {close: () => void}) {
    const dialog = useRef<HTMLDialogElement>(null);
    const {data:qr,error}=useCachedQuery('profile-qr',()=>response('/profile/me/qr').then(imageURL),0,'Не удалось открыть QR-профиль',0);
    useEffect(()=>{dialog.current?.showModal();},[]);

    return <dialog className="modal" ref={dialog} aria-labelledby="profile-qr-title" onClose={close}
        style={{border: 0, maxWidth: 'calc(100vw - 32px)', color: 'inherit'}}>
        <div className="card-head">
            <h2 className="card-title" id="profile-qr-title">QR-профиль</h2>
            <button type="button" className="icon-btn" aria-label="Закрыть QR-профиль" onClick={()=>dialog.current?.close()}>×</button>
        </div>
        {qr ? <img src={qr} alt="QR-код с именем и номером студента" width={300} height={300}
            style={{display: 'block', maxWidth: '100%', height: 'auto', margin: '0 auto'}}/>
            : <p className="caption" role={error ? 'alert' : 'status'}>{error || 'Формируем QR-код…'}</p>}
        <p className="caption">В коде указаны ФИО и номер студента.</p>
    </dialog>;
}

export function ProfileQR({disabled}: {disabled: boolean}) {
    const [open, setOpen] = useState(false);
    return <>
        <button className="secondary" style={{width: '100%', marginTop: 10}}
            disabled={disabled} onClick={() => setOpen(true)}>Показать QR-профиль</button>
        {open && <ProfileQRDialog close={() => setOpen(false)}/>}
    </>;
}
