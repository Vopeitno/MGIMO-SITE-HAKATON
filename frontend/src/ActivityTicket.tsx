import {useEffect,useState} from 'react';
import {download,response} from '../api';
import {studyError} from './study-format';

export function ActivityTicket({eventID,attended}: {eventID:string;attended?:boolean}) {
    const path='/activities/'+encodeURIComponent(eventID)+'/ticket';
    const [image,setImage]=useState('');
    const [error,setError]=useState('');
    const [busy,setBusy]=useState(false);
    const [attempt,setAttempt]=useState(0);

    useEffect(()=>{
        let live=true;
        let objectURL='';
        response(path).then(r=>r.blob()).then(blob=>{
            if (!live) return;
            objectURL=URL.createObjectURL(blob);
            setImage(objectURL);
        }).catch(err=>{if(live)setError(studyError(err));});
        return()=>{live=false;if(objectURL)URL.revokeObjectURL(objectURL);};
    },[path,attempt]);

    async function save(){
        setBusy(true);setError('');
        try{await download(path+'.pdf','Билет на мероприятие.pdf');}
        catch(err){setError(studyError(err));}
        finally{setBusy(false);}
    }

    return <div className="task">
        <div className="row-name">{attended?'Посещение подтверждено':'Билет на мероприятие'}</div>
        <p className="caption">Покажи QR-код ответственному на мероприятии.</p>
        {image?<img src={image} alt="QR-код билета" width={180} height={180}
            style={{display:'block',height:'auto',maxWidth:'100%',margin:'0 auto 10px'}}/>
            :!error&&<p className="caption" role="status">Загружаем QR-код…</p>}
        {error&&<p className="caption" role="alert">{error} {!image&&<button className="link-btn" onClick={()=>{setError('');setAttempt(x=>x+1)}}>Повторить</button>}</p>}
        <button type="button" className="secondary" disabled={busy} onClick={()=>void save()}>{busy?'Готовим билет…':'Скачать билет PDF'}</button>
    </div>;
}
