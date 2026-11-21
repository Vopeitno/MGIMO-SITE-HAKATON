import {useRef,useState} from 'react';
import {api} from '../api';
import {PrivateFile,StaffAction,StaffForm,StaffRow,StaffState} from './StaffCommon';
import {formString,staffInput,staffSelect,useStaffList} from './staff-data';
import type {RosterRow,StaffGroup,StaffMaterial} from './staff-data';

export function StaffRoster({grades=false}:{grades?:boolean}){
    const groupQuery=useStaffList<StaffGroup>('/staff/teacher/groups');
    const [choice,setChoice]=useState('');
    const groups=groupQuery.data??[];
    const courses=[...new Map(groups.map(item=>[item.courseID,item])).values()].sort((a,b)=>b.semester-a.semester||a.title.localeCompare(b.title,'ru'));
    const course=courses.find(item=>item.courseID===choice)??courses[0];
    const roster=useStaffList<RosterRow>(course?'/staff/teacher/roster?courseID='+encodeURIComponent(course.courseID):null);
    return <section className="role-card staff-panel">
        <StaffState data={groupQuery.data} error={groupQuery.error} empty="Дисциплины и группы пока не назначены."/>
        {!grades&&groups.map(group=><StaffRow key={group.courseGroupID} title={group.title}
            description={`Группа ${group.groupNumber} · семестр ${group.semester}`} status={`Студентов: ${group.studentCount}`}/>)}
        {course&&<>
            <label>Дисциплина<select className={staffSelect} value={course.courseID} onChange={event=>setChoice(event.target.value)}>
                {courses.map(item=><option value={item.courseID} key={item.courseID}>{item.title} · семестр {item.semester}</option>)}
            </select></label>
            <StaffState data={roster.data} error={roster.error} empty="В списке этой дисциплины пока нет студентов."/>
            {roster.data?.map(row=><div className="staff-item" key={course.courseID+row.studentID}>
                <StaffRow title={row.studentName} description={`№ ${row.studentNumber} · группа ${row.groupNumber}`}
                    status={row.score==null?'Баллы не выставлены':`${row.score} / 100`}/>
                {grades&&row.editable&&<StaffForm key={row.score??'empty'} label="Сохранить баллы" reset={false} save={async form=>{
                    const score=Number(form.get('score'));
                    if(row.courseGraduate)await api('/academic/grades/'+row.courseGraduate,'PATCH',{score});
                    else await api('/academic/grades','POST',{courseID:course.courseID,studentID:row.studentID,score});
                    await roster.reload();
                }}><label>Баллы от 0 до 100<input className={staffInput} type="number" name="score" min={0} max={100} step={1} required defaultValue={row.score??''}/></label></StaffForm>}
            </div>)}
        </>}
    </section>;
}

async function uploadMaterial(id:string,file:File){
    if(!file.size)throw new Error('Выберите файл для загрузки.');
    if(file.size>10*1024*1024)throw new Error('Файл должен быть не больше 10 МБ.');
    const body=new FormData();
    body.append('file',file);
    await api('/learning/materials/'+id+'/file','POST',body);
}

export function StaffMaterials(){
    const groups=useStaffList<StaffGroup>('/staff/teacher/groups');
    const materials=useStaffList<StaffMaterial>('/learning/materials');
    const [editing,setEditing]=useState<StaffMaterial|null>(null);
    const waiting=useRef<{id:string;fingerprint:string}|null>(null);
    return <section className="role-card staff-panel">
        <h2 className="role-name">{editing?'Редактирование материала':'Новый материал'}</h2>
        <StaffState data={groups.data} error={groups.error} empty="Для публикации преподавателю нужна назначенная учебная группа."/>
        {(editing||!!groups.data?.length)&&<StaffForm key={editing?.id??'new'} label={editing?'Сохранить материал':'Опубликовать материал'} save={async form=>{
            const body={title:formString(form,'title'),description:formString(form,'description'),url:formString(form,'url')};
            if(editing){
                await api('/learning/materials/'+editing.id,'PUT',body);
                setEditing(null);
                await materials.reload();
                return;
            }
            const group=formString(form,'group');
            const chosen=form.get('file');
            const file=chosen instanceof File&&chosen.size>0?chosen:null;
            if(file&&body.url)throw new Error('Выберите один способ: ссылку или файл.');
            if(file&&file.size>10*1024*1024)throw new Error('Файл должен быть не больше 10 МБ.');
            const fingerprint=JSON.stringify([body,group,file?.name,file?.size,file?.lastModified]);
            const existing=waiting.current?.fingerprint===fingerprint?waiting.current.id:null;
            const id=existing??(await api<{id:string}>('/learning/materials','POST',{...body,courseGroupID:group,waitForFile:!!file})).id;
            try{
                if(file){waiting.current={id,fingerprint};await uploadMaterial(id,file)}
                waiting.current=null;
            }finally{await materials.reload()}
        }}>
            {!editing&&<label>Дисциплина и группа<select className={staffSelect} name="group" required>
                {groups.data?.map(g=><option value={g.courseGroupID} key={g.courseGroupID}>{g.title} · группа {g.groupNumber} · семестр {g.semester}</option>)}
            </select></label>}
            <label>Название<input className={staffInput} name="title" maxLength={150} required defaultValue={editing?.title}/></label>
            <label>Описание<textarea className={staffInput} rows={4} name="description" required maxLength={3000} defaultValue={editing?.description}/></label>
            {(!editing||editing.published&&!editing.hasFile)&&<label>Ссылка на материал<input className={staffInput} type="url" name="url" maxLength={1000} defaultValue={editing?.url??''}/></label>}
            {!editing&&<label>Файл вместо ссылки<input className="staff-file" type="file" name="file" accept=".pdf,.docx,.png,.jpg,.jpeg"/></label>}
            {editing&&<button className="role-primary" type="button" onClick={()=>setEditing(null)}>Отменить редактирование</button>}
        </StaffForm>}
        <StaffState data={materials.data} error={materials.error} empty="Материалов пока нет."/>
        {materials.data?.map(item=><div className="staff-item staff-panel" key={item.id}>
            <StaffRow title={item.title} description={`${item.course} · группа ${item.groupNumber}`} status={item.published?'Опубликован':'Ожидает файл'}/>
            <div className="staff-copy">{item.description}</div>
            {item.url&&<a href={item.url} target="_blank" rel="noopener noreferrer">Открыть материал</a>}
            {item.hasFile&&<PrivateFile path={'/learning/materials/'+item.id+'/file'} staff/>}
            {!item.published&&<StaffForm label="Загрузить файл и опубликовать" save={async form=>{
                await uploadMaterial(item.id,form.get('file') as File);if(waiting.current?.id===item.id)waiting.current=null;
                await materials.reload();
            }}><label>Файл материала<input className="staff-file" type="file" name="file" required accept=".pdf,.docx,.png,.jpg,.jpeg"/></label></StaffForm>}
            <div className="staff-actions"><button className="role-primary" type="button" onClick={()=>setEditing(item)}>Редактировать</button>
                <StaffAction label="Удалить" run={async()=>{
                    if(!window.confirm('Удалить материал «'+item.title+'»?'))return;
                    await api('/learning/materials/'+item.id,'DELETE');
                    if(editing?.id===item.id)setEditing(null);
                    if(waiting.current?.id===item.id)waiting.current=null;
                    await materials.reload();
                }}/></div>
        </div>)}
    </section>;
}

export function StudentCourseMaterials({courseID}:{courseID:string}){
    const list=useStaffList<StaffMaterial>('/learning/materials?courseID='+encodeURIComponent(courseID));
    return <>
        <h3 className="card-title" style={{marginTop:20,marginBottom:12}}>Материалы</h3>
        <StaffState data={list.data} error={list.error} empty="Материалы пока не опубликованы."/>
        {list.data?.map(item=><div key={item.id} className="task">
            <div className="row-name">{item.title}</div>
            <div className="row-sub" style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{item.description}</div>
            {item.url&&<a className="link-btn" href={item.url} target="_blank" rel="noopener noreferrer">Открыть материал</a>}
            {item.hasFile&&<PrivateFile path={'/learning/materials/'+item.id+'/file'}/>}
        </div>)}
    </>;
}
