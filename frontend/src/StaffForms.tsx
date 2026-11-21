import {useState} from 'react';
import {api} from '../api';
import {PrivateFile,StaffForm,StaffRow,StaffState} from './StaffCommon';
import {formString,staffInput,staffSelect,useStaffList} from './staff-data';
import type {Building,RequestRecord,RequestType} from './requests-model';
import {requestDate} from './requests-model';
import {appointmentStatus} from './today-model';

type Course={courseID:string;facultyID:string;title:string;description:string|null;hours:number;semester:number};
type Faculty={facultyID:string;title:string};
type Teacher={teacherID:string;teacherNumber:number;teacherName:string;teacherPosition:string;teacherDegree:string;teacherStatus:string};

export function StaffOwnRequests(){
    const own=useStaffList<RequestRecord>('/requests');
    const types=useStaffList<RequestType>('/requests/types');
    const buildings=useStaffList<Building>('/buildings');
    return <>
        <StaffState data={types.data} error={types.error} empty="Типы заявок пока не настроены."/>
        {!!types.data?.length&&<StaffForm label="Отправить новую заявку" save={async form=>{
            await api('/requests','POST',{appointmentTypeID:formString(form,'type'),appointmentTitle:formString(form,'title'),
                appointmentDescription:formString(form,'description'),buildingID:formString(form,'building')||null});
            await own.reload();
        }}>
            <label>Вид заявки<select className={staffSelect} name="type" required>
                {types.data.map(item=><option key={item.appointmentTypeID} value={item.appointmentTypeID}>{item.appointmentTypeTitle}</option>)}
            </select></label>
            <label>Заголовок<input className={staffInput} name="title" maxLength={100} required/></label>
            <label>Описание<textarea className={staffInput} name="description" required/></label>
            <label>Корпус<select className={staffSelect} name="building"><option value="">Без указания корпуса</option>
                {buildings.data?.map(item=><option key={item.buildingID} value={item.buildingID}>{item.building}</option>)}
            </select></label>
        </StaffForm>}
        <StaffState data={own.data} error={own.error} empty="В истории пока нет заявок."/>
        {own.data?.map(item=><div className="staff-item staff-panel" key={item.appoinmentID}>
            <StaffRow title={item.appointmentTitle} description={requestDate(item.created_at)} status={appointmentStatus(item.appointmentStatus)}/>
            <div className="staff-copy">{item.appointmentDescription}</div>
            {item.fileUrlAttachment&&<PrivateFile path={'/requests/'+item.appoinmentID+'/attachment'} staff/>}
        </div>)}
    </>;
}

export function StaffCourses({dean}:{dean:boolean}){
    const query=useStaffList<Course>('/courses');
    const faculties=useStaffList<Faculty>(dean?'/career/faculties':'/staff/support/faculties');
    const [selected,setSelected]=useState<Course|null>(null);
    const [search,setSearch]=useState('');
    const courses=query.data?.filter(item=>item.title.toLowerCase().includes(search.trim().toLowerCase()))??null;
    return <section className="role-card staff-panel">
        <h2 className="role-name">{selected?'Редактирование курса':'Новый курс'}</h2>
        <StaffState data={faculties.data} error={faculties.error} empty="Доступных факультетов пока нет."/>
        {!!faculties.data?.length&&<StaffForm key={selected?.courseID??'new'} label={selected?'Сохранить курс':'Добавить курс'} save={async form=>{
            const data={facultyID:formString(form,'faculty'),title:formString(form,'title'),description:formString(form,'description')||null,
                hours:Number(form.get('hours')),semester:Number(form.get('semester'))};
            await api(selected?'/courses/'+selected.courseID:'/courses',selected?'PATCH':'POST',data);await query.reload();setSelected(null);
        }}>
            <label>Факультет<select name="faculty" className={staffSelect} required defaultValue={selected?.facultyID}>
                {faculties.data.map(faculty=><option key={faculty.facultyID} value={faculty.facultyID}>{faculty.title}</option>)}
            </select></label>
            <label>Название<input className={staffInput} name="title" required maxLength={128} defaultValue={selected?.title}/></label>
            <label>Описание<textarea className={staffInput} name="description" maxLength={564} defaultValue={selected?.description??''}/></label>
            <label>Часы<input className={staffInput} name="hours" type="number" min={1} required defaultValue={selected?.hours}/></label>
            <label>Семестр<input className={staffInput} name="semester" type="number" min={1} required defaultValue={selected?.semester}/></label>
            {selected&&<button className="role-primary" type="button" onClick={()=>setSelected(null)}>Отменить редактирование</button>}
        </StaffForm>}
        <label>Поиск курса<input className={staffInput} value={search} onChange={event=>setSearch(event.target.value)}/></label>
        <StaffState data={courses} error={query.error} empty="Курсы не найдены."/>
        {courses?.map(course=><article className="staff-item staff-panel" key={course.courseID}>
            <StaffRow title={course.title} description={`Семестр ${course.semester} · ${course.hours} ч`} />
            <button className="role-primary" type="button" onClick={()=>setSelected(course)}>Редактировать курс</button>
        </article>)}
    </section>;
}

export function DeanTeachers(){
    const query=useStaffList<Teacher>('/faculty/teachers');
    return <section className="role-card staff-panel">
        <StaffState data={query.data} error={query.error} empty="Преподаватели факультетов не найдены."/>
        {query.data?.map(item=><StaffRow key={item.teacherID} title={item.teacherName}
            description={[item.teacherNumber&&'№ '+item.teacherNumber,item.teacherPosition,item.teacherDegree].filter(Boolean).join(' · ')} />)}
    </section>;
}
