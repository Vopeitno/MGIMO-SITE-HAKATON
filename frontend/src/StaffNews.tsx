import {useState} from 'react';
import type {FormEvent} from 'react';
import {allPages,api} from '../api';
import {newsLabels} from './news-data';
import type {NewsArticle} from './news-data';
import {studyDate,studyError} from './study-format';
import {useCachedQuery} from './useCachedQuery';

type Deanery={deaneryID:number;name:string};
const input={display:'block',width:'100%',marginTop:5,padding:'10px 12px',font:'inherit',
    border:'1px solid var(--line)',borderRadius:12,background:'#fff',color:'inherit'};

function NewsEditor({article,deaneries,requireDeanery,onSaved}: {
    article?:NewsArticle;deaneries:Deanery[];requireDeanery:boolean;onSaved:(article:NewsArticle)=>void
}) {
    const [busy,setBusy]=useState(false);
    const [error,setError]=useState('');
    const [message,setMessage]=useState('');

    async function save(event:FormEvent<HTMLFormElement>){
        event.preventDefault();
        if(busy)return;
        const form=event.currentTarget;
        const fields=new FormData(form);
        const newsTitle=String(fields.get('title')??'').trim();
        const newsDescription=String(fields.get('body')??'').trim();
        const newsCategory=String(fields.get('category')??'')||null;
        if(!newsTitle||!newsDescription){setError('Заполните заголовок и текст');return}
        const deaneryID=!article&&requireDeanery?Number(fields.get('deanery')):undefined;
        if(!article&&requireDeanery&&!deaneryID){setError('Выберите деканат');return}
        setBusy(true);setError('');setMessage('');
        try{
            const result=await api<NewsArticle>(article?`/news/${article.newsID}`:'/news',article?'PATCH':'POST',
                {newsTitle,newsDescription,newsCategory,deaneryID});
            onSaved(result);
            setMessage(article?'Публикация обновлена':'Новость опубликована');
            if(!article)form.reset();
        }catch(err){setError(studyError(err))}
        finally{setBusy(false)}
    }

    return <form className="role-row" style={{display:'grid',gap:12,cursor:'default'}} onSubmit={save}>
        <strong className="role-name">{article?'Редактирование публикации':'Новая публикация'}</strong>
        {!article&&requireDeanery&&<label className="role-sub">Деканат
            <select name="deanery" style={input} required defaultValue=""><option value="">Выбрать</option>
                {deaneries.map(deanery=><option value={deanery.deaneryID} key={deanery.deaneryID}>{deanery.name}</option>)}
            </select></label>}
        <label className="role-sub">Категория
            <select name="category" style={input} required={!article} defaultValue={article?.newsCategory??''}>
                <option value="">{article?'Без категории':'Выбрать'}</option>
                {Object.entries(newsLabels).map(([value,label])=><option value={value} key={value}>{label}</option>)}
            </select></label>
        <label className="role-sub">Заголовок<input name="title" style={input} defaultValue={article?.newsTitle??''} required maxLength={128}/></label>
        <label className="role-sub">Текст<textarea name="body" style={input} defaultValue={article?.newsDescription??''} required rows={5}/></label>
        <div><button className="role-primary" disabled={busy}>{busy?'Сохраняем…':article?'Сохранить':'Опубликовать'}</button></div>
        {error&&<p className="role-sub" role="alert">{error}</p>}
        {message&&<p className="role-sub" role="status">{message}</p>}
    </form>;
}

export function StaffNews({dean}:{dean:boolean}){
    const query=useCachedQuery('staff-news',async()=>{
        const [news,profile]=await Promise.all([allPages<NewsArticle>('/news'),
            dean?api<{deaneries:Deanery[]}>('/profile/me'):Promise.resolve({deaneries:[]})]);
        return {items:news.items,deaneries:profile.deaneries};
    });
    const [selected,setSelected]=useState<number|null>(null);
    const allowed=query.data?.items.filter(item=>!dean||query.data?.deaneries.some(d=>d.deaneryID===item.deaneryID))??[];
    const current=allowed.find(item=>item.newsID===selected);
    function saved(article:NewsArticle){
        if(query.data)query.update({...query.data,items:[article,...query.data.items.filter(item=>item.newsID!==article.newsID)]
            .sort((a,b)=>Date.parse(b.createdAt??'')-Date.parse(a.createdAt??'')||b.newsID-a.newsID)});
        void query.reload();
    }
    return <section className="role-card">
        <div className="role-head"><div><h2>Новости и объявления</h2><div className="role-sub">Публикации для раздела «Актуальное»</div></div>
            <button className="role-primary" onClick={()=>void query.reload()}>Обновить</button></div>
        {!query.data?<p className="role-sub" role={query.error?'alert':'status'}>{query.error||'Загружаем публикации…'}</p>:
            <div className="role-list">
                {(!dean||query.data.deaneries.length>0)?<NewsEditor deaneries={query.data.deaneries} requireDeanery={dean} onSaved={saved}/>:
                    <p className="role-sub">За аккаунтом пока не закреплён деканат.</p>}
                <strong className="role-name">Публикации, доступные для редактирования</strong>
                {allowed.map(article=><button className="role-row" key={article.newsID} onClick={()=>setSelected(article.newsID)}>
                    <span><span className="role-name">{article.newsTitle}</span>
                        {article.createdAt&&<span className="role-sub" style={{display:'block'}}>{studyDate(article.createdAt)} МСК</span>}</span>
                    <span className="role-status">{article.newsCategory?newsLabels[article.newsCategory]:'Без категории'}</span>
                </button>)}
                {!allowed.length&&<p className="role-sub">Публикаций пока нет.</p>}
                {current&&<NewsEditor key={current.newsID} article={current} deaneries={[]} requireDeanery={false} onSaved={saved}/>}
            </div>}
    </section>;
}
