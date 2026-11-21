import {DeanTeachers,StaffCourses} from "./StaffForms";
import {AdminAccounts,AdminAudit,AdminDirectories,AdminSystem} from "./StaffAdmin";
import {StaffAccountPanel,StaffStyles} from "./StaffCommon";
import {DeanReports,DeanStudents,StaffRequests,StaffSchedule,SupportAnalytics} from "./StaffOperations";
import {StaffMaterials,StaffRoster} from "./StaffTeaching";
import {DeanCareer} from "./Career";
import {ChatBadge} from "./ChatBadge";
import {useUnreadChats} from "./chat-inbox";
import {SupportKnowledgeEditor,SupportTickets} from "./SupportTickets";
import {StaffCampusServices} from "./CampusServices";
import {StaffChat} from "./StaffChat";
import {StaffNews} from "./StaffNews";
import {ActivityCheckIn, DeanActivities, DeanStudy, TeacherAttendance} from "./StudyStaff";
import DeaneryMessages from "./DeaneryMessages";
import TeacherDeadlineLinks from "./TeacherDeadlineLinks";
import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";

export type StaffRole = "teacher" | "dean" | "support" | "admin";
type IconName = "home" | "calendar" | "chart" | "file" | "book" | "grid" | "bell" | "user" | "plus" | "help" | "logout";
type Section = {id:string;label:string;icon:IconName};
type Config = {name:string;subtitle:string;sections:Section[]};

const paths:Record<IconName,ReactNode>={home:<><path d="M3.5 11 12 4l8.5 7v9H15v-6H9v6H3.5v-9Z"/></>,calendar:<><rect x="3.5" y="5" width="17" height="15" rx="3"/><path d="M7 3.5V7M17 3.5V7M3.5 10h17"/></>,chart:<path d="M5 20V10M12 20V4M19 20v-7"/>,file:<><path d="M7 3.5h7l4 4v13H7a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z"/><path d="M14 3.5V8h4M9 12h6M9 16h5"/></>,book:<><path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H11v17H6.5A2.5 2.5 0 0 0 4 22V5.5ZM20 5.5A2.5 2.5 0 0 0 17.5 3H13v17h4.5A2.5 2.5 0 0 1 20 22V5.5Z"/></>,grid:<><rect x="3.5" y="3.5" width="7" height="7" rx="2"/><rect x="13.5" y="3.5" width="7" height="7" rx="2"/><rect x="3.5" y="13.5" width="7" height="7" rx="2"/><rect x="13.5" y="13.5" width="7" height="7" rx="2"/></>,bell:<><path d="M18.5 9a6.5 6.5 0 1 0-13 0c0 7-2.5 7-2.5 8.5h18C21 16 18.5 16 18.5 9Z"/><path d="M9.5 20a3 3 0 0 0 5 0"/></>,user:<><circle cx="12" cy="8" r="4"/><path d="M4.5 20c.8-4 3.3-6 7.5-6s6.7 2 7.5 6"/></>,plus:<path d="M12 5v14M5 12h14"/>,help:<><circle cx="12" cy="12" r="9"/><path d="M9.7 9a2.5 2.5 0 1 1 3.7 2.2c-.9.5-1.4 1-1.4 2M12 17h.01"/></>,logout:<><path d="M10 5H5v14h5M14 8l4 4-4 4M18 12H9"/></>};
function Icon({name,size=22}:{name:IconName;size?:number}){return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>}

const configs:Record<StaffRole,Config>={
 teacher:{name:"Преподаватель",subtitle:"учебный кабинет",sections:[{id:"overview",label:"Сегодня",icon:"home"},{id:"schedule",label:"Расписание",icon:"calendar"},{id:"groups",label:"Мои группы",icon:"user"},{id:"grades",label:"Оценки",icon:"chart"},{id:"requests",label:"Заявки",icon:"file"},{id:"materials",label:"Материалы",icon:"book"},{id:"deadlines",label:"Дедлайны",icon:"calendar"},{id:"sport-attendance",label:"Физкультура",icon:"calendar"},{id:"activity-check-in",label:"QR мероприятий",icon:"file"},{id:"online",label:"Онлайн-занятия",icon:"grid"},{id:"chat",label:"Чаты",icon:"bell"}]},
 dean:{name:"Деканат",subtitle:"управление факультетом",sections:[{id:"overview",label:"Сводка",icon:"home"},{id:"requests",label:"Заявления",icon:"file"},{id:"students",label:"Студенты",icon:"user"},{id:"teachers",label:"Преподаватели",icon:"user"},{id:"courses",label:"Курсы",icon:"book"},{id:"schedule",label:"Расписание",icon:"calendar"},{id:"reports",label:"Отчёты",icon:"chart"},{id:"career",label:"Карьера",icon:"grid"},{id:"campus-services",label:"Льготы и бонусы",icon:"file"},{id:"announcements",label:"Объявления",icon:"bell"},{id:"messages",label:"Вопросы студентов",icon:"bell"},{id:"chat",label:"Чаты",icon:"user"},{id:"study-plans",label:"Учебные планы",icon:"book"},{id:"activities",label:"Афиша кампуса",icon:"calendar"}]},
 support:{name:"Поддержка",subtitle:"единое окно помощи",sections:[{id:"overview",label:"Очередь",icon:"home"},{id:"tickets",label:"Обращения",icon:"help"},{id:"requests",label:"Заявки",icon:"file"},{id:"schedule",label:"Расписание",icon:"calendar"},{id:"courses",label:"Курсы",icon:"book"},{id:"chat",label:"Диалоги",icon:"bell"},{id:"knowledge",label:"База знаний",icon:"book"},{id:"news",label:"Новости",icon:"bell"},{id:"activity-check-in",label:"Посещения мероприятий",icon:"file"},{id:"campus-services",label:"Сервисы кампуса",icon:"grid"},{id:"analytics",label:"Аналитика",icon:"chart"}]},
 admin:{name:"Администратор",subtitle:"управление системой",sections:[{id:"overview",label:"Система",icon:"home"},{id:"users",label:"Пользователи",icon:"user"},{id:"roles",label:"Роли и доступ",icon:"grid"},{id:"directories",label:"Справочники",icon:"book"},{id:"audit",label:"Журнал действий",icon:"file"},{id:"settings",label:"Настройки",icon:"help"}]}
};

const css=`*{box-sizing:border-box}body{margin:0}.role-app{--bg:#f7f6fb;--text:#252335;--muted:#77738a;--line:#ece9f4;--violet:#8b7cf6;--violet-soft:#e8e4ff;--blue:#dceeff;--mint:#daf4e9;--shadow:0 12px 36px #40346f14;min-height:100vh;background:var(--bg);color:var(--text);font-family:Manrope,Inter,system-ui,sans-serif}.role-app button{font:inherit;color:inherit}.role-shell{display:grid;grid-template-columns:252px minmax(0,1fr);gap:18px;min-height:100vh;padding:18px}.role-side{position:sticky;top:18px;display:flex;flex-direction:column;height:calc(100vh - 36px);padding:24px 18px 18px;border-radius:28px;background:#fff;box-shadow:var(--shadow)}.role-brand{display:flex;align-items:center;gap:12px;padding:0 8px 24px}.role-mark,.role-avatar{display:grid;place-items:center;flex:0 0 auto;color:#fff;font-weight:900;background:linear-gradient(145deg,#a495ff,#7466e6)}.role-mark{width:42px;height:42px;border-radius:15px}.role-title{font-weight:900}.role-sub{color:var(--muted);font-size:11px;line-height:1.4}.role-nav{display:grid;align-content:start;gap:6px;flex:1;overflow-y:auto}.role-nav button{display:flex;align-items:center;gap:12px;min-height:48px;padding:0 14px;border:0;border-radius:15px;color:var(--muted);background:transparent;cursor:pointer}.role-nav button.active{color:#6759d9;background:var(--violet-soft);font-weight:850}.role-profile{display:flex;align-items:center;gap:11px;padding:12px;border:0;border-radius:18px;background:#faf9fd;text-align:left}.role-avatar{width:42px;height:42px;border-radius:50%}.role-main{width:min(1220px,100%);margin:auto;padding:20px 22px 120px}.role-top{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:24px}.role-top h1{margin:5px 0 0;font-size:clamp(28px,3vw,40px);letter-spacing:-.04em}.role-actions{display:flex;gap:10px}.role-icon{display:grid;place-items:center;width:46px;height:46px;border:1px solid var(--line);border-radius:16px;background:#fff;cursor:pointer}.role-metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.role-metric,.role-card{border-radius:22px;background:#fff;box-shadow:var(--shadow)}.role-metric{padding:18px}.role-metric strong{display:block;font-size:28px}.role-card{margin-top:20px;padding:22px}.role-head,.role-row{display:flex;align-items:center;justify-content:space-between;gap:14px}.role-head{margin-bottom:17px}.role-head h2{margin:0;font-size:18px}.role-primary{display:flex;align-items:center;gap:7px;min-height:42px;padding:0 15px;border:0;border-radius:14px;color:#fff;background:linear-gradient(135deg,#9283f7,#7567df);cursor:pointer}.role-list{display:grid;gap:10px}.role-row{width:100%;padding:15px;border:0;border-radius:17px;background:#faf9fd;text-align:left;cursor:pointer}.role-row:hover{background:#f1eeff}.role-name{font-size:13px;font-weight:900}.role-status{max-width:42%;padding:7px 10px;border-radius:99px;color:#477865;font-size:10px;font-weight:850;text-align:center;background:var(--mint)}.role-quick{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.role-quick button{min-height:105px;padding:15px;border:0;border-radius:18px;background:var(--violet-soft);cursor:pointer}.role-quick button:nth-child(2){background:var(--blue)}.role-quick button:nth-child(3){background:var(--mint)}.role-quick button:nth-child(4){background:#ffe7da}.role-bottom{display:none}.role-toast{position:fixed;left:50%;bottom:25px;z-index:90;padding:11px 16px;border-radius:14px;color:#fff;background:#373246;opacity:0;transform:translate(-50%,15px);transition:.2s}.role-toast.show{opacity:1;transform:translate(-50%,0)}@media(max-width:900px){.role-shell{grid-template-columns:88px 1fr}.role-copy{display:none}.role-nav button{justify-content:center;padding:0}.role-profile{justify-content:center;padding:8px}.role-metrics,.role-quick{grid-template-columns:repeat(2,1fr)}}@media(max-width:680px){.role-shell{display:block;padding:0}.role-side{display:none}.role-main{padding:22px 16px 115px}.role-metrics{grid-template-columns:repeat(2,minmax(0,1fr))}.role-card{padding:18px}.role-head{align-items:flex-start;flex-direction:column}.role-row{align-items:flex-start;flex-direction:column}.role-status{max-width:100%}.role-quick{grid-template-columns:1fr 1fr}.role-bottom{position:fixed;left:10px;right:10px;bottom:10px;z-index:50;display:grid;grid-template-columns:repeat(5,1fr);padding:8px;border-radius:22px;background:#fffffff2;box-shadow:var(--shadow)}.role-bottom button{display:grid;place-items:center;gap:4px;min-height:53px;padding:3px;border:0;border-radius:14px;color:var(--muted);font-size:8px;background:transparent}.role-bottom button.active{color:#6759d9;background:var(--violet-soft)}}`;

function routePart(id:string){
  return id==="chat"?"chats":id==="announcements"?"news":id;
}

function Workspace({role,active,accountID,toast}:{role:StaffRole;active:string;accountID:string;toast:(text:string)=>void}){
  if(active==="chat"&&role!=="admin")return <StaffChat key={accountID} accountID={accountID} add={<Icon name="plus" size={18}/>} info={<Icon name="help" size={18}/>}/>;
  if((role==="dean"&&active==="announcements")||(role==="support"&&active==="news"))return <StaffNews dean={role==="dean"}/>;
  if(active==="career"&&role==="dean")return <DeanCareer/>;
  if(active==="campus-services")return <StaffCampusServices dean={role==="dean"}/>;
  if(active==="activity-check-in")return <ActivityCheckIn manual={role==="support"}/>;
  if(active==="courses"&&(role==="dean"||role==="support"))return <StaffCourses dean={role==="dean"}/>;
  if(active==="schedule")return <StaffSchedule editable={role==="support"}/>;
  if(role==="teacher"){
    if(active==="groups"||active==="grades")return <StaffRoster grades={active==="grades"}/>;
    if(active==="requests")return <StaffRequests teacher/>;
    if(active==="materials")return <StaffMaterials/>;
    if(active==="deadlines")return <TeacherDeadlineLinks toast={toast}/>;
    if(active==="sport-attendance")return <TeacherAttendance/>;
    if(active==="online")return <StaffSchedule online/>;
    return <StaffSchedule todayOnly/>;
  }
  if(role==="dean"){
    if(active==="students")return <DeanStudents/>;
    if(active==="teachers")return <DeanTeachers/>;
    if(active==="requests")return <StaffRequests dean/>;
    if(active==="reports")return <DeanReports/>;
    if(active==="study-plans")return <DeanStudy/>;
    if(active==="activities")return <DeanActivities/>;
    if(active==="messages")return <section className="role-card"><div className="role-head"><h2>Вопросы студентов</h2></div><DeaneryMessages accountID={accountID} staff/></section>;
    return <DeanReports overview/>;
  }
  if(role==="support"){
    if(active==="knowledge")return <SupportKnowledgeEditor/>;
    if(active==="requests")return <StaffRequests/>;
    if(active==="analytics")return <SupportAnalytics/>;
    return <SupportTickets manage/>;
  }
  if(active==="users")return <AdminAccounts/>;
  if(active==="audit")return <AdminAudit/>;
  if(active==="directories")return <AdminDirectories/>;
  return <AdminSystem section={active==="settings"||active==="roles"?active:"overview"}/>;
}

export default function RoleDashboard({role,logout,displayName,accountID=""}:{role:StaffRole;logout:()=>void;displayName?:string;accountID?:string}){
  const c=configs[role];
  const unreadChats=useUnreadChats(role==="admin"?"":accountID);
  const [active,setActive]=useState(()=>c.sections.find(section=>typeof window!=="undefined"&&window.location.hash===`#${role}-${routePart(section.id)}`)?.id??c.sections[0].id);
  const [toast,setToast]=useState("");
  const [panel,setPanel]=useState<"profile"|"notifications"|"menu"|null>(null);
  const modal=useRef<HTMLDialogElement>(null);
  useEffect(()=>{if(!toast)return;const timer=window.setTimeout(()=>setToast(""),1800);return()=>window.clearTimeout(timer)},[toast]);
  useEffect(()=>{if(panel&&!modal.current?.open)modal.current?.showModal()},[panel]);
  const initials=(displayName||c.name).split(/\s+/).slice(0,2).map(value=>value[0]).join("");
  const section=c.sections.find(item=>item.id===active)??c.sections[0];
  const selectSection=(id:string)=>{
    setActive(id);
    window.history.replaceState(null,"",window.location.pathname+window.location.search+`#${role}-${routePart(id)}`);
  };
  const quick:Section[]=role==="teacher"?c.sections.filter(item=>["groups","grades","materials","sport-attendance"].includes(item.id)):
    role==="dean"?c.sections.filter(item=>["students","reports","announcements","activities"].includes(item.id)):
    role==="support"?c.sections.filter(item=>["requests","analytics","news","activity-check-in"].includes(item.id)):
    c.sections.filter(item=>["users","roles","directories","audit"].includes(item.id));
  return <div className="role-app">
    <style>{css}</style><StaffStyles/>
    <div className="role-shell">
      <aside className="role-side">
        <div className="role-brand"><div className="role-mark">{c.name[0]}</div><div className="role-copy"><div className="role-title">{c.name}</div><div className="role-sub">{c.subtitle}</div></div></div>
        <nav className="role-nav">{c.sections.map(item=><button key={item.id} className={active===item.id?"active":""} onClick={()=>selectSection(item.id)}>
          <Icon name={item.icon}/><span className="role-copy">{item.label}</span>{item.id==="chat"&&unreadChats&&<ChatBadge/>}
        </button>)}</nav>
        <button className="role-profile" onClick={()=>setPanel("profile")}><div className="role-avatar">{initials}</div><div className="role-copy">
          <div className="role-title" style={{fontSize:13}}>{displayName||c.name}</div><div className="role-sub">{c.name}</div>
        </div></button>
      </aside>
      <main className="role-main">
        <header className="role-top"><div><div className="role-sub">{c.name} · рабочее пространство</div><h1>{section.label}</h1></div>
          <div className="role-actions"><button className="role-icon" aria-label="Уведомления" onClick={()=>setPanel("notifications")}><Icon name="bell"/></button>
            <button className="role-avatar" aria-label="Профиль" onClick={()=>setPanel("profile")}>{initials}</button></div>
        </header>
        <Workspace key={role+":"+active} role={role} active={active} accountID={accountID} toast={setToast}/>
        <section className="role-card"><div className="role-head"><h2>Быстрые действия</h2></div>
          <div className="role-quick">{quick.filter(item=>item.id!==active).map(item=><button key={item.id} onClick={()=>selectSection(item.id)}>
            <Icon name={item.icon}/><div className="role-name">{item.label}</div></button>)}
            <button onClick={logout}><Icon name="logout"/><div className="role-name">Выйти</div></button>
          </div>
        </section>
      </main>
    </div>
    <nav className="role-bottom">{c.sections.slice(0,4).map(item=><button key={item.id} className={active===item.id?"active":""} onClick={()=>selectSection(item.id)}>
      <Icon name={item.icon} size={19}/><span>{item.label}</span></button>)}
      <button onClick={()=>setPanel("menu")}><Icon name="grid" size={19}/><span>Ещё{unreadChats&&<ChatBadge/>}</span></button>
    </nav>
    <dialog ref={modal} className="role-card" aria-labelledby="role-dialog-title" onClose={()=>setPanel(null)}
      style={{width:560,maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",border:"1px solid var(--line)",color:"inherit"}}>
      {panel&&<><div className="role-head"><h2 id="role-dialog-title">{panel==="menu"?"Все разделы":panel==="profile"?"Профиль":"Уведомления"}</h2>
        <button className="role-icon" type="button" aria-label="Закрыть" onClick={()=>modal.current?.close()}>×</button></div>
        {panel==="menu"?<div className="role-list">{c.sections.map(item=><button className="role-row" key={item.id}
          onClick={()=>{selectSection(item.id);modal.current?.close()}}><span>{item.label}{item.id==="chat"&&unreadChats&&<ChatBadge/>}</span></button>)}</div>
          :<StaffAccountPanel key={panel} notifications={panel==="notifications"}/>}
      </>}
    </dialog>
    <div className={`role-toast ${toast?"show":""}`}>{toast}</div>
  </div>;
}
