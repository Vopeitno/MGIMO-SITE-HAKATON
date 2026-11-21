import {useState} from 'react';
import {api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import {StaffForm,StaffRow,StaffState} from './StaffCommon';
import {formString,roleLabels,staffInput,staffSelect,useStaffList} from './staff-data';
import type {Directory} from './staff-data';
import {requestDate} from './requests-model';

type AccountRole=keyof typeof roleLabels;
type Account={accountID:string;accountRole:AccountRole;displayName:string;email:string;isActive:boolean};
type Person={id:string;name:string;number:number;personKey:'studentID'|'teacherID'|'employerID'};
type Audit={id:string;actorEmail:string|null;targetEmail:string;action:'created'|'updated';previousRole:AccountRole|null;
    nextRole:AccountRole;previousActive:boolean|null;nextActive:boolean;changedAt:string};
type Overview={accountCounts:{role:AccountRole;total:number;active:number}[];permissions:Record<AccountRole,string[]>;
    mongoConnected:boolean;secureCookies:boolean};

const roles=Object.keys(roleLabels) as AccountRole[];
const staffRoles:AccountRole[]=['DEAN','SUPPORT','ADMIN'];
const personKey=(role:AccountRole)=>role==='STUDENT'?'studentID':role==='TEACHER'?'teacherID':'employerID';
const activityLabel=(value:boolean)=>value?'Активен':'Отключён';

export function AdminSystem({section}:{section:'overview'|'settings'|'roles'}){
    const query=useCachedQuery('admin-overview',()=>api<Overview>('/admin/overview'));
    const data=query.data;
    return <section className="role-card staff-panel">
        <StaffState data={data} error={query.error}/>
        {data&&section==='overview'&&<>
            <StaffRow title="PostgreSQL" status="Подключена"/>
            <StaffRow title="Сервис сообщений" status={data.mongoConnected?'Подключён':'Недоступен'}/>
            <StaffRow title="Учётных записей" status={data.accountCounts.reduce((n,item)=>n+item.total,0)}/>
            <StaffRow title="Активных учётных записей" status={data.accountCounts.reduce((n,item)=>n+item.active,0)}/>
        </>}
        {data&&section==='settings'&&<>
            <StaffRow title="Авторизация" description="Сессии с обновлением токена"/>
            <StaffRow title="Защищённые cookies" status={data.secureCookies?'Включены':'Локальный режим'}/>
            <StaffRow title="Сервис сообщений" status={data.mongoConnected?'Подключён':'Недоступен'}/>
        </>}
        {data&&section==='roles'&&roles.map(role=><details className="staff-item" key={role}>
            <summary><strong>{roleLabels[role]}</strong> · учётных записей {data.accountCounts.find(item=>item.role===role)?.total??0}</summary>
            <div className="role-list" style={{marginTop:10}}>
                {[...new Set(data.permissions[role]??[])].sort().map(permission=><code key={permission}>{permission}</code>)}
            </div>
        </details>)}
    </section>;
}

export function AdminAccounts(){
    const accounts=useStaffList<Account>('/admin/accounts');
    const [create,setCreate]=useState(false);
    const [query,setQuery]=useState('');
    const [chosenRole,setRole]=useState<AccountRole>('STUDENT');
    const [selected,setSelected]=useState('');
    const people=useStaffList<Person>(create?'/admin/persons':null);
    const filtered=accounts.data?.filter(item=>`${item.displayName} ${item.email} ${roleLabels[item.accountRole]}`.toLowerCase().includes(query.trim().toLowerCase()))??null;
    const candidates=people.data?.filter(item=>item.personKey===personKey(chosenRole))??[];
    return <section className="role-card staff-panel">
        <div className="staff-actions"><button className="role-primary" type="button" onClick={()=>setCreate(!create)}>
            {create?'Закрыть форму':'Создать учётную запись'}
        </button></div>
        {create&&<StaffForm label="Создать аккаунт" save={async form=>{
            const id=formString(form,'person');
            await api('/admin/accounts','POST',{email:formString(form,'email'),password:String(form.get('password')??''),accountRole:chosenRole,[personKey(chosenRole)]:id});
            await Promise.all([accounts.reload(),people.reload()]);setCreate(false);
        }}>
            <label>Роль<select className={staffSelect} value={chosenRole} onChange={event=>setRole(event.target.value as AccountRole)}>
                {roles.map(role=><option value={role} key={role}>{roleLabels[role]}</option>)}
            </select></label>
            <label>Студент или сотрудник<select className={staffSelect} key={chosenRole} name="person" required defaultValue="">
                <option value="" disabled>{candidates.length?'Выберите человека':'Подходящих записей без аккаунта нет'}</option>
                {candidates.map(person=><option key={person.id} value={person.id}>{person.name} · № {person.number}</option>)}
            </select></label>
            {people.error&&<p className="role-sub" role="alert">{people.error}</p>}
            <label>Электронная почта<input className={staffInput} name="email" type="email" required maxLength={258}/></label>
            <label>Пароль<input className={staffInput} name="password" type="password" autoComplete="new-password" required minLength={12}/></label>
        </StaffForm>}
        <label>Поиск по имени, почте или роли<input className={staffInput} value={query} onChange={event=>setQuery(event.target.value)}/></label>
        <StaffState data={filtered} error={accounts.error} empty="Учётные записи не найдены."/>
        {filtered?.map(item=><article className="staff-item staff-panel" key={item.accountID}>
            <StaffRow title={item.displayName} description={item.email+' · '+roleLabels[item.accountRole]} status={activityLabel(item.isActive)}/>
            <button className="role-primary" type="button" onClick={()=>setSelected(selected===item.accountID?'':item.accountID)}>
                {selected===item.accountID?'Отменить':'Изменить доступ'}
            </button>
            {selected===item.accountID&&<StaffForm label="Сохранить доступ" save={async form=>{
                await api('/admin/accounts/'+item.accountID,'PATCH',{accountRole:formString(form,'role'),isActive:form.get('active')==='yes'});
                await accounts.reload();setSelected('');
            }}>
                <label>Роль<select className={staffSelect} name="role" defaultValue={item.accountRole}>
                    {(personKey(item.accountRole)==='employerID'?staffRoles:[item.accountRole]).map(role=><option key={role} value={role}>{roleLabels[role]}</option>)}
                </select></label>
                <label>Состояние<select className={staffSelect} name="active" defaultValue={item.isActive?'yes':'no'}>
                    <option value="yes">Активен</option><option value="no">Отключён</option>
                </select></label>
            </StaffForm>}
        </article>)}
    </section>;
}

export function AdminAudit(){
    const query=useStaffList<Audit>('/admin/audit');
    return <section className="role-card staff-panel">
        <StaffState data={query.data} error={query.error} empty="Изменения аккаунтов пока не зарегистрированы."/>
        {query.data?.map(item=><StaffRow key={item.id} title={item.action==='created'?'Создан аккаунт':'Изменён доступ'}
            description={<>{item.targetEmail} · {requestDate(item.changedAt)}<br/>{item.actorEmail??'Сотрудник'} ·
                {item.previousRole?` ${roleLabels[item.previousRole]} / ${activityLabel(!!item.previousActive)} →`:''}
                {' '+roleLabels[item.nextRole]} / {activityLabel(item.nextActive)}</>}/>)}
    </section>;
}

export function AdminDirectories(){
    const query=useCachedQuery('admin-directories',()=>api<Directory>('/staff/directories'));
    const [kind,setKind]=useState<'building'|'room'>('building');
    const [selected,setSelected]=useState('');
    const b=query.data?.buildings.find(item=>item.buildingID===selected);
    const c=query.data?.classrooms.find(item=>String(item.classroomID)===selected);
    return <section className="role-card staff-panel">
        <div className="staff-actions">
            <button className="role-primary" type="button" aria-pressed={kind==='building'} onClick={()=>{setKind('building');setSelected('')}}>Корпуса</button>
            <button className="role-primary" type="button" aria-pressed={kind==='room'} onClick={()=>{setKind('room');setSelected('')}}>Аудитории</button>
        </div>
        <StaffState data={query.data} error={query.error}/>
        {query.data&&kind==='building'&&<>
            <StaffForm key={selected||'new'} label={b?'Сохранить корпус':'Добавить корпус'} save={async form=>{
                await api(b?'/admin/buildings/'+b.buildingID:'/admin/buildings',b?'PUT':'POST',
                    {building:formString(form,'building'),address:formString(form,'address')});
                await query.reload();setSelected('');
            }}>
                <label>Название корпуса<input className={staffInput} name="building" required maxLength={256} defaultValue={b?.building}/></label>
                <label>Адрес<input className={staffInput} name="address" maxLength={256} defaultValue={b?.buildingAddress??''}/></label>
                {b&&<button className="role-primary" type="button" onClick={()=>setSelected('')}>Отменить редактирование</button>}
            </StaffForm>
            {query.data.buildings.map(item=><div key={item.buildingID} className="staff-item staff-panel">
                <StaffRow title={item.building} description={item.buildingAddress||'Адрес не указан'}/>
                <button className="role-primary" type="button" onClick={()=>setSelected(item.buildingID)}>Редактировать корпус</button>
            </div>)}
        </>}
        {query.data&&kind==='room'&&<>
            {query.data.buildings.length>0?<StaffForm key={selected||'new'} label={c?'Сохранить аудиторию':'Добавить аудиторию'} save={async form=>{
                await api(c?'/admin/classrooms/'+c.classroomID:'/admin/classrooms',c?'PUT':'POST',
                    {buildingID:formString(form,'building'),number:Number(form.get('number'))});
                await query.reload();setSelected('');
            }}>
                <label>Корпус<select className={staffSelect} name="building" defaultValue={c?.buildingID} required>
                    {query.data.buildings.map(item=><option key={item.buildingID} value={item.buildingID}>{item.building}</option>)}
                </select></label>
                <label>Номер аудитории<input className={staffInput} type="number" name="number" min={1} max={2147483647} required defaultValue={c?.classroomNumber}/></label>
                {c&&<button className="role-primary" type="button" onClick={()=>setSelected('')}>Отменить редактирование</button>}
            </StaffForm>:<p className="role-sub">Сначала добавьте корпус.</p>}
            {query.data.classrooms.map(item=><div className="staff-item staff-panel" key={item.classroomID}>
                <StaffRow title={'Аудитория '+item.classroomNumber} description={query.data?.buildings.find(b=>b.buildingID===item.buildingID)?.building}/>
                <button className="role-primary" type="button" onClick={()=>setSelected(String(item.classroomID))}>Редактировать аудиторию</button>
            </div>)}
        </>}
    </section>;
}
