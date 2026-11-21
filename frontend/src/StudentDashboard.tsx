import "./student-header-spacing.css";
import {Portfolio} from "./Portfolio";
import {Career} from "./Career";
import {ChatBadge} from "./ChatBadge";
import {useUnreadChats} from "./chat-inbox";
import {selectArrowCSS} from "./select-arrow";
import {findFAQ} from "./faq-search";
import {SupportTickets} from "./SupportTickets";
import {createSupportTicket,ticketCategories,useSupportArticles} from "./support-data";
import type {TicketCategory,TicketPriority} from "./support-data";
import {CampusServiceContent} from "./CampusServices";
import type {CampusMapProvider,CampusService} from "./campus-data";
import {campusAddress,campusRoute,campusServiceTitles,roomQuery,useCampusData} from "./campus-data";
import type {RoomFilter,RoomResult} from "./campus-data";
import {ChatPanel} from "./ChatPanel";
import type {ApiLesson,ApiProfile,ApiNotification} from "./today-model";
import {isNewsHash,newsExcerpt,newsIDFromHash,newsLabels,useNewsData} from "./news-data";
import type {NewsArticle,NewsCategory} from "./news-data";
import {ActivityTicket} from "./ActivityTicket";
import {activityLabels,useActivitiesData} from "./activities-data";
import type {ActivityCategory,CampusEvent} from "./activities-data";
import {useCachedQuery} from "./useCachedQuery";
import {StudentSettings} from "./StudentSettings";
import type {SettingsPanel} from "./StudentSettings";
import {ProfileAvatar, ProfileQR, ProfileQRDialog} from "./ProfileMedia";
import {useProfileData} from "./useProfileData";
import {LearningEnrollment, LearningPhysical} from './LearningEnrollment';
import LearningCourseDetails from "./LearningCourseDetails";
import {useLearningData} from "./useLearningData";
import DeaneryMessages from "./DeaneryMessages";
import React, { lazy, Suspense, useEffect, useState, useRef } from "react";
import type { FormEvent } from "react";

import { login as serverLogin, logout as serverLogout, refresh, onSessionExpired, ApiError } from "../api";
import type { User } from "../api";
import { api, download as downloadFile } from "../api";
import { useNotificationsData, markNotificationsRead } from "./useNotificationsData";
import { useRequestsData } from "./useRequestsData";
import { requestDate, requestDescription, requestPickup, requestFileError } from "./requests-model";
import type { Building, RequestRecord, RequestType, RequestHistory } from "./requests-model";
import { submitRequestDraft } from "./request-service";
import { allPages } from "../api";
import { useTodayData } from "./useTodayData";
import { useDeadlinesData } from "./useDeadlinesData";
import { deadlinesCalendar } from "./deadlines-model";
import { useScheduleData } from "./useScheduleData";
import { shiftDate, weekPeriod } from "./schedule-model";
import { ScheduleDatePicker } from "./ScheduleDatePicker";
import type { ScheduleLesson } from "./schedule-model";
import { gradesForYear } from "./grades-model";
import { useGradesData } from "./useGradesData";
import { appointmentStatus, safeLessonLink, moscowDate } from "./today-model";
import type { ApiAppointment, LessonView, TodaySummary } from "./today-model";

const RoleDashboard=lazy(()=>import("./RoleDashboard"));
type Screen = "today" | "schedule" | "deadlines" | "grades" | "requests" | "learning" | "news" | "chat" | "activities" | "campus" | "career" | "finance" | "portfolio" | "services" | "support" | "notifications" | "profile" | "login";
type Modal = "newRequest" | "success" | "details" | null;
type IconName = "home" | "calendar" | "chart" | "file" | "book" | "spark" | "grid" | "bell" | "user" | "plus" | "check" | "download" | "help" | "logout";
type RequestItem = {id:string;displayId?:string;title:string;date:string;status:string;tone:string};
type UserRole = "student" | "teacher" | "dean" | "support" | "admin";

const iconPaths: Record<IconName, React.ReactNode> = {
  home:<><path d="M3.5 11 12 4l8.5 7v8.5a1 1 0 0 1-1 1H15v-6H9v6H4.5a1 1 0 0 1-1-1V11Z"/><path d="M9 20.5v-6h6v6"/></>,
  calendar:<><rect x="3.5" y="5" width="17" height="15" rx="3"/><path d="M7 3.5V7M17 3.5V7M3.5 10h17"/></>, chart:<path d="M5 20V10M12 20V4M19 20v-7"/>,
  file:<><path d="M7 3.5h7l4 4v13H7a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z"/><path d="M14 3.5V8h4M9 12h6M9 16h5"/></>,
  book:<><path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H11v17H6.5A2.5 2.5 0 0 0 4 22V5.5ZM20 5.5A2.5 2.5 0 0 0 17.5 3H13v17h4.5A2.5 2.5 0 0 1 20 22V5.5Z"/></>, spark:<><path d="m12 3 1.5 5.5L19 10l-5.5 1.5L12 17l-1.5-5.5L5 10l5.5-1.5L12 3Z"/><path d="m19 16 .7 2.3L22 19l-2.3.7L19 22l-.7-2.3L16 19l2.3-.7L19 16Z"/></>, grid:<><rect x="3.5" y="3.5" width="7" height="7" rx="2"/><rect x="13.5" y="3.5" width="7" height="7" rx="2"/><rect x="3.5" y="13.5" width="7" height="7" rx="2"/><rect x="13.5" y="13.5" width="7" height="7" rx="2"/></>,
  bell:<><path d="M18.5 9a6.5 6.5 0 1 0-13 0c0 7-2.5 7-2.5 8.5h18C21 16 18.5 16 18.5 9Z"/><path d="M9.5 20a3 3 0 0 0 5 0"/></>,
  user:<><circle cx="12" cy="8" r="4"/><path d="M4.5 20c.8-4 3.3-6 7.5-6s6.7 2 7.5 6"/></>, plus:<path d="M12 5v14M5 12h14"/>, check:<path d="m5 12 4 4L19 6"/>,
  download:<><path d="M12 4v11m0 0 4-4m-4 4-4-4M5 20h14"/></>, help:<><circle cx="12" cy="12" r="9"/><path d="M9.7 9a2.5 2.5 0 1 1 3.7 2.2c-.9.5-1.4 1-1.4 2M12 17h.01"/></>,
  logout:<><path d="M10 5H5v14h5M14 8l4 4-4 4M18 12H9"/></>,
};
function Icon({name,size=22}:{name:IconName;size?:number}){return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{iconPaths[name]}</svg>}

const nav:Array<{id:Screen;label:string;icon:IconName}>=[{id:"today",label:"Сегодня",icon:"home"},{id:"schedule",label:"Расписание",icon:"calendar"},{id:"deadlines",label:"Дедлайны",icon:"check"},{id:"grades",label:"Успеваемость",icon:"chart"},{id:"learning",label:"Обучение",icon:"book"},{id:"news",label:"Актуальное",icon:"bell"},{id:"chat",label:"Чаты",icon:"user"},{id:"activities",label:"Активности",icon:"spark"},{id:"campus",label:"Кампус",icon:"home"},{id:"services",label:"Сервисы",icon:"grid"},{id:"requests",label:"Заявки",icon:"file"},{id:"career",label:"Карьера",icon:"grid"},{id:"finance",label:"Финансы",icon:"chart"},{id:"portfolio",label:"Портфолио",icon:"spark"},{id:"support",label:"Поддержка",icon:"help"},{id:"notifications",label:"Уведомления",icon:"bell"}];
const titles:Record<Exclude<Screen,"login">,[string,string]>={today:["Понедельник, 16 ноября","Доброе утро, Анна"],schedule:["Учебная неделя 16–22 ноября","Расписание"],deadlines:["Все важные даты в одном месте","Дедлайны"],grades:["Осенний семестр · 2 курс","Успеваемость"],requests:["Справки и обращения","Мои заявки"],learning:["Курсы и индивидуальная траектория","Обучение"],news:["Новости, конкурсы и объявления","Актуальное"],chat:["Студенты, преподаватели и группы","Чаты"],activities:["События, клубы и возможности","Внеучебная активность"],campus:["Навигация и жизнь в университете","Кампус"],career:["Практика, стажировки и вакансии","Карьера"],finance:["Начисления, выплаты и помощь","Финансы"],portfolio:["Проекты, достижения и документы","Портфолио"],services:["Всё необходимое в одном месте","Сервисы кампуса"],support:["Единое окно помощи","Поддержка"],notifications:["3 новых события","Уведомления"],profile:["Личные данные и настройки","Профиль"]};
const lessons=[{t:"08:45",n:"Web-программирование",d:"Практика · Мария Котова",r:"215",format:"Очно",place:"Кронверкский пр., 49"},{t:"10:30",n:"Базы данных",d:"Лекция · Алексей Петров",r:"304",format:"Очно",place:"Ломоносова, 9",a:true},{t:"12:15",n:"Английский язык",d:"Практика · Елена Волкова",r:"Онлайн",format:"Онлайн",place:"Moodle · видеокомната"}];


const css=String.raw`
/* Request form controls */
.request-form .select,.grades-year.select{appearance:none;-webkit-appearance:none;padding-right:44px;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");background-repeat:no-repeat;background-position:right 16px center;background-size:16px}
.request-form .request-upload{display:flex;align-items:center;gap:12px;flex-wrap:wrap;min-height:52px;padding:11px;border:1px solid var(--line);border-radius:14px;background:#fff}
.request-form .request-upload .secondary{flex:0 0 auto}
.request-form .request-file-name{flex:1;min-width:100px;overflow-wrap:anywhere;font-size:12px;color:var(--muted)}
/* End request form controls */
/* Grade alignment */
.subject-row>.score-ring{justify-self:center}
.subject-row .subject-actions>.status{flex:0 0 88px;white-space:nowrap}
@media(min-width:681px){.subject-table>.subject-row{grid-template-columns:minmax(0,1fr) 64px 136px}}
/* End grade alignment */

*{box-sizing:border-box}body{margin:0}.app,.app *{min-width:0}.app{--bg:#f7f6fb;--surface:#fff;--soft:#fbfaff;--text:#252335;--muted:#77738a;--line:#ece9f4;--violet:#8b7cf6;--violet-dark:#6759d9;--violet-soft:#e8e4ff;--blue:#dceeff;--mint:#daf4e9;--peach:#ffe7da;--yellow:#fff1c9;--shadow:0 12px 36px rgba(64,52,111,.08);--card-shadow:0 8px 22px rgba(64,52,111,.07);min-height:100vh;background:var(--bg);color:var(--text);font-family:Manrope,Inter,ui-sans-serif,system-ui,sans-serif}.app button,.app input,.app select,.app textarea{font:inherit;color:inherit}.shell{display:grid;grid-template-columns:252px minmax(0,1fr);gap:18px;min-height:100vh;padding:18px}.sidebar{position:sticky;top:18px;display:flex;flex-direction:column;height:calc(100vh - 36px);padding:24px 18px 18px;border:1px solid #ffffffea;border-radius:28px;background:#ffffffdc;box-shadow:var(--shadow);backdrop-filter:blur(16px)}.brand{display:flex;align-items:center;gap:12px;padding:0 8px 24px}.brand-mark{display:grid;place-items:center;width:42px;height:42px;flex:0 0 auto;border-radius:15px;color:#fff;font-weight:850;background:linear-gradient(145deg,#a495ff,#7466e6);box-shadow:0 8px 18px #7868e74d}.brand-title{font-size:16px;font-weight:850}.sub,.caption{color:var(--muted);font-size:11px;line-height:1.45;overflow-wrap:anywhere}.nav{display:grid;gap:6px}.nav-btn{display:flex;align-items:center;gap:12px;min-height:48px;padding:0 14px;border:0;border-radius:15px;color:var(--muted);background:transparent;text-align:left;cursor:pointer}.nav-btn:hover{color:var(--text);background:var(--soft)}.nav-btn.active{color:var(--violet-dark);background:var(--violet-soft);font-weight:800}.profile-mini{display:flex;align-items:center;gap:11px;margin-top:auto;padding:12px;border:0;border-radius:18px;background:var(--soft);text-align:left;cursor:pointer}.avatar{display:grid;place-items:center;width:42px;height:42px;flex:0 0 auto;border:0;border-radius:50%;color:#5e50c9;font-size:13px;font-weight:850;background:linear-gradient(145deg,#efeaff,#dcd5ff)}.main{width:min(1220px,100%);margin:0 auto;padding:20px 22px 38px}.topbar{display:flex;align-items:center;justify-content:space-between;gap:20px;margin-bottom:24px}.eyebrow{color:var(--muted);font-size:13px;font-weight:650}.page-title{margin:5px 0 0;font-size:clamp(28px,3vw,40px);line-height:1.1;letter-spacing:-.04em;overflow-wrap:anywhere}.top-actions{display:flex;gap:10px}.icon-btn{position:relative;display:grid;place-items:center;width:46px;height:46px;flex:0 0 auto;border:1px solid var(--line);border-radius:16px;background:#fff;box-shadow:var(--card-shadow);cursor:pointer}.dot{position:absolute;top:8px;right:9px;width:8px;height:8px;border:2px solid #fff;border-radius:50%;background:#ff8c79}.content-grid{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(300px,.85fr);gap:20px;align-items:start}.stack,.column{display:grid;gap:20px}.card{overflow:hidden;padding:22px;border:1px solid #ece9f4e6;border-radius:22px;background:#fff;box-shadow:var(--card-shadow)}.between,.card-head{display:flex;align-items:center;justify-content:space-between;gap:12px}.card-head{margin-bottom:17px}.card-title{margin:0;font-size:17px;line-height:1.3;letter-spacing:-.02em;overflow-wrap:anywhere}.hero{position:relative;overflow:hidden;min-height:252px;padding:26px;border-radius:28px;background:linear-gradient(130deg,#dfd9ff,#ede9ff 46%,#dceeff);box-shadow:var(--shadow)}.hero:before,.hero:after{content:"";position:absolute;border-radius:50%}.hero:before{width:220px;height:220px;right:-46px;top:-76px;background:#ffffff75}.hero:after{width:130px;height:130px;right:105px;bottom:-72px;background:#8fdccd4d}.hero>*{position:relative;z-index:1}.label{color:#6f6787;font-size:13px;font-weight:800}.pill,.status{display:inline-flex;align-items:center;justify-content:center;max-width:100%;min-height:29px;padding:5px 11px;border-radius:999px;font-size:11px;font-weight:850;white-space:normal;text-align:center;overflow-wrap:anywhere}.pill{background:#ffffffb8}.pill:before{content:"";width:7px;height:7px;flex:0 0 auto;margin-right:7px;border-radius:50%;background:#f49a6e}.lesson-time{margin-top:30px;color:#655f79;font-size:14px;font-weight:750}.lesson-title{margin:8px 0 18px;font-size:clamp(25px,3vw,35px);line-height:1.15;letter-spacing:-.035em;overflow-wrap:anywhere}.chips,.filters{display:flex;flex-wrap:wrap;gap:8px}.chip{max-width:100%;padding:8px 11px;border-radius:11px;font-size:12px;font-weight:750;background:#ffffff9e;overflow-wrap:anywhere}.list{display:grid;gap:10px}.lesson-row{display:grid;grid-template-columns:62px minmax(0,1fr) auto;align-items:center;gap:14px;padding:13px;border-radius:16px;background:#faf9fd}.lesson-row.active{background:var(--mint)}.row-name{font-size:13px;font-weight:850;line-height:1.35;overflow-wrap:anywhere}.row-sub{margin-top:3px;color:var(--muted);font-size:11px;line-height:1.4;overflow-wrap:anywhere}.room{padding:7px 9px;border-radius:10px;font-size:11px;font-weight:850;background:#ffffffc4}.quick-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.quick{display:flex;align-items:center;gap:12px;min-height:86px;padding:14px;border:0;border-radius:18px;text-align:left;cursor:pointer}.quick:nth-child(1){background:var(--violet-soft)}.quick:nth-child(2){background:var(--blue)}.quick:nth-child(3){background:var(--peach)}.quick:nth-child(4){background:var(--mint)}.quick-icon{display:grid;place-items:center;width:38px;height:38px;flex:0 0 auto;border-radius:13px;background:#ffffffad}.quick-title{font-size:12px;font-weight:850;line-height:1.3;overflow-wrap:normal;word-break:normal}.quick-sub{margin-top:3px;color:#736e80;font-size:10px;line-height:1.35;overflow-wrap:normal;word-break:normal}.link-btn,.ghost-btn{padding:7px 4px;border:0;color:var(--violet-dark);font-size:12px;font-weight:850;background:transparent;cursor:pointer}.primary,.secondary{display:inline-flex;align-items:center;justify-content:center;gap:8px;min-height:44px;padding:7px 17px;border-radius:14px;font-weight:850;white-space:normal;text-align:center;cursor:pointer}.primary{border:0;color:#fff;background:linear-gradient(135deg,#9283f7,#7567df);box-shadow:0 8px 18px #7567df38}.secondary{border:1px solid var(--line);background:#fff}.status.blue{color:#42637c;background:var(--blue)}.status.mint{color:#477865;background:var(--mint)}.status.gray{color:#6d687a;background:#efedf3}.score{font-size:36px;font-weight:900;letter-spacing:-.05em}.score span{color:var(--muted);font-size:14px;font-weight:750;letter-spacing:0}.bar,.mini-bar{overflow:hidden;height:9px;margin-top:14px;border-radius:99px;background:#eeebf5}.bar-fill,.mini-fill{height:100%;border-radius:inherit;background:linear-gradient(90deg,#9b8cf8,#6fcbb2)}.date{display:grid;place-items:center;width:43px;height:43px;flex:0 0 auto;border-radius:13px;color:#685a3c;font-size:10px;font-weight:850;line-height:1.05;text-align:center;background:var(--yellow)}.date strong{display:block;font-size:16px}.simple-row{display:flex;align-items:center;gap:12px;padding:11px 0;border-top:1px solid var(--line)}.simple-row:first-child{padding-top:0;border-top:0}.row-main{flex:1}.tabs{display:flex;flex-wrap:wrap;gap:7px;padding:5px;border-radius:15px;background:#efedf5}.tab,.filter{min-height:36px;padding:0 14px;border:0;border-radius:11px;color:var(--muted);font-size:12px;font-weight:850;background:transparent;cursor:pointer}.tab.active{color:var(--violet-dark);background:#fff;box-shadow:0 4px 12px #40346f14}.filter{border:1px solid var(--line);background:#fff}.filter.active{color:var(--violet-dark);border-color:#cfc7ff;background:var(--violet-soft)}.toolbar{display:flex;align-items:center;justify-content:space-between;gap:14px;margin-bottom:18px}.week{display:grid;grid-template-columns:repeat(5,minmax(155px,1fr));gap:10px;overflow-x:auto;padding-bottom:4px}.day{padding:12px;border-radius:18px;background:#faf9fd}.day-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:10px;font-weight:900}.day-date{display:grid;place-items:center;width:30px;height:30px;border-radius:10px;background:#fff}.week-lesson{margin-top:8px;padding:11px;border-radius:13px;background:#fff}.week-lesson.changed{background:var(--mint)}.week-time{color:var(--violet-dark);font-size:10px;font-weight:900}.week-name{margin-top:5px;font-size:11px;font-weight:850;line-height:1.35;overflow-wrap:anywhere}.week-room{margin-top:6px;color:var(--muted);font-size:10px}.summary-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.summary{padding:18px;border-radius:20px}.summary:nth-child(1){background:var(--violet-soft)}.summary:nth-child(2){background:var(--blue)}.summary:nth-child(3){background:var(--mint)}.summary-value{font-size:30px;font-weight:900}.subject-table{overflow-x:auto}.subject-row{display:grid;grid-template-columns:minmax(190px,1.5fr) minmax(150px,1fr) minmax(105px,.7fr) 45px;align-items:center;gap:14px;min-width:650px;padding:14px 10px;border-top:1px solid var(--line)}.subject-row:first-child{border-top:0}.subject-score{display:flex;align-items:center;gap:9px}.mini-bar{flex:1;height:7px;margin:0}.mark{display:grid;place-items:center;width:38px;height:38px;border-radius:12px;font-weight:900;background:var(--violet-soft)}.request{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:12px;padding:16px;border:0;border-radius:17px;background:#faf9fd;text-align:left;cursor:pointer}.request:hover{background:#f5f2ff}.request-meta{display:flex;flex-wrap:wrap;gap:7px;margin-top:6px;color:var(--muted);font-size:10px}.course-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.course{padding:19px;border:0;border-radius:21px;background:#fff;box-shadow:var(--card-shadow);text-align:left;cursor:pointer}.course-top{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.course-icon{display:grid;place-items:center;width:44px;height:44px;flex:0 0 auto;border-radius:14px}.course-icon.violet{background:var(--violet-soft)}.course-icon.blue{background:var(--blue)}.course-icon.mint{background:var(--mint)}.course-icon.peach{background:var(--peach)}.course-title{margin:14px 0 4px;font-size:16px;font-weight:900;line-height:1.3;overflow-wrap:anywhere}.task{margin-top:14px;padding:12px;border-radius:14px;background:#faf9fd}.notice{display:grid;grid-template-columns:42px minmax(0,1fr) auto;align-items:start;gap:12px;padding:15px;border:0;border-radius:17px;background:#faf9fd;text-align:left;cursor:pointer}.notice.unread{background:var(--violet-soft)}.notice-icon{display:grid;place-items:center;width:42px;height:42px;border-radius:14px;background:#fff}.notice-time{color:var(--muted);font-size:10px;white-space:nowrap}.profile-grid{display:grid;grid-template-columns:minmax(260px,.75fr) minmax(0,1.25fr);gap:20px}.profile-card{text-align:center}.profile-avatar{display:grid;place-items:center;width:92px;height:92px;margin:0 auto 14px;border-radius:30px;color:#5e50c9;font-size:26px;font-weight:900;background:linear-gradient(145deg,#efeaff,#dcd5ff)}.info-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.info{padding:14px;border-radius:15px;background:#faf9fd}.info-label{color:var(--muted);font-size:10px}.info-value{margin-top:5px;font-size:13px;font-weight:850;overflow-wrap:anywhere}.settings{display:grid;gap:9px;margin-top:16px}.setting{display:flex;align-items:center;gap:12px;width:100%;min-height:52px;padding:8px 14px;border:1px solid var(--line);border-radius:15px;background:#fff;text-align:left;cursor:pointer}.danger{color:#a24f58}.field{display:grid;gap:7px;margin-top:16px}.field label{font-size:11px;font-weight:850}.input,.select,.textarea{width:100%;border:1px solid var(--line);border-radius:14px;background:#faf9fd;outline:0}.input,.select{height:46px;padding:0 13px}.textarea{min-height:94px;padding:13px;resize:vertical}.login{display:grid;place-items:center;min-height:100vh;padding:20px;background:radial-gradient(circle at 20% 10%,#e6e0ff 0,transparent 34%),radial-gradient(circle at 90% 80%,#d9f3ea 0,transparent 30%),var(--bg)}.login-card{width:min(430px,100%);padding:30px;border:1px solid #ffffffea;border-radius:28px;background:#ffffffe8;box-shadow:var(--shadow)}.login-logo{display:grid;place-items:center;width:56px;height:56px;margin-bottom:20px;border-radius:18px;color:#fff;font-size:20px;font-weight:900;background:linear-gradient(145deg,#a495ff,#7466e6)}.login-title{margin:0;font-size:28px;letter-spacing:-.04em}.modal-bg{position:fixed;inset:0;z-index:60;display:grid;place-items:center;padding:18px;background:#24203557;backdrop-filter:blur(6px)}.modal{overflow:auto;width:min(520px,100%);max-height:calc(100vh - 36px);padding:24px;border-radius:24px;background:#fff;box-shadow:0 24px 70px #2523353d}.modal-actions{display:flex;flex-wrap:wrap;justify-content:flex-end;gap:9px;margin-top:20px}.success{display:grid;place-items:center;width:64px;height:64px;margin-bottom:16px;border-radius:20px;color:#477865;background:var(--mint)}.bottom-nav{display:none}.toast{position:fixed;left:50%;bottom:26px;z-index:80;max-width:calc(100vw - 32px);padding:11px 16px;border-radius:14px;color:#fff;font-size:12px;font-weight:800;text-align:center;background:#373246;box-shadow:0 12px 30px #25233538;opacity:0;transform:translate(-50%,18px);pointer-events:none;transition:.22s}.toast.show{opacity:1;transform:translate(-50%,0)}
.nav{flex:1;min-height:0;overflow-y:auto;padding-right:5px;scrollbar-width:thin;scrollbar-color:#d8d2ef transparent}.nav::-webkit-scrollbar{width:5px}.nav::-webkit-scrollbar-thumb{border-radius:99px;background:#d8d2ef}.profile-mini{margin-top:10px}.avatar,.profile-avatar{overflow:hidden}.avatar img,.profile-avatar img{width:100%;height:100%;object-fit:cover}.quick{display:grid;grid-template-columns:38px minmax(0,1fr);align-items:center;min-height:104px;height:auto}.quick-title,.quick-sub{max-width:100%;word-break:normal;overflow-wrap:anywhere}.activity-hero{display:grid;grid-template-columns:minmax(0,1.4fr) minmax(250px,.6fr);gap:18px;padding:26px;border-radius:28px;background:linear-gradient(135deg,#e7e1ff,#dff1ff 54%,#d9f3e8);box-shadow:var(--shadow)}.activity-hero h2{margin:8px 0 10px;font-size:30px;line-height:1.15;letter-spacing:-.035em;overflow-wrap:anywhere}.hero-stat{display:grid;place-items:center;align-content:center;padding:18px;border-radius:22px;background:#ffffff9e;text-align:center}.hero-stat strong{font-size:38px}.event-grid,.service-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.event,.service{overflow:hidden;padding:0;border:0;border-radius:21px;background:#fff;box-shadow:var(--card-shadow);text-align:left;cursor:pointer}.event-cover{display:flex;align-items:flex-end;min-height:125px;padding:16px;background:linear-gradient(135deg,var(--violet-soft),var(--blue))}.event:nth-child(2) .event-cover{background:linear-gradient(135deg,var(--peach),var(--yellow))}.event:nth-child(3) .event-cover{background:linear-gradient(135deg,var(--mint),var(--blue))}.event-date{padding:8px 10px;border-radius:12px;background:#ffffffd6;font-size:11px;font-weight:900}.event-body,.service{padding:17px}.event-title,.service-title{font-size:15px;font-weight:900;line-height:1.35;overflow-wrap:anywhere}.event-meta{display:flex;flex-wrap:wrap;gap:7px;margin-top:10px;color:var(--muted);font-size:10px}.activity-pass{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:14px;padding:15px;border-radius:18px;background:var(--violet-soft)}.activity-level{display:grid;place-items:center;width:52px;height:52px;border-radius:17px;color:#fff;font-weight:900;background:linear-gradient(145deg,#9c8df8,#7162dc)}.service{min-height:155px}.service-icon{display:grid;place-items:center;width:45px;height:45px;margin-bottom:16px;border-radius:15px;background:var(--violet-soft)}.service:nth-child(2n) .service-icon{background:var(--blue)}.service:nth-child(3n) .service-icon{background:var(--mint)}.support-grid{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(280px,.8fr);gap:20px}.support-search{display:flex;gap:9px;margin:16px 0}.support-search .input{flex:1}.faq{width:100%;padding:15px;border:0;border-top:1px solid var(--line);background:transparent;text-align:left;cursor:pointer}.faq:first-child{border-top:0}.faq-answer{margin-top:8px;color:var(--muted);font-size:11px;line-height:1.5}.channel{display:flex;align-items:center;gap:12px;padding:14px;border:0;border-radius:16px;background:#faf9fd;text-align:left;cursor:pointer}.more-bg{position:fixed;inset:0;z-index:55;display:none;align-items:flex-end;padding:12px;background:#24203557}.more-sheet{overflow-y:auto;width:100%;max-height:82vh;padding:18px;border-radius:24px;background:#fff;box-shadow:0 24px 70px #2523353d}.more-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:9px}.more-btn{display:grid;place-items:center;gap:7px;min-height:82px;padding:10px 5px;border:0;border-radius:16px;background:#faf9fd;font-size:10px;font-weight:800;text-align:center;cursor:pointer}
.metric-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}.metric{padding:17px;border-radius:18px;background:#fff;box-shadow:var(--card-shadow)}.metric strong{display:block;margin-bottom:4px;font-size:25px;letter-spacing:-.04em}.news-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.news-card{overflow:hidden;border:0;border-radius:21px;background:#fff;box-shadow:var(--card-shadow);text-align:left;cursor:pointer}.news-cover{min-height:120px;padding:18px;background:linear-gradient(135deg,var(--violet-soft),var(--blue))}.news-card:nth-child(2n) .news-cover{background:linear-gradient(135deg,var(--mint),var(--yellow))}.news-body{padding:17px}.deadline-row{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:14px;padding:15px;border-radius:17px;background:#faf9fd}.deadline-row.urgent{background:#fff1eb}.check-btn{display:grid;place-items:center;width:34px;height:34px;border:1px solid var(--line);border-radius:11px;background:#fff;cursor:pointer}.check-btn.done{color:#fff;border-color:#67bda4;background:#67bda4}.chat-layout{display:grid;grid-template-columns:minmax(240px,.72fr) minmax(0,1.28fr);gap:18px;min-height:620px}.chat-list{padding:10px}.chat-person{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:10px;width:100%;padding:12px;border:0;border-radius:15px;background:transparent;text-align:left;cursor:pointer}.chat-person.active{background:var(--violet-soft)}.mini-avatar{display:grid;place-items:center;width:42px;height:42px;border-radius:14px;color:#5e50c9;font-size:11px;font-weight:900;background:#fff}.unread-count{display:grid;place-items:center;min-width:21px;height:21px;padding:0 6px;border-radius:99px;color:#fff;font-size:9px;background:var(--violet)}.conversation{display:flex;flex-direction:column;padding:0}.chat-head{display:flex;align-items:center;gap:11px;padding:17px;border-bottom:1px solid var(--line)}.messages{display:flex;flex:1;flex-direction:column;gap:10px;padding:18px;background:#fbfaff}.bubble{align-self:flex-start;max-width:min(76%,520px);padding:11px 13px;border-radius:15px 15px 15px 4px;background:#fff;box-shadow:var(--card-shadow);font-size:12px;line-height:1.45}.bubble.mine{align-self:flex-end;border-radius:15px 15px 4px 15px;background:var(--violet-soft)}.chat-compose{display:flex;gap:9px;padding:14px;border-top:1px solid var(--line)}.chat-compose .input{flex:1}.campus-map{position:relative;overflow:hidden;min-height:310px;border-radius:22px;background:linear-gradient(140deg,#e1f2ec,#e6e2ff)}.campus-map:before,.campus-map:after{content:"";position:absolute;inset:18% -10%;border:25px solid #ffffff9c;border-radius:45%;transform:rotate(-12deg)}.campus-map:after{inset:-20% 35%;transform:rotate(33deg)}.map-pin{position:absolute;z-index:2;display:grid;place-items:center;width:48px;height:48px;border:7px solid #fff;border-radius:50%;color:#fff;background:var(--violet);box-shadow:var(--shadow)}.tile-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.tile{padding:17px;border:0;border-radius:18px;background:#fff;box-shadow:var(--card-shadow);text-align:left;cursor:pointer}.timeline{display:grid;gap:12px}.timeline-item{display:grid;grid-template-columns:16px minmax(0,1fr);gap:13px}.timeline-dot{width:14px;height:14px;margin-top:4px;border:4px solid #d8d2ff;border-radius:50%;background:var(--violet)}.portfolio-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.portfolio-card{padding:17px;border:0;border-radius:19px;background:#fff;box-shadow:var(--card-shadow);text-align:left;cursor:pointer}.edit-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.upload-label{display:inline-flex;align-items:center;justify-content:center;min-height:40px;margin-top:10px;padding:8px 13px;border:1px solid var(--line);border-radius:13px;font-size:11px;font-weight:850;cursor:pointer}.upload-label input{display:none}.app{overflow-x:clip}.subject-table{overflow:visible}.subject-row{grid-template-columns:minmax(0,1fr) 64px minmax(120px,auto);min-width:0}.score-ring{display:grid;place-items:center;width:58px;height:58px;padding:6px;border-radius:50%;box-shadow:inset 0 0 0 1px #ffffff66}.score-ring span{display:grid;place-items:center;width:100%;height:100%;border-radius:50%;background:#fff;font-size:15px;font-weight:900}.subject-actions{display:flex;align-items:center;justify-content:flex-end;gap:9px}
.quick-card{container-type:inline-size;container-name:quick-actions}
@container quick-actions (max-width:400px){.quick-grid{grid-template-columns:1fr}}
@media(max-width:1200px) and (min-width:1001px){.quick-grid{grid-template-columns:1fr}.quick{min-height:86px}}
@media(max-width:1000px){.shell{grid-template-columns:88px minmax(0,1fr)}.sidebar{padding-inline:12px}.brand{justify-content:center;padding-inline:0}.brand-copy,.nav-label,.profile-copy{display:none}.nav-btn{justify-content:center;padding:0}.profile-mini{justify-content:center;padding:8px}.content-grid{grid-template-columns:1fr}.right{grid-template-columns:repeat(2,minmax(0,1fr))}.quick-card{grid-column:1/-1}.week{grid-template-columns:repeat(5,180px)}.event-grid,.service-grid,.portfolio-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.metric-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:680px){.shell{display:block;padding:0}.sidebar{display:none}.main{padding:22px 16px 170px;scroll-padding-bottom:170px}.topbar{margin-bottom:19px}.eyebrow{font-size:11px}.page-title{font-size:28px}.top-actions>.avatar{display:none}.icon-btn{width:42px;height:42px;border-radius:14px}.content-grid,.column,.right{display:grid;grid-template-columns:1fr;gap:15px}.hero{min-height:244px;padding:20px;border-radius:24px}.hero .between{align-items:flex-start}.lesson-title{font-size:27px}.card{padding:18px;border-radius:20px}.lesson-row{grid-template-columns:52px minmax(0,1fr);gap:10px}.room{grid-column:2;justify-self:start}.quick-grid{grid-template-columns:1fr}.quick{grid-template-columns:38px minmax(0,1fr);min-height:88px}.quick-icon{margin:0}.quick-card{grid-column:auto}.toolbar{align-items:flex-start;flex-direction:column}.summary-grid,.course-grid,.profile-grid,.info-grid,.event-grid,.service-grid,.support-grid,.activity-hero,.news-grid,.chat-layout,.portfolio-grid,.edit-grid{grid-template-columns:1fr}.metric-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.request{grid-template-columns:1fr}.request .status{justify-self:start}.notice{grid-template-columns:42px minmax(0,1fr)}.notice-time{grid-column:2}.activity-pass{grid-template-columns:auto minmax(0,1fr)}.activity-pass>.status{grid-column:2;justify-self:start}.deadline-row{grid-template-columns:auto minmax(0,1fr)}.deadline-row>.status{grid-column:2;justify-self:start}.chat-layout{min-height:auto}.conversation{min-height:520px}.chat-list{max-height:300px;overflow-y:auto}.tile-grid{grid-template-columns:1fr}.bottom-nav{position:fixed;left:10px;right:10px;bottom:max(10px,env(safe-area-inset-bottom));z-index:40;display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:2px;padding:8px;border:1px solid #ffffffeb;border-radius:22px;background:#fffffff0;box-shadow:0 14px 34px #392f6326;backdrop-filter:blur(18px)}.bottom-btn{display:grid;place-items:center;gap:4px;min-height:54px;overflow:hidden;padding:4px 1px;border:0;border-radius:15px;color:var(--muted);font-size:9px;font-weight:800;background:transparent;cursor:pointer}.bottom-btn span{display:block;max-width:100%;overflow:hidden;font-size:9px;line-height:1.1;text-overflow:ellipsis;white-space:nowrap}.bottom-btn.active{color:var(--violet-dark);background:var(--violet-soft)}.more-bg{display:flex}.toast{bottom:110px}.modal{padding:20px}.modal-actions>*{flex:1}.week{grid-template-columns:repeat(5,165px)}}
@media(max-width:680px){html,body,.app{max-width:100%;overflow-x:hidden}.card-head{flex-wrap:wrap}.week{grid-template-columns:1fr;overflow:visible}.day{width:100%}.subject-row{grid-template-columns:minmax(0,1fr) 62px;gap:10px;min-width:0;padding:15px 0}.subject-main{grid-column:1;grid-row:1}.score-ring{grid-column:2;grid-row:1/3;width:58px;height:58px}.subject-actions{grid-column:1;grid-row:2;justify-content:flex-start}.subject-actions .mark{width:30px;height:30px}.support-search{flex-direction:column}.chat-compose{flex-wrap:wrap}.chat-compose .input{flex:1 1 170px}.chat-compose .primary{flex:1 1 100%}.subject-table{width:100%;overflow:visible}.summary{width:100%}}
`;

function Quick({icon,title,sub,onClick}:{icon:IconName;title:string;sub:string;onClick:()=>void}){return <button className="quick" onClick={onClick}><span className="quick-icon"><Icon name={icon}/></span><div><div className="quick-title">{title}</div><div className="quick-sub">{sub}</div></div></button>}
function Deadline({date,title,sub,peach,month="ноя"}:{date:string;title:string;sub:string;peach?:boolean;month?:string}){return <div className="simple-row"><div className="date" style={peach?{background:"var(--peach)"}:undefined}><span><strong>{date}</strong>{month}</span></div><div className="row-main"><div className="row-name">{title}</div><div className="row-sub">{sub}</div></div></div>}
function LessonList({format="Все занятия",toast,items=lessons,emptyText="Занятий этого формата нет"}:{format?:string;toast?:(s:string)=>void;items?:LessonView[];emptyText?:string}){const list=format==="Все занятия"?items:items.filter(x=>x.format===format);return <div className="list">{list.map(x=><div className={`lesson-row ${x.a?"active":""}`} key={x.id??x.t}><b>{x.t}</b><div><div className="row-name">{x.n}</div><div className="row-sub">{x.d}</div><div className="row-sub">{x.format==="Очно"?`${x.place} · аудитория ${x.r}`:x.place}</div></div>{x.format==="Онлайн"?<button className="room" style={{border:0,cursor:"pointer",color:"var(--violet-dark)"}} onClick={()=>{if(items===lessons){toast?.(`Подключаемся к занятию «${x.n}»`);return}const link=safeLessonLink(x.remoteLessonLink);if(link)window.open(link,"_blank","noopener,noreferrer");else toast?.("Ссылка на занятие недоступна")}}>Войти</button>:<span className="room">{x.r}</span>}</div>)}{!list.length&&<div className="caption" style={{padding:22,textAlign:"center"}}>{emptyText}</div>}</div>}
function RequestList({items,compact=false,open,status="Все",emptyText="В этой категории пока нет заявок"}:{items:RequestItem[];compact?:boolean;open:(id:string)=>void;status?:string;emptyText?:string}){const list=(status==="Все"?items:items.filter(x=>x.status===status)).slice(0,compact?2:undefined);return <div className="list">{list.map(x=><button className="request" key={x.id} onClick={()=>open(x.id)}><div><div className="row-name">{x.title}</div><div className="request-meta"><span>№ {x.displayId??x.id}</span><span>•</span><span>{x.date}</span></div></div><span className={`status ${x.tone}`}>{x.status}</span></button>)}{!list.length&&<div className="caption" style={{padding:18,textAlign:"center"}}>{emptyText}</div>}</div>}
function Info({label,value}:{label:string;value:string}){return <div className="info"><div className="info-label">{label}</div><div className="info-value">{value}</div></div>}
function ScoreRing({value}:{value:number|null}){return <div className="score-ring" style={{background:`conic-gradient(#8879ef ${(value??0)*3.6}deg,#ece9f4 0deg)`}} aria-label={value===null?"Оценка не выставлена":`${value} баллов из 100`}><span>{value??"—"}</span></div>}

function Today({go,toast,onSummary}:{go:(s:Screen)=>void;toast:(message:string)=>void;onSummary:(summary:TodaySummary)=>void}){
const {data,error}=useTodayData();
useEffect(()=>{if(error)toast(error)},[error,toast]);
useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);
const message=error?"Не удалось загрузить данные":data?"Записей пока нет":"Загружаем данные…";
const openRequest=async(id:string)=>{try{const request=await api<ApiAppointment>(`/requests/${id}`);toast(`${request.appointmentTitle} · ${appointmentStatus(request.appointmentStatus)}`)}catch(error){toast(error instanceof Error?error.message:"Не удалось открыть заявку")}};
return <div className="content-grid"><section className="column"><article className="hero"><div className="between"><div className="label">Следующая пара</div><div className="pill">{data?.hero.status||(error?"Данные недоступны":"Загрузка")}</div></div><div className="lesson-time">{data?.hero.when||message}</div><h2 className="lesson-title">{data?.hero.title||(error?"Расписание недоступно":"Загружаем расписание…")}</h2><div className="chips"><span className="chip">{data?.hero.room||"—"}</span><span className="chip">{data?.hero.teacher||"—"}</span><span className="chip">{data?.hero.format||"—"}</span></div></article><section className="card"><div className="card-head"><h3 className="card-title">Расписание на сегодня</h3><button className="link-btn" onClick={()=>go("schedule")}>На неделю →</button></div><LessonList items={data?.lessons??[]} toast={toast} emptyText={data?"Сегодня занятий нет":message}/></section><section className="card"><div className="card-head"><h3 className="card-title">Последние заявки</h3><button className="link-btn" onClick={()=>go("requests")}>Все заявки →</button></div><RequestList items={data?.requests??[]} compact open={openRequest} emptyText={data?"Заявок пока нет":message}/></section></section><aside className="column right"><section className="card quick-card"><div className="card-head"><h3 className="card-title">Быстрые действия</h3></div><div className="quick-grid"><Quick icon="plus" title="Заказать справку" sub="Пара касаний до заявки" onClick={()=>go("requests")}/><Quick icon="calendar" title="Расписание" sub="Сегодня и на неделю" onClick={()=>go("schedule")}/><Quick icon="file" title="Мои документы" sub="Справки и выписки" onClick={()=>go("requests")}/><Quick icon="chart" title="Успеваемость" sub="Баллы и зачёты" onClick={()=>go("grades")}/></div></section><section className="card"><div className="card-head"><h3 className="card-title">Учебный прогресс</h3><span className="status mint">{data?.progressStatus||(error?"Нет данных":"Загрузка")}</span></div><div className="score">{data?.average??"—"} <span>из 100 баллов</span></div><div className="bar"><div className="bar-fill" style={{width:`${data?.progress??0}%`}}/></div><p className="caption">{data?`Закрыто ${data.completed} из ${data.courseCount} дисциплин`:message}</p></section><section className="card"><div className="card-head"><h3 className="card-title">Ближайшие дедлайны</h3><button className="link-btn" onClick={()=>go("deadlines")}>Все →</button></div>{data?.deadlines.length?data.deadlines.map((d,index)=><Deadline key={d.id} date={d.date} month={d.month} title={d.title} sub={d.sub} peach={index===1}/>):<Deadline date="—" month="" title={data?"Ближайших дедлайнов нет":message} sub={data?"Новые сроки появятся после назначения заданий":""}/>}</section></aside></div>}

function Schedule({toast,accountID,onSummary}:{toast:(s:string)=>void;accountID:string;onSummary:(s:TodaySummary)=>void}){const[view,setView]=useState<"week"|"day">("week");const[format,setFormat]=useState("Все занятия");const[date,setDate]=useState(()=>moscowDate());const {data,error}=useScheduleData(accountID,date,view);useEffect(()=>{if(error)toast(error)},[error,toast]);useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);const message=error?"Не удалось загрузить расписание":data?"Нет занятий":"Загружаем расписание…";const move=(days:number)=>setDate(shiftDate(date,days));const period=weekPeriod(date);const selected=view==="week"?`${period.from.split("-").reverse().join(".")} — ${period.to.split("-").reverse().join(".")}`:new Intl.DateTimeFormat("ru-RU",{timeZone:"Europe/Moscow",day:"numeric",month:"long",year:"numeric"}).format(new Date(`${date}T12:00:00Z`));const openLesson=(lesson:ScheduleLesson)=>{if(lesson.canceled){toast(`${lesson.n} · Занятие отменено`);return}if(lesson.format==="Онлайн"){const link=safeLessonLink(lesson.remoteLessonLink);if(link)window.open(link,"_blank","noopener,noreferrer");else toast("Ссылка на занятие недоступна")}else toast(`${lesson.n} · ${lesson.place} · аудитория ${lesson.r}`)};const week=data?.week??[];return <div className="stack"><section className="card"><div className="toolbar"><div className="tabs"><button className={`tab ${view==="week"?"active":""}`} onClick={()=>setView("week")}>Неделя</button><button className={`tab ${view==="day"?"active":""}`} onClick={()=>setView("day")}>День</button></div><div className="filters"><button className="filter" onClick={()=>move(view==="week"?-7:-1)}>←</button><ScheduleDatePicker value={date} onChange={setDate} icon={<Icon name="calendar" size={17}/>}/><button className="filter" onClick={()=>move(view==="week"?7:1)}>→</button></div></div><div className="caption" style={{marginBottom:14}}>Выбранный период: {selected}</div><div className="filters" style={{marginBottom:16}}>{["Все занятия","Очно","Онлайн"].map(x=><button key={x} className={`filter ${format===x?"active":""}`} onClick={()=>setFormat(x)}>{x}</button>)}</div>{view==="day"?<LessonList format={format} toast={toast} items={data?.lessons??[]} emptyText={data?"Занятий этого формата нет":message}/>:<div className="week">{!data&&<div className="caption">{message}</div>}{week.map(d=>{const items=format==="Все занятия"?d.items:d.items.filter(x=>x.format===format);return <div className="day" key={d.day}><div className="day-head"><span>{d.day}</span><span className="day-date">{d.date}</span></div>{items.map(x=><button className={`week-lesson ${x.changed?"changed":""}`} style={{width:"100%",border:0,textAlign:"left",cursor:"pointer"}} key={x.id} onClick={()=>openLesson(x)}><div className="week-time">{x.t} · {x.canceled?"Отменено":x.format}</div><div className="week-name">{x.n}</div><div className="week-room">{x.canceled?"Занятие отменено":x.format==="Онлайн"?"Подключиться →":`Аудитория ${x.r}`}</div></button>)}{!items.length&&<div className="caption" style={{padding:"10px 0"}}>Нет занятий</div>}</div>})}</div>}</section><section className="card"><h3 className="card-title" style={{marginBottom:14}}>Изменения</h3>{data?.changes.map(change=><div className="simple-row" key={change.id}><div className="date" style={{background:"var(--mint)"}}><Icon name="calendar"/></div><div className="row-main"><div className="row-name">{change.title}</div><div className="row-sub">{change.reason} · {change.when}</div></div><span className="status mint">{change.status}</span></div>)}{!data?.changes.length&&<div className="caption">{data?"Изменений за выбранный период нет":message}</div>}</section></div>}

function Grades({toast,accountID,onSummary}:{toast:(s:string)=>void;accountID:string;onSummary:(summary:TodaySummary)=>void}){
const {data,error}=useGradesData(accountID);
const[busy,setBusy]=useState(false);
useEffect(()=>{if(error)toast(error)},[error,toast]);
useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);
const [transcriptYear,setTranscriptYear]=useState("");
const selectedYear=transcriptYear||String(data?.periods.currentYear??"");
const period=data?gradesForYear(data,selectedYear):null;
const message=error?"Не удалось загрузить успеваемость":data?"За этот год дисциплин пока нет":"Загружаем успеваемость…";
const rows=period?.subjects.length?period.subjects:[{id:"",n:message,t:"",s:null,credit:null,semester:0,status:"—",mark:"—"}];
const download=async()=>{if(!period?.total||busy)return;setBusy(true);try{await downloadFile(`/academic/transcript.pdf?year=${encodeURIComponent(selectedYear)}`,`Успеваемость-${selectedYear==="all"?"все-годы":`${selectedYear}-год`}-${moscowDate()}.pdf`)}catch(error){toast(error instanceof Error?error.message:"Не удалось скачать выписку")}finally{setBusy(false)}};
const openSubject=async(id:string)=>{if(!id||busy)return;setBusy(true);try{const result=await api<{course:{title:string};scores:{score:number|null;credit:boolean|null}[]}>(`/academic/subjects/${id}/results`);const score=result.scores[0];toast(`${result.course.title} · ${score?.score==null?"Оценка не выставлена":`${score.score} баллов из 100 · ${score.credit?"Зачтено":"Не зачтено"}`}`)}catch(error){toast(error instanceof Error?error.message:"Не удалось прочитать результаты")}finally{setBusy(false)}};
return <div className="stack"><div className="summary-grid"><div className="summary"><div className="summary-value">{period?.average??"—"}</div><div className="caption">Средний балл из 100</div></div><div className="summary"><div className="summary-value">{period?`${period.completed}/${period.total}`:"—"}</div><div className="caption">Закрыто дисциплин</div></div><div className="summary"><div className="summary-value">—</div><div className="caption">Средняя посещаемость</div></div></div><section className="card"><div className="card-head"><h3 className="card-title">Дисциплины</h3><div className="filters"><select className="input select grades-year" style={{width:"auto",maxWidth:"100%"}} aria-label="Год обучения" value={selectedYear} disabled={!data||busy} onChange={e=>setTranscriptYear(e.target.value)}>{data?.periods.years.map(year=><option key={year} value={String(year)}>{year}-й год обучения{year===data.periods.currentYear?" (текущий)":""}</option>)}<option value="all">Все годы</option></select><button className="secondary" onClick={download} disabled={!period?.total||busy}><Icon name="download" size={17}/>Скачать выписку</button></div></div><div className="subject-table">{rows.map(x=><button className="subject-row" style={{width:"100%",borderLeft:0,borderRight:0,borderBottom:0,background:"transparent",textAlign:"left",cursor:"pointer"}} key={x.id||"state"} disabled={!x.id||busy} onClick={()=>openSubject(x.id)}><div className="subject-main"><div className="row-name">{x.n}</div><div className="row-sub">{x.t}</div></div><ScoreRing value={x.s}/><div className="subject-actions"><span className={`status ${x.credit===true?"mint":"gray"}`}>{x.status}</span><div className="mark">{x.mark}</div></div></button>)}</div></section></div>}

function Requests({toast,accountID,onSummary}:{toast:(s:string)=>void;accountID:string;onSummary:(s:TodaySummary)=>void}){const[filter,setFilter]=useState("Все");const[modal,setModal]=useState<Modal>(()=>typeof window!=="undefined"&&window.location.hash==="#requests/new"?"newRequest":null);const[selectedID,setSelectedID]=useState("");const[created,setCreated]=useState<RequestRecord|null>(null);const[questionOpen,setQuestionOpen]=useState(()=>typeof window!=="undefined"&&window.location.hash==="#deanery-question");const[revision,setRevision]=useState(0);const {data,error}=useRequestsData(accountID,revision);useEffect(()=>{if(error)toast(error)},[error,toast]);useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);const open=(id:string)=>{setSelectedID(id);setModal("details")};const newRequest=()=>{if(!data){toast(error||"Загружаем справочники…");return}setCreated(null);setModal("newRequest")};const items=data?.items??[];return <><div className="content-grid"><section className="card"><div className="card-head"><h3 className="card-title">История заявок</h3><button className="primary" onClick={newRequest}><Icon name="plus" size={18}/>Новая заявка</button></div><div className="filters" style={{marginBottom:14}}>{["Все","Создана","В обработке","Готова","Отклонена"].map(x=><button key={x} className={`filter ${filter===x?"active":""}`} onClick={()=>setFilter(x)}>{x}</button>)}</div><RequestList items={items} status={filter} open={open} emptyText={error?"Не удалось загрузить заявки":data?"В этой категории пока нет заявок":"Загружаем заявки…"}/></section><aside className="stack"><section className="card" style={{background:"var(--violet-soft)"}}><div className="quick-icon"><Icon name="file"/></div><h3 className="card-title" style={{marginTop:14}}>Нужна справка?</h3><p className="caption">Выберите документ, способ получения и отслеживайте статус онлайн.</p><button className="primary" style={{marginTop:14}} onClick={newRequest}>Заказать справку</button></section><section className="card"><h3 className="card-title" style={{marginBottom:14}}>Помощь</h3><button className="setting" onClick={()=>setQuestionOpen(true)}><Icon name="help"/><span><div className="row-name">Задать вопрос деканату</div><div className="row-sub">Ответ появится в переписке</div></span></button></section></aside></div>{modal&&data&&<RequestModal key={`${modal}/${selectedID}`} modal={modal} close={()=>{setModal(null);setRevision(v=>v+1)}} success={record=>{setCreated(record);setFilter("Все");setRevision(v=>v+1);setModal("success")}} types={data?.types??[]} buildings={data?.buildings??[]} selectedID={selectedID} created={created}/>}{questionOpen&&<div className="modal-bg" onMouseDown={()=>setQuestionOpen(false)}><section className="modal" role="dialog" aria-modal="true" aria-label="Вопрос деканату" onMouseDown={e=>e.stopPropagation()}><div className="card-head"><h2 className="card-title" style={{fontSize:21}}>Вопрос деканату</h2><button className="icon-btn" onClick={()=>setQuestionOpen(false)} aria-label="Закрыть переписку">×</button></div><DeaneryMessages accountID={accountID}/></section></div>}</>}

function Learning({toast,accountID,onSummary}:{toast:(s:string)=>void;accountID:string;onSummary:(s:TodaySummary)=>void}){const[revision,setRevision]=useState(0);const{data,error}=useLearningData(accountID,revision);const[selectedCourse,setSelectedCourse]=useState<string|null>(null);useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);useEffect(()=>{if(error)toast(error)},[error,toast]);const courses=data?.courses??[];const[tab,setTab]=useState<"courses"|"choice"|"physical">(()=>typeof window!=="undefined"&&window.location.hash==="#learning-physical"?"physical":"courses");return <><div className="stack"><div className="tabs" style={{width:"fit-content",flexWrap:"wrap"}}><button className={`tab ${tab==="courses"?"active":""}`} onClick={()=>setTab("courses")}>Мои курсы</button><button className={`tab ${tab==="choice"?"active":""}`} onClick={()=>setTab("choice")}>Выбор дисциплин</button><button className={`tab ${tab==="physical"?"active":""}`} onClick={()=>setTab("physical")}>Физкультура</button></div>{tab==="courses"?<div className="course-grid">{courses.map(x=><button className="course" key={x.id} onClick={()=>setSelectedCourse(x.id)}><div className="course-top"><span className={`course-icon ${x.c}`}><Icon name="book"/></span><span className={`status ${x.credit===true?"mint":"gray"}`}>{x.p===null?"—":`${x.p}/100`}</span></div><div className="course-title">{x.n}</div><div className="row-sub">{x.t}</div><div className="bar" aria-label={x.p===null?"Оценка не выставлена":`Баллы: ${x.p} из 100`}><div className="bar-fill" style={{width:`${x.p??0}%`}}/></div><div className="task"><div className="row-name">{x.task}</div><div className="row-sub">{x.due}</div></div></button>)}{!courses.length&&<div className="caption">{error?"Не удалось загрузить обучение":data?"Назначенных дисциплин пока нет":"Загружаем обучение…"}</div>}</div>:tab==="choice"?<LearningEnrollment onSaved={()=>setRevision(x=>x+1)}/>:<LearningPhysical/>}</div>{selectedCourse&&<LearningCourseDetails key={selectedCourse} courseID={selectedCourse} close={()=>setSelectedCourse(null)}/>}</>}

function Deadlines({toast,accountID,onSummary}:{toast:(s:string)=>void;accountID:string;onSummary:(s:TodaySummary)=>void}){const[filter,setFilter]=useState("Все");const {data,error}=useDeadlinesData(accountID);useEffect(()=>{if(error)toast(error)},[error,toast]);useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);const completedOnly=filter==="Выполнено";const message=completedOnly?"Статусы сдачи хранятся на учебной платформе. Портал пока их не получает.":error?"Не удалось загрузить дедлайны":data?"Здесь пока ничего нет":"Загружаем дедлайны…";const items=data?.items??[];const active=items.filter(x=>!x.pending&&!x.overdue);const shown=completedOnly?[]:filter==="Активные"?active:items;const openAssignment=(url:string|null)=>{if(url)window.open(url,"_blank","noopener,noreferrer");else toast("Ссылка на сдачу ещё не указана")};const exportCalendar=()=>{const result=deadlinesCalendar(shown,accountID);if(!result.count){toast("Нет заданий с указанным сроком для календаря");return}const url=URL.createObjectURL(new Blob([result.content],{type:"text/calendar;charset=utf-8"}));const link=document.createElement("a");link.href=url;link.download=result.filename;link.click();window.setTimeout(()=>URL.revokeObjectURL(url),1000);toast(`Файл календаря скачан · сроков: ${result.count}`)};return <div className="stack"><div className="metric-grid"><div className="metric"><strong>{data?active.length:"—"}</strong><div className="caption">активных задач</div></div><div className="metric"><strong>{data?items.filter(x=>x.urgent).length:"—"}</strong><div className="caption">срочных</div></div><div className="metric"><strong>—</strong><div className="caption">выполнено</div></div><div className="metric"><strong>—</strong><div className="caption">сдано вовремя</div></div></div><section className="card"><div className="card-head"><h3 className="card-title">Все дедлайны</h3><button className="secondary" onClick={exportCalendar} disabled={!data||completedOnly}><Icon name="calendar" size={17}/>В календарь</button></div><div className="filters" style={{marginBottom:15}}>{["Все","Активные","Выполнено"].map(x=><button className={`filter ${filter===x?"active":""}`} key={x} aria-pressed={filter===x} onClick={()=>setFilter(x)}>{x}</button>)}</div><div className="list">{shown.map(x=><div className={`deadline-row ${x.urgent?"urgent":""}`} key={x.id}><button className="check-btn" onClick={()=>openAssignment(x.submissionURL)} aria-label={x.submissionURL?`Открыть задание «${x.title}» на учебной платформе`:"Ссылка на сдачу ещё не указана"} title={x.submissionURL?"Перейти к сдаче задания":"Ссылка на сдачу ещё не указана"}><Icon name="book" size={18}/></button><div><div className="row-name"><button type="button" onClick={()=>openAssignment(x.submissionURL)} style={{font:"inherit",color:"inherit",border:0,padding:0,background:"transparent",textAlign:"left",cursor:"pointer"}} title={x.submissionURL?"Открыть задание на учебной платформе":"Ссылка на сдачу ещё не указана"}>{x.title}</button></div><div className="row-sub">{x.course} · {x.due}</div></div><span className={`status ${x.urgent?"blue":"gray"}`}>{x.left}</span></div>)}{!shown.length&&<div className="caption" role="status" style={{padding:24,textAlign:"center"}}>{message}</div>}</div></section></div>}

function News({accountID,onSummary}:{accountID:string;onSummary:(summary:TodaySummary)=>void}){
  const {data,error,reload}=useNewsData(accountID);
  const[filter,setFilter]=useState<NewsCategory|"all">("all");
  const[selected,setSelected]=useState<number|null>(()=>typeof window==="undefined"?null:newsIDFromHash(window.location.hash));
  const dialog=useRef<HTMLDialogElement>(null);
  const articleQuery=useCachedQuery(selected!==null?`news-item:${selected}`:null,()=>api<NewsArticle>(`/news/${selected}`));
  const article=articleQuery.data??data?.items.find(item=>item.newsID===selected);
  useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);
  useEffect(()=>{
    if(selected!==null&&!dialog.current?.open)dialog.current?.showModal();
  },[selected]);
  function changeArticle(id:number|null){
    setSelected(id);
    window.history.replaceState(null,"",window.location.pathname+window.location.search+(id===null?"#news":`#news/${id}`));
  }
  const items=data?.items??[];
  const categories=Object.entries(newsLabels) as [NewsCategory,string][];
  const shown=filter==="all"?items:items.filter(item=>item.newsCategory===filter);
  return <div className="stack">
    <div className="filters">
      <button type="button" className={`filter ${filter==="all"?"active":""}`} onClick={()=>setFilter("all")}>Все</button>
      {categories.map(([key,label])=><button type="button" className={`filter ${filter===key?"active":""}`} key={key} onClick={()=>setFilter(key)}>{label}</button>)}
    </div>
    <div className="news-grid">{shown.map(item=><button className="news-card" key={item.newsID} onClick={()=>changeArticle(item.newsID)}>
      <div className="news-cover"><span className="status blue">{item.newsCategory?newsLabels[item.newsCategory]:"Новости"}</span></div>
      <div className="news-body"><div className="event-title">{item.newsTitle}</div><p className="caption">{newsExcerpt(item.newsDescription)}</p><span className="link-btn" style={{padding:0}}>Подробнее →</span></div>
    </button>)}</div>
    {!shown.length&&<div className="card caption" role={error?"alert":"status"}>{error||(!data?"Загружаем публикации…":filter==="all"?"Публикаций пока нет":"В этой категории пока нет публикаций")}{error&&<button className="secondary" onClick={()=>void reload()}>Повторить</button>}</div>}
    <dialog ref={dialog} className="modal" aria-labelledby="news-article-title" onClose={()=>changeArticle(null)}
      style={{maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",border:0,color:"inherit"}}>
      <div className="card-head"><h2 className="card-title" id="news-article-title">{article?.newsTitle||"Публикация"}</h2><button className="icon-btn" aria-label="Закрыть публикацию" onClick={()=>dialog.current?.close()}>×</button></div>
      {articleQuery.error?<p className="caption" role="alert">{articleQuery.error}</p>:article?<>
        {article.createdAt&&<p className="caption">{requestDate(article.createdAt)}</p>}
        <p className="row-sub" style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{article.newsDescription}</p>
      </>:<p className="caption" role="status">Загружаем публикацию…</p>}
    </dialog>
  </div>;
}

function Chat({accountID}:{accountID:string}){
  return <ChatPanel accountID={accountID} icons={{add:<Icon name="plus" size={18}/>,info:<Icon name="help" size={18}/>}}/>;
}

function Activities({openSport,accountID,onSummary}:{openSport:()=>void;accountID:string;onSummary:(summary:TodaySummary)=>void}){
  const query=useActivitiesData(accountID);
  const {data,error}=query;
  useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);
  const [filter,setFilter]=useState<ActivityCategory|"all">("all");
  const [recommended,setRecommended]=useState(false);
  const [full,setFull]=useState(false);
  const [detailID,setDetailID]=useState("");
  const [busy,setBusy]=useState(false);
  const [notice,setNotice]=useState("");
  const dialog=useRef<HTMLDialogElement>(null);
  const events=data?.events??[];
  const tickets=data?.myTickets??[];
  const detail=events.find(event=>event.eventID===detailID)??tickets.find(ticket=>ticket.eventID===detailID);
  const displayed=events.filter(event=>(filter==="all"||event.category===filter)&&(!recommended||(!event.registered&&event.remaining>0&&(!data?.interests.length||data.interests.includes(event.category)))));
  const visible=full?displayed:displayed.slice(0,6);
  
  const date=new Intl.DateTimeFormat("ru-RU",{timeZone:"Europe/Moscow",day:"numeric",month:"short"});
  const time=new Intl.DateTimeFormat("ru-RU",{timeZone:"Europe/Moscow",hour:"2-digit",minute:"2-digit"});
  const when=(event:CampusEvent)=>date.format(new Date(event.startAt))+" · "+time.format(new Date(event.startAt))+" МСК";
  function show(event:CampusEvent){
    setDetailID(event.eventID);setNotice("");dialog.current?.showModal();
  }
  async function register(event:CampusEvent){
    const registered=!event.registered;
    setBusy(true);setNotice("");
    try{
      await api(`/activities/${event.eventID}/registration`,"PUT",{registered});
      if(data)query.update({
        ...data,
        events:data.events.map(item=>item.eventID===event.eventID?{...item,registered,remaining:Math.max(0,item.remaining+(registered?-1:1))}:item),
        registrations:Math.max(0,data.registrations+(registered?1:-1)),
        myTickets:registered?[{...event,registered:true,remaining:Math.max(0,event.remaining-1)},...data.myTickets.filter(ticket=>ticket.eventID!==event.eventID)]:data.myTickets.filter(ticket=>ticket.eventID!==event.eventID),
      });
      void query.reload();
      setNotice(registered?"Вы записаны":"Запись отменена");
    }catch(err){setNotice(err instanceof Error?err.message:"Не удалось сохранить запись")}
    finally{setBusy(false)}
  }
  return <div className="stack activities-view">
    <section className="activity-hero"><div><div className="label">Твоя жизнь вне занятий</div><h2>Находи людей, события и возможности</h2><p className="caption" style={{maxWidth:590}}>Единая афиша кампуса: клубы, спорт, волонтёрство, конкурсы и карьерные мероприятия — с записью в пару касаний.</p><button className="primary" style={{marginTop:14}} aria-pressed={recommended} onClick={()=>{setRecommended(value=>!value);setFilter("all")}}>{recommended?"Открыть всю афишу":"Подобрать для меня"}</button></div><div className="hero-stat"><strong>{data?.weekCount??"—"}</strong><div className="row-name">событий на неделе</div><div className="caption">{data? `Мои предстоящие записи: ${data.registrations}`:""}</div></div></section>
    <section className="card"><div className="card-head"><h3 className="card-title">{recommended?"Подборка событий":"Ближайшие события"}</h3><button className="link-btn" onClick={()=>setFull(value=>!value)}>{full?"Ближайшие события":"Весь календарь →"}</button></div>
      <div className="filters" style={{marginBottom:15}}>{(["all","community","career","volunteering","sport"] as const).map(key=><button key={key} className={`filter ${filter===key?"active":""}`} onClick={()=>setFilter(key)}>{key==="all"?"Все":activityLabels[key]}</button>)}</div>
      <div className="event-grid">{visible.map(event=><button className="event" key={event.eventID} onClick={()=>show(event)}>
        <div className="event-cover"><span className="event-date">{date.format(new Date(event.startAt)).toLocaleUpperCase("ru")}</span></div>
        <div className="event-body"><div className="event-title">{event.title}</div><div className="event-meta"><span>{event.building} · {time.format(new Date(event.startAt))}</span><span>·</span><span>{activityLabels[event.category]}</span></div>
          <span className={`status ${event.registered?"blue":event.remaining?"mint":"gray"}`} style={{marginTop:12}}>{event.registered?"Вы записаны":event.remaining?`Мест: ${event.remaining}`:"Мест нет"}</span></div>
      </button>)}</div>
      {!visible.length&&<p className="caption" role={error?"alert":"status"}>{error||(!data?"Загружаем афишу…":"По выбранному фильтру пока нет событий")}</p>}
    </section>
    <section className="content-grid"><div className="card"><div className="card-head"><h3 className="card-title">Паспорт активности</h3><span className="status mint">Посещения</span></div>
      <div className="activity-pass"><div className="activity-level">{data?.visits??"—"}</div><div><div className="row-name">Посещённые события</div><div className="row-sub">Учитываются подтверждённые посещения</div></div><span className="status blue">{data?Math.floor(data.volunteerMinutes/60):"—"} ч волонтёрства</span></div>
      <div className="simple-row"><div className="date" style={{background:"var(--mint)"}}><Icon name="spark"/></div><div className="row-main"><div className="row-name">Мои записи</div><div className="row-sub">{data?`Предстоящих записей: ${data.registrations}`:"Загружаем…"}</div></div></div></div>
      <aside className="card"><div className="card-head"><h3 className="card-title">Мои сообщества</h3></div>{tickets.map(event=><button key={event.eventID} type="button" className="simple-row" onClick={()=>show(event)} style={{background:"transparent",border:0,textAlign:"left"}}><span className="service-icon" style={{margin:0}}><Icon name="grid"/></span><div className="row-main"><div className="row-name">{event.title}</div><div className="row-sub">{when(event)} · {event.attended?"Посещение подтверждено":"QR-билет"}</div></div></button>)}{!tickets.length&&<p className="caption">{data?"После записи здесь появится билет":"Загружаем…"}</p>}</aside>
    </section>
    <dialog ref={dialog} className="modal" style={{maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",border:0,color:"inherit"}} onClose={()=>setDetailID("")}>
      {detail?<><div className="card-head"><h2 className="card-title">{detail.title}</h2><button className="icon-btn" aria-label="Закрыть" onClick={()=>dialog.current?.close()}>×</button></div>
        <p className="caption">{when(detail)} · {detail.building}</p>{detail.description&&<p className="caption" style={{whiteSpace:"pre-wrap"}}>{detail.description}</p>}
        {detail.registered&&detail.category!=="sport"&&<ActivityTicket key={detail.eventID} eventID={detail.eventID} attended={tickets.find(ticket=>ticket.eventID===detail.eventID)?.attended}/>}
        <p className="caption">Свободных мест: {detail.remaining}</p>{notice&&<p role="status" className="caption">{notice}</p>}
        <div className="modal-actions">{detail.category==="sport"?<button className="primary" onClick={openSport}>К календарю физкультуры</button>:
          <>{events.some(e=>e.eventID===detail.eventID)&&<button className={detail.registered?"secondary":"primary"} disabled={busy||(!detail.registered&&!detail.remaining)} onClick={()=>void register(detail)}>{detail.registered?"Отменить запись":"Записаться"}</button>}</>}</div>
      </>:<><p className="caption">Событие больше недоступно в афише.</p><button className="secondary" onClick={()=>dialog.current?.close()}>Закрыть</button></>}
    </dialog>
  </div>;
}

function Services({go,askDean,requestDocument}:{go:(screen:Screen)=>void;askDean:()=>void;requestDocument:()=>void}){
  const [search,setSearch]=useState("");
  const [showQR,setShowQR]=useState(false);
  const [showDocuments,setShowDocuments]=useState(false);
  const [campusService,setCampusService]=useState<CampusService|null>(null);
  const documents=useRef<HTMLDialogElement>(null);
  const serviceDialog=useRef<HTMLDialogElement>(null);
  useEffect(()=>{if(campusService&&!serviceDialog.current?.open)serviceDialog.current?.showModal()},[campusService]);
  useEffect(()=>{if(showDocuments&&!documents.current?.open)documents.current?.showModal()},[showDocuments]);
  const items:Array<{icon:IconName;title:string;subtitle:string;open?:()=>void}>=[
    {icon:"user",title:"Цифровой пропуск",subtitle:"QR-визитка студента. Для входа нужен отдельный пропуск",open:()=>setShowQR(true)},
    {icon:"book",title:"Библиотека",subtitle:"Научная библиотека МГИМО",open:()=>window.open("https://mgimo.ru/library/scientific-library/","_blank","noopener,noreferrer")},
    {icon:"file",title:"Документы",subtitle:"Справки и официальные выписки",open:()=>setShowDocuments(true)},
    {icon:"home",title:"Общежитие",subtitle:"Задать вопрос деканату",open:askDean},
    {icon:"calendar",title:"Карта кампуса",subtitle:"Корпуса, аудитории и маршруты",open:()=>go("campus")},
    {icon:"grid",title:"Оплата услуг",subtitle:"Квитанции и история платежей"},
    {icon:"spark",title:campusServiceTitles.benefits,subtitle:"Условия и предложения от деканата",open:()=>setCampusService("benefits")},
    {icon:"help",title:campusServiceTitles["lost-reports"],subtitle:"Сообщить о пропаже или находке",open:()=>setCampusService("lost-reports")},
    {icon:"chart",title:campusServiceTitles["resource-slots"],subtitle:"Печать, техника и бронирования",open:()=>setCampusService("resource-slots")},
  ];
  const normalize=(text:string)=>text.toLowerCase().replaceAll("ё","е");
  const query=normalize(search.trim()).split(/\s+/).filter(Boolean);
  const matches=(text:string)=>query.every(word=>normalize(text).includes(word));
  const visible=items.filter(({title,subtitle})=>matches(`${title} ${subtitle}`));
  const sections=[...nav,{id:"profile" as const,label:"Профиль",icon:"user" as const}];
  const keywords:Partial<Record<Screen,string>>={grades:"оценки баллы аттестация",schedule:"пары календарь",
    learning:"курсы дисциплины спорт физкультура физра",chat:"переписка сообщения онлайн занятия",
    news:"объявления новости",profile:"настройки и данные аватар зачётная книжка",requests:"заявления справки"};
  const foundSections=query.length?sections.filter(({id,label})=>matches(label+" "+(keywords[id]??""))):[];
  return <div className="stack">
    <section className="card"><div className="support-search" style={{margin:0}}>
      <input className="input" aria-label="Найти сервис или раздел" value={search} onChange={event=>setSearch(event.target.value)} placeholder="Найти сервис или раздел"/>
      <button className="secondary" onClick={()=>setSearch("")}>Сбросить</button>
    </div></section>
    <div className="service-grid">{visible.map(({icon,title,subtitle,open})=>
      <button type="button" className="service" key={title} onClick={open} disabled={!open}>
        <span className="service-icon"><Icon name={icon}/></span>
        <div className="service-title">{title}</div><div className="row-sub">{subtitle}</div>
        <div className="link-btn" style={{marginTop:10,padding:0}}>{open?"Открыть →":"Пока недоступно"}</div>
      </button>
    )}</div>
    {foundSections.length>0&&<section className="card">
      <h3 className="card-title">Разделы сайта</h3>
      <div className="list" style={{marginTop:14}}>
        {foundSections.map(({id,label,icon})=><button type="button" className="setting" key={id} onClick={()=>id==="services"?setSearch(""):go(id)}>
          <Icon name={icon}/><span className="row-name">{label}</span>
        </button>)}
      </div>
    </section>}
    {!visible.length&&!foundSections.length&&<section className="card" style={{textAlign:"center"}}><h3 className="card-title">Ничего не найдено</h3>
      <p className="caption">Попробуйте изменить запрос.</p><button className="secondary" onClick={()=>setSearch("")}>Показать все сервисы</button>
    </section>}
    {showQR&&<ProfileQRDialog close={()=>setShowQR(false)}/>}
    <dialog ref={serviceDialog} className="modal" aria-labelledby="campus-service-title" onClose={()=>setCampusService(null)}
      style={{border:0,maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",color:"inherit"}}>
      {campusService&&<>
        <div className="card-head"><h2 className="card-title" id="campus-service-title">{campusServiceTitles[campusService]}</h2>
          <button type="button" className="icon-btn" aria-label="Закрыть сервис" onClick={()=>serviceDialog.current?.close()}>×</button>
        </div>
        <CampusServiceContent key={campusService} service={campusService}/>
      </>}
    </dialog>
    <dialog ref={documents} className="modal" aria-labelledby="service-documents-title" onClose={()=>setShowDocuments(false)} style={{border:0,maxWidth:"calc(100vw - 32px)",color:"inherit"}}>
      <div className="card-head"><h2 className="card-title" id="service-documents-title">Документы</h2>
        <button type="button" className="icon-btn" aria-label="Закрыть документы" onClick={()=>documents.current?.close()}>×</button></div>
      <div className="stack">
        <button type="button" className="secondary" onClick={requestDocument}>Запросить справку</button>
        <button type="button" className="secondary" onClick={()=>go("grades")}>Скачать выписку об успеваемости</button>
      </div>
    </dialog>
  </div>;
}

function Campus({accountID,onSummary,askDean}:{accountID:string;onSummary:(summary:TodaySummary)=>void;askDean:()=>void}){
  const {data,error,reload}=useCampusData(accountID);
  const buildings=data?.buildings??[];
  const [mapProvider,setMapProvider]=useState<CampusMapProvider>("yandex");
  const [destinationID,setDestinationID]=useState("");
  const [panel,setPanel]=useState<"rooms"|"dormitory"|null>(null);
  const [roomFilter,setRoomFilter]=useState<RoomFilter|null>(null);
  const [formError,setFormError]=useState("");
  const [roomDraft,setRoomDraft]=useState<RoomFilter>(()=>{
    const start=new Date(Math.ceil(Date.now()/900000)*900000);
    const end=new Date(start.getTime()+60*60*1000);
    const clock=new Intl.DateTimeFormat("ru-RU",{timeZone:"Europe/Moscow",hour:"2-digit",minute:"2-digit",hourCycle:"h23"});
    return{buildingID:"",from:`${moscowDate(start)}T${clock.format(start)}`,to:`${moscowDate(end)}T${clock.format(end)}`};
  });
  const roomPath=panel==="rooms"&&roomFilter?roomQuery(roomFilter):null;
  const roomSearch=useCachedQuery(roomPath,()=>api<RoomResult>(roomPath!));
  const dialog=useRef<HTMLDialogElement>(null);
  const destination=buildings.find(building=>building.buildingID===destinationID)??buildings.find(building=>campusAddress(building))??buildings[0];
  const route=campusRoute(destination,mapProvider);
  useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);
  useEffect(()=>{if(panel&&!dialog.current?.open)dialog.current?.showModal()},[panel]);
  function searchRooms(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    const from=Date.parse(roomDraft.from+":00+03:00"),to=Date.parse(roomDraft.to+":00+03:00");
    if(!Number.isFinite(from)||!Number.isFinite(to)||to<=from||to-from>86400000){
      setFormError("Выберите интервал от начала до окончания длительностью до 24 часов.");return;
    }
    setFormError("");
    if(roomFilter&&roomQuery(roomFilter)===roomQuery(roomDraft))void roomSearch.reload();
    else setRoomFilter({...roomDraft});
  }
  return <div className="content-grid">
    <style>{`.campus-select.select{appearance:none;-webkit-appearance:none;padding-right:44px;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");background-repeat:no-repeat;background-position:right 16px center;background-size:16px}`}</style>
    <section className="stack">
      <div className="campus-map" style={{padding:20}}><div style={{position:"relative",zIndex:2}}>
        <h3 className="card-title">Корпуса</h3>
        <div className="list" style={{marginTop:14}}>
          {buildings.map(building=><button type="button" className="request" key={building.buildingID} onClick={()=>setDestinationID(building.buildingID)}
            aria-pressed={destination?.buildingID===building.buildingID} style={{width:"100%",textAlign:"left"}}>
            <div className="row-main"><div className="row-name">{building.building}</div><div className="row-sub">{campusAddress(building)||"Адрес для маршрута не указан"}</div></div>
            <span className={`status ${destination?.buildingID===building.buildingID?"mint":"gray"}`}>{destination?.buildingID===building.buildingID?"Выбран":"Выбрать"}</span>
          </button>)}
          {!buildings.length&&<p className="caption" role={error?"alert":"status"}>{error||(!data?"Загружаем корпуса…":"Корпуса пока не внесены")}</p>}
          {error&&<button className="secondary" onClick={()=>void reload()}>Повторить</button>}
        </div>
      </div></div>
      <section className="card">
        <div className="card-head"><h3 className="card-title">Построить маршрут</h3><span className="status mint">Пешком</span></div>
        <div className="info-grid">
          <label><span className="info-label">Куда</span><select className="select campus-select" value={destination?.buildingID??""} disabled={!buildings.length} onChange={event=>setDestinationID(event.target.value)}>
            {!buildings.length&&<option value="">Корпусов пока нет</option>}
            {buildings.map(building=><option value={building.buildingID} key={building.buildingID}>{building.building}</option>)}
          </select></label>
          <label><span className="info-label">Открыть в</span><select className="select campus-select" value={mapProvider} onChange={event=>setMapProvider(event.target.value as CampusMapProvider)}>
            <option value="yandex">Яндекс Карты</option>
            <option value="google">Google Maps</option>
            <option value="apple">Apple Maps</option>
            <option value="2gis">2ГИС</option>
          </select></label>
        </div>
        {route&&<p className="caption">Карты определят точку отправления, если им разрешён доступ к местоположению.</p>}
        {route?<a className="primary" href={route} target="_blank" rel="noopener noreferrer" style={{marginTop:15,display:"inline-flex",textDecoration:"none"}}>Показать маршрут</a>:<>
          <button className="primary" style={{marginTop:15}} disabled>Показать маршрут</button>
          <p className="caption">Для маршрута выбери корпус с указанным адресом.</p>
        </>}
      </section>
    </section>
    <aside className="stack">
      <section className="card" style={{background:"var(--violet-soft)"}}>
        <div className="between"><div><div className="caption">Студенческая QR-визитка</div><div className="score" style={{fontSize:24}}>№ {data?.studentNumber??"—"}</div></div><Icon name="user"/></div>
        <p className="caption">Визитка содержит ФИО и номер студента. Для проходной нужен отдельный пропуск.</p>
        <ProfileQR disabled={!data?.studentNumber}/>
      </section>
      <div className="tile-grid">
        <button className="tile" onClick={()=>{setPanel("rooms");setFormError("")}}><span className="service-icon"><Icon name="calendar"/></span><div className="row-name">Свободные аудитории</div><div className="row-sub">Найти место для работы</div></button>
        <button className="tile" onClick={()=>setPanel("dormitory")}><span className="service-icon" style={{background:"var(--mint)"}}><Icon name="home"/></span><div className="row-name">Общежитие</div><div className="row-sub">Уточнить в деканате</div></button>
      </div>
    </aside>
    <dialog ref={dialog} className="modal" aria-labelledby="campus-panel-title" style={{maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",border:0,color:"inherit"}} onClose={()=>setPanel(null)}>
      <div className="card-head"><h2 className="card-title" id="campus-panel-title">{panel==="rooms"?"Свободные аудитории":"Общежитие"}</h2><button className="icon-btn" aria-label="Закрыть" onClick={()=>dialog.current?.close()}>×</button></div>
      {panel==="rooms"?<>
        <form className="stack" onSubmit={searchRooms}>
          <label><span className="info-label">Корпус</span><select className="select campus-select" value={roomDraft.buildingID} onChange={event=>setRoomDraft({...roomDraft,buildingID:event.target.value})}>
            <option value="">Все корпуса</option>{buildings.map(building=><option value={building.buildingID} key={building.buildingID}>{building.building}</option>)}
          </select></label>
          <div className="info-grid">
            <label><span className="info-label">Начало (МСК)</span><input className="input" type="datetime-local" value={roomDraft.from} required onChange={event=>setRoomDraft({...roomDraft,from:event.target.value})}/></label>
            <label><span className="info-label">Окончание (МСК)</span><input className="input" type="datetime-local" value={roomDraft.to} required onChange={event=>setRoomDraft({...roomDraft,to:event.target.value})}/></label>
          </div>
          <div><button className="primary" type="submit">Найти</button></div>
        </form>
        {(formError||roomSearch.error)&&<p className="caption" role="alert">{formError||roomSearch.error}</p>}
        {roomPath&&!roomSearch.ready&&!roomSearch.error&&<p className="caption" role="status">Проверяем расписание…</p>}
        {roomSearch.data&&<div className="list" style={{marginTop:14}}>
          <p className="caption">{roomSearch.data.total?`Свободных по расписанию: ${roomSearch.data.available} из ${roomSearch.data.total}`:"В этом корпусе пока нет аудиторий"}</p>
          {roomSearch.data.items.filter(room=>room.available).map(room=><div className="simple-row" key={room.classroomID}>
            <div className="row-name">Аудитория {room.classroomNumber}</div><div className="row-sub">{room.building}</div>
          </div>)}
        </div>}
      </>:<><p className="caption">Информацию о заселении и статусе заявки можно уточнить в деканате.</p>
        <button className="primary" onClick={()=>{dialog.current?.close();askDean()}}>Задать вопрос деканату</button></>}
    </dialog>
  </div>;
}



function Finance({toast}:{toast:(s:string)=>void}){
  const [visible,setVisible]=useState(true);
  const [panel,setPanel]=useState<"aid"|"benefits"|null>(null);
  const [created,setCreated]=useState<RequestRecord|null>(null);
  const dialog=useRef<HTMLDialogElement>(null);
  const aid=useCachedQuery(panel==="aid"&&!created?"finance-aid-form":null,async()=>{
    const [types,buildings]=await Promise.all([
      allPages<RequestType>("/requests/types?activeOnly=true"),
      allPages<Building>("/buildings"),
    ]);
    return {types:types.items.filter(type=>type.appointmentTypeTitle.trim().toLowerCase()==="материальная помощь"),buildings:buildings.items};
  });
  const showAid=panel==="aid"&&(!!created||!!aid.data?.types.length);
  const showDialog=!!panel&&!showAid;
  useEffect(()=>{
    if(showDialog&&!dialog.current?.open)dialog.current?.showModal();
  },[showDialog]);
  const close=()=>{setPanel(null);setCreated(null)};
  return <div className="stack"><section className="activity-hero"><div><div className="label">Баланс и выплаты</div><h2>{visible?"8 420 ₽":"•••••• ₽"}</h2><p className="caption">Ближайшее начисление стипендии — 25 ноября</p><button className="secondary" style={{marginTop:12}} onClick={()=>setVisible(v=>!v)}>{visible?"Скрыть сумму":"Показать сумму"}</button></div><div className="hero-stat"><strong>+5 000</strong><div className="row-name">стипендия</div><div className="caption">последнее начисление · 25 октября</div></div></section><div className="content-grid"><section className="card"><div className="card-head"><h3 className="card-title">История операций</h3><button className="link-btn" onClick={()=>toast("Выписка скачана")}>Скачать выписку</button></div>{[{n:"Академическая стипендия",d:"25 октября",v:"+5 000 ₽",tone:"mint"},{n:"Печать документов",d:"21 октября",v:"−120 ₽",tone:"gray"},{n:"Оплата общежития",d:"10 октября",v:"−1 580 ₽",tone:"gray"}].map(x=><button className="simple-row" style={{width:"100%",borderLeft:0,borderRight:0,borderBottom:0,background:"transparent",textAlign:"left",cursor:"pointer"}} key={x.n} onClick={()=>toast(`Операция: ${x.n}`)}><span className="service-icon" style={{margin:0}}><Icon name="chart"/></span><span className="row-main"><div className="row-name">{x.n}</div><div className="row-sub">{x.d}</div></span><b>{x.v}</b></button>)}</section><aside className="stack"><section className="card" style={{background:"var(--mint)"}}><h3 className="card-title">Материальная помощь</h3><p className="caption">Проверьте критерии и подайте заявление онлайн.</p><button className="primary" onClick={()=>setPanel("aid")}>Подать заявление</button></section><section className="card"><h3 className="card-title">Льготы</h3><p className="caption">Транспорт, питание, культурные события и предложения партнёров.</p><button className="secondary" onClick={()=>setPanel("benefits")}>Смотреть льготы</button></section></aside></div>
    {showAid&&<RequestModal modal={created?"success":"newRequest"} close={close} success={setCreated}
      types={aid.data?.types??[]} buildings={aid.data?.buildings??[]} selectedID="" created={created} heading="Материальная помощь"/>}
    {showDialog&&<dialog ref={dialog} className="modal" aria-labelledby="finance-dialog-title"
      style={{maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",border:0,color:"inherit"}} onClose={close}>
      <div className="card-head">
        <h2 id="finance-dialog-title" className="card-title">{panel==="benefits"?"Льготы":"Материальная помощь"}</h2>
        <button type="button" className="icon-btn" aria-label="Закрыть" onClick={()=>dialog.current?.close()}>×</button>
      </div>
      {panel==="benefits"?<CampusServiceContent service="benefits"/>:
        <div className="stack">
          <p className="caption" role={aid.error?"alert":"status"}>{aid.error||(aid.ready?"Приём заявлений на материальную помощь пока недоступен.":"Открываем форму заявления…")}</p>
          {aid.error&&<div className="modal-actions"><button className="secondary" onClick={()=>void aid.reload()}>Повторить</button></div>}
        </div>}
    </dialog>}
  </div>;
}



function Support({accountID}:{accountID:string}){
  const articles=useSupportArticles();
  const [open,setOpen]=useState<string|null>(null);
  const [query,setQuery]=useState("");
  const [topic,setTopic]=useState<TicketCategory>("study");
  const [createdID,setCreatedID]=useState("");
  const [sending,setSending]=useState(false);
  const [error,setError]=useState("");
  const [chat,setChat]=useState(false);
  const dialog=useRef<HTMLDialogElement>(null);
  const message=useRef<HTMLTextAreaElement>(null);
  const history=useRef<HTMLDivElement>(null);
  const submitting=useRef(false);
  useEffect(()=>{if(chat&&!dialog.current?.open)dialog.current?.showModal()},[chat]);
  const visible=findFAQ(articles.data??[],query);
  async function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    if(submitting.current)return;
    const form=event.currentTarget;
    const fields=new FormData(form);
    const text=String(fields.get("message")??"").trim();
    if(!text){setError("Опишите вопрос.");return}
    submitting.current=true;setSending(true);setError("");
    try{
      const result=await createSupportTicket(topic,fields.get("priority") as TicketPriority,text);
      form.reset();
      setTopic("study");
      setCreatedID(result.id);
      history.current?.scrollIntoView({block:"start",behavior:"smooth"});
    }catch(err){setError(err instanceof Error?err.message:"Не удалось отправить обращение.")}
    finally{submitting.current=false;setSending(false)}
  }
  return <div className="stack">
    <style>{selectArrowCSS}</style>
    <div className="support-grid">
      <section className="card">
        <h3 className="card-title">Чем помочь?</h3>
        <form className="support-search" onSubmit={event=>{event.preventDefault();setOpen(null)}}>
          <input className="input" aria-label="Поиск по базе знаний" value={query} onChange={event=>{setQuery(event.target.value);setOpen(null)}}
            placeholder="Найти ответ, например «справка»"/>
          <button className="primary" type="submit">Найти</button>
        </form>
        <div>
          {visible.map(item=><button type="button" className="faq" key={item.id} aria-expanded={open===item.id}
            onClick={()=>setOpen(open===item.id?null:item.id)}>
            <div className="between"><span className="row-name">{item.question}</span><b>{open===item.id?"−":"+"}</b></div>
            {open===item.id&&<div className="faq-answer" style={{whiteSpace:"pre-wrap"}}>{item.answer}</div>}
          </button>)}
          {articles.error&&<p className="caption" role="alert">{articles.error}</p>}
          {!articles.error&&!visible.length&&<p className="caption">{articles.data?"Подходящих ответов пока нет.":"Загружаем базу знаний…"}</p>}
        </div>
        <h3 className="card-title" style={{margin:"22px 0 12px"}}>Каналы связи</h3>
        <div className="list">
          <button type="button" className="channel" onClick={()=>setChat(true)}>
            <span className="service-icon" style={{margin:0}}><Icon name="help"/></span>
            <span className="row-main"><div className="row-name">Чат с оператором</div><div className="row-sub">Написать сотруднику поддержки</div></span>
          </button>
          <button type="button" className="channel" disabled={sending} onClick={()=>{setTopic("consultation");message.current?.focus()}}>
            <span className="service-icon" style={{margin:0,background:"var(--blue)"}}><Icon name="calendar"/></span>
            <span className="row-main"><div className="row-name">Очная консультация</div><div className="row-sub">Запросить и согласовать время с оператором</div></span>
          </button>
        </div>
      </section>
      <form className="card" onSubmit={submit}>
        <div className="card-head"><h3 className="card-title">Новое обращение</h3><span className="status blue">Ответ здесь</span></div>
        <p className="caption">{topic==="consultation"?"Укажите вопрос и удобные даты. Оператор предложит время в обращении.":"Опишите вопрос. Сотрудник поддержки ответит в истории обращения."}</p>
        <div className="field"><label htmlFor="support-topic">Тема</label><select className="select spaced-select" id="support-topic" disabled={sending}
          value={topic} onChange={event=>setTopic(event.target.value as TicketCategory)}>
          {Object.entries(ticketCategories).map(([value,label])=><option key={value} value={value}>{label}</option>)}
        </select></div>
        <div className="field"><label htmlFor="support-message">Сообщение</label><textarea className="textarea" id="support-message" ref={message} name="message"
          required maxLength={5000} placeholder="Расскажите, что произошло" disabled={sending}/></div>
        <div className="field"><label htmlFor="support-priority">Приоритет</label><select className="select spaced-select" id="support-priority" name="priority" disabled={sending}>
          <option value="normal">Обычный</option><option value="urgent">Срочный</option>
        </select></div>
        <button className="primary" type="submit" disabled={sending} style={{width:"100%",marginTop:18}}>{sending?"Отправляем…":"Отправить обращение"}</button>
        {error&&<p className="caption" role="alert">{error}</p>}
        {createdID&&<p className="caption" role="status">Обращение отправлено. Переписка открыта ниже.</p>}
        <p className="caption" style={{textAlign:"center"}}>Все ответы и история сохраняются в кабинете</p>
      </form>
    </div>
    <div ref={history}><SupportTickets key={createdID} initialID={createdID}/></div>
    <dialog ref={dialog} className="modal" aria-labelledby="support-chat-title" onClose={()=>setChat(false)}
      style={{border:0,maxWidth:"calc(100vw - 32px)",maxHeight:"calc(100dvh - 32px)",overflowY:"auto",color:"inherit"}}>
      <div className="card-head"><h2 className="card-title" id="support-chat-title">Чат с оператором</h2>
        <button type="button" className="icon-btn" aria-label="Закрыть чат" onClick={()=>dialog.current?.close()}>×</button>
      </div>
      {chat&&<DeaneryMessages accountID={accountID} operator/>}
    </dialog>
  </div>;
}

function Notifications({toast,go,accountID,onSummary}:{toast:(s:string)=>void;go:(s:Screen,newsID?:number)=>void;accountID:string;onSummary:(s:TodaySummary)=>void}){const[revision,setRevision]=useState(0);const[busy,setBusy]=useState(false);const lock=useRef(false);const{data,error}=useNotificationsData(accountID,revision);useEffect(()=>{if(data)onSummary(data.summary)},[data,onSummary]);useEffect(()=>{if(error)toast(error)},[error,toast]);const items=data?.items??[];const read=async(ids:string[],target:Screen|null=null,message="",newsID?:number)=>{if(lock.current)return;lock.current=true;setBusy(true);try{await markNotificationsRead(ids);if(target)go(target,newsID);else toast(message||"Все уведомления прочитаны")}catch(error){toast(error instanceof Error?error.message:"Не удалось обновить уведомления")}finally{setRevision(v=>v+1);lock.current=false;setBusy(false)}};return <section className="card"><div className="card-head"><h3 className="card-title">Все уведомления</h3><button className="link-btn" disabled={busy||!items.some(x=>x.unread)} onClick={()=>void read(items.filter(x=>x.unread).map(x=>x.id))}>Прочитать все</button></div><div className="list">{items.map(x=><button className={`notice ${x.unread?"unread":""}`} key={x.id} disabled={busy} onClick={()=>void read(x.unread?[x.id]:[],x.target,x.d,x.newsID)}><span className="notice-icon"><Icon name={x.i}/></span><span><div className="row-name">{x.t}</div><div className="row-sub">{x.d}</div></span><span className="notice-time">{x.time}</span></button>)}{!items.length&&<div className="caption" style={{padding:22,textAlign:"center"}}>{error?"Не удалось загрузить уведомления":data?"Уведомлений пока нет":"Загружаем уведомления…"}</div>}</div></section>}

function Profile({logout,go,accountID,onSummary,initials}:{
  logout:()=>void;go:(screen:Screen)=>void;accountID:string;onSummary:(summary:TodaySummary)=>void;initials:string;
}){
  const [settings,setSettings]=useState<SettingsPanel|null>(null);
  const {profile,course,error,reload,recordBookNumber}=useProfileData(accountID,onSummary);
  const status=profile?.studentStatus;
  return <div className="profile-grid">
    <section className="card profile-card">
      <ProfileAvatar initials={initials}/>
      <h3 className="card-title">{profile?.studentName||"Профиль студента"}</h3>
      <p className="caption">Учебный профиль · {course}</p>
      {status&&<div style={{display:"flex",justifyContent:"center"}}>
        <span className={`status ${status==="active"?"mint":"gray"}`}>
          {status==="active"?"Обучается":status==="academic_leave"?"Академический отпуск":status==="expelled"?"Отчислен":"Статус не указан"}
        </span>
      </div>}
      <ProfileQR disabled={!profile}/>
    </section>
    <section className="stack">
      <section className="card">
        <div className="card-head"><h3 className="card-title">Личные данные</h3><button className="link-btn" disabled>Изменить</button></div>
        {error?<><div className="status" role="alert" style={{marginBottom:14,color:"#9a4751",background:"#ffe5e7"}}>{error}</div><button className="secondary" onClick={reload}>Повторить</button></>
          :!profile?<div className="caption" role="status">Загружаем профиль…</div>
          :<div className="info-grid">
            <Info label="ФИО" value={profile.studentName||"—"}/>
            <Info label="Электронная почта" value={profile.studentEmail||"—"}/>
            <Info label="Телефон" value={profile.studentPhoneNumber||"—"}/>
            <Info label="Учебная группа" value={String(profile.group?.groupNumber??"—")}/>
            <Info label="Курс" value={course}/>
            <Info label="Номер студента" value={String(profile.studentNumber??"—")}/>
            <Info label="Направление" value={profile.direction?.directionName||"—"}/>
            <Info label="Факультет" value={profile.faculty?.title||"—"}/>
            <Info label="Зачётная книжка" value={recordBookNumber||"Номер ещё не внесён"}/>
          </div>}
        {profile&&<div className="caption" style={{marginTop:10}}>Личные данные доступны для просмотра</div>}
      </section>
      <section className="card">
        <h3 className="card-title">Настройки и данные</h3>
        <div className="settings">
          <button className="setting" onClick={()=>setSettings("notifications")}><Icon name="bell"/><span><div className="row-name">Уведомления</div><div className="row-sub">Расписание, заявки и дедлайны</div></span></button>
          <button className="setting" onClick={()=>setSettings("security")}><Icon name="check"/><span><div className="row-name">Безопасность</div><div className="row-sub">Пароль и активные сеансы</div></span></button>
          <button className="setting" onClick={()=>setSettings("documents")}><Icon name="file"/><span><div className="row-name">Документы и согласия</div><div className="row-sub">Документы университета и мои согласия</div></span></button>
          <button className="setting danger" onClick={logout}><Icon name="logout"/><span className="row-name">Выйти из аккаунта</span></button>
        </div>
      </section>
    </section>
    {settings&&<StudentSettings panel={settings} close={()=>{if(settings==="notifications")reload();setSettings(null)}} openInbox={()=>go("notifications")}/>}
  </div>
}

function Login({login}:{login:(user:User)=>void}){const accounts:Record<UserRole,string>={student:"student1@example.test",teacher:"teacher@example.test",dean:"dean@example.test",support:"support@example.test",admin:"admin@example.test"};const[role,setRole]=useState<UserRole>("student");const[username,setUsername]=useState(accounts.student);const[password,setPassword]=useState("");const[error,setError]=useState("");const[loading,setLoading]=useState(false);const changeRole=(value:UserRole)=>{setRole(value);setUsername(accounts[value]);setError("")};const submit=async(e:FormEvent)=>{e.preventDefault();setError("");if(!username.trim()||!password){setError("Заполните логин и пароль");return}setLoading(true);try{login(await serverLogin(username.trim(),password))}catch(error){setError(error instanceof Error?error.message:"Не удалось войти")}finally{setLoading(false)}};return <div className="login"><form className="login-card" onSubmit={submit}><div className="login-logo">С</div><h1 className="login-title">Вход в кабинет</h1><p className="caption">Выберите учётную запись для входа</p><div className="field"><label>Роль</label><select className="select" value={role} onChange={e=>changeRole(e.target.value as UserRole)}><option value="student">Студент</option><option value="teacher">Преподаватель</option><option value="dean">Деканат</option><option value="support">Сотрудник поддержки</option><option value="admin">Администратор</option></select></div><div className="field"><label>Логин</label><input className="input" value={username} onChange={e=>setUsername(e.target.value)} autoComplete="username"/></div><div className="field"><label>Пароль</label><input className="input" type="password" value={password} onChange={e=>setPassword(e.target.value)} autoComplete="current-password"/></div><div className="caption" style={{marginTop:10}}>Используйте выданный вам пароль</div>{error&&<div className="status" style={{width:"100%",marginTop:14,color:"#9a4751",background:"#ffe5e7"}}>{error}</div>}<button className="primary" disabled={loading} style={{width:"100%",marginTop:20,opacity:loading?.65:1}}>{loading?"Входим…":"Войти"}</button><button type="button" className="link-btn" style={{width:"100%",marginTop:8}} onClick={()=>window.alert("Для восстановления доступа обратитесь в поддержку")}>Не получается войти?</button><div style={{height:1,margin:"15px 0",background:"var(--line)"}}/><button type="button" className="secondary" style={{width:"100%"}} onClick={()=>window.alert("Переход к форме абитуриента будет подключен через серверный маршрут")}>Ты абитуриент? Тебе сюда</button></form></div>}

function RequestModal({modal,close,success,types,buildings,selectedID,created,heading="Новая заявка"}:{modal:Modal;close:()=>void;success:(record:RequestRecord)=>void;types:RequestType[];buildings:Building[];selectedID:string;created:RequestRecord|null;heading?:string}){const[pickup,setPickup]=useState("Электронный документ");const[fileError,setFileError]=useState("");const[typeID,setTypeID]=useState(types[0]?.appointmentTypeID||"");const[buildingID,setBuildingID]=useState(buildings[0]?.buildingID||"");const[comment,setComment]=useState("");const[file,setFile]=useState<File|null>(null);const[busy,setBusy]=useState(false);const[locked,setLocked]=useState(false);const[error,setError]=useState("");const fileInput=useRef<HTMLInputElement>(null);const draftID=useRef("");const sending=useRef(false);const detailQuery=useCachedQuery(modal==="details"&&selectedID?`request:${selectedID}`:null,async()=>{
 const[detail,page]=await Promise.all([api<RequestRecord>(`/requests/${selectedID}`),allPages<RequestHistory>(`/requests/${selectedID}/history`)]);
 return{detail,history:page.items};
 });const detail=detailQuery.data?.detail;const history=detailQuery.data?.history??[];const detailError=error||detailQuery.error;const dismiss=()=>{if(!sending.current)close()};const submit=async(e:FormEvent)=>{e.preventDefault();if(sending.current||fileError)return;const type=types.find(t=>t.appointmentTypeID===typeID);if(!type||pickup==="Забрать в деканате"&&!buildings.some(b=>b.buildingID===buildingID)){setError("Выберите тип обращения и корпус выдачи");return}const validation=requestFileError(file);if(validation){setFileError(validation);return}sending.current=true;setBusy(true);setError("");try{const body={appointmentTypeID:typeID,appointmentTitle:type.appointmentTypeTitle,appointmentDescription:requestDescription(pickup,comment)};if(!draftID.current){const draft=await api<{appointmentDraftID:string}>("/requests/drafts","POST",body);draftID.current=draft.appointmentDraftID}else{const draft=await api<{draftStatus:string}>(`/requests/drafts/${draftID.current}`);if(draft.draftStatus==="submitted"){success(await api<RequestRecord>(`/requests/${draftID.current}`));return}await api(`/requests/drafts/${draftID.current}`,"PATCH",body)}if(file&&!locked){const form=new FormData();form.set("file",file);await api(`/requests/drafts/${draftID.current}/attachment`,"POST",form)}setLocked(true);const record=await submitRequestDraft(draftID.current,pickup==="Забрать в деканате"?buildingID:null);success(record)}catch(error){setError(error instanceof Error?error.message:"Не удалось отправить заявку")}finally{sending.current=false;setBusy(false)}};if(!modal)return null;if(modal==="success")return <div className="modal-bg"><div className="modal"><div className="success"><Icon name="check" size={30}/></div><h2 className="card-title" style={{fontSize:23}}>Заявка создана</h2><p className="caption">Заявке присвоен номер {created?.appoinmentID.slice(0,8)}. Статус доступен в разделе «Мои заявки».</p><div className="modal-actions"><button className="primary" onClick={dismiss}>Понятно</button></div></div></div>;if(modal==="details")return <div className="modal-bg" onMouseDown={dismiss}><div className="modal" onMouseDown={e=>e.stopPropagation()}><div className="card-head"><h2 className="card-title" style={{fontSize:21}}>{detail?.appointmentTitle||(detailError?"Не удалось открыть заявку":"Загружаем заявку…")}</h2><span className={`status ${detail?.appointmentStatus==="ready"?"mint":detail?.appointmentStatus==="canceled"?"gray":"blue"}`}>{detail?appointmentStatus(detail.appointmentStatus):"—"}</span></div><div className="info-grid"><Info label="Номер" value={detail?.appoinmentID||"—"}/><Info label="Дата подачи" value={detail?requestDate(detail.created_at):"—"}/><Info label="Способ получения" value={detail?requestPickup(detail):"—"}/><Info label="Корпус выдачи" value={buildings.find(b=>b.buildingID===detail?.buildingID)?.buildingAddress||buildings.find(b=>b.buildingID===detail?.buildingID)?.building||"Не указан"}/></div><div className="task"><div className="row-name">{detailError||"Ваше обращение"}</div><div className="row-sub" style={{whiteSpace:"pre-wrap"}}>{detail?.appointmentDescription||"—"}</div></div>{history.map(h=><div className="task" key={h.appointmentHistoryID}><div className="row-name">{appointmentStatus(h.appointmentStatus)}</div><div className="row-sub">{requestDate(h.changedAt)}{h.reason?` · ${h.reason}`:""}</div></div>)}{detail?.fileUrlAttachment&&<button className="secondary" onClick={()=>{void downloadFile(`/requests/${detail.appoinmentID}/attachment`,"Вложение"+(/\.[a-z0-9]+$/i.exec(detail.fileUrlAttachment||"")?.[0]||"")).catch(error=>setError(error instanceof Error?error.message:"Не удалось скачать вложение"))}}>Скачать вложение</button>}<div className="modal-actions"><button className="secondary" onClick={dismiss}>Закрыть</button></div></div></div>;return <div className="modal-bg" onMouseDown={dismiss}><form className="modal request-form" onMouseDown={e=>e.stopPropagation()} onSubmit={submit}><div className="card-head"><h2 className="card-title" style={{fontSize:21}}>{heading}</h2><button type="button" className="icon-btn" onClick={dismiss}>×</button></div><div className="field"><label>Тип обращения</label><select className="select" required value={typeID} disabled={busy||locked} onChange={e=>setTypeID(e.target.value)}>{!types.length&&<option value="">Нет доступных типов обращений</option>}{types.map(t=><option key={t.appointmentTypeID} value={t.appointmentTypeID}>{t.appointmentTypeTitle}</option>)}</select></div><div className="field"><label>Способ получения</label><select className="select" value={pickup} disabled={busy||locked} onChange={e=>setPickup(e.target.value)}><option>Электронный документ</option><option>Забрать в деканате</option></select></div>{pickup==="Забрать в деканате"&&<div className="field"><label>Корпус выдачи</label><select className="select" required disabled={busy||locked} value={buildingID} onChange={e=>setBuildingID(e.target.value)}>{!buildings.length&&<option value="">Нет доступных корпусов</option>}{buildings.map(b=><option key={b.buildingID} value={b.buildingID}>{b.buildingAddress||b.building}</option>)}</select></div>}<div className="field"><label>Комментарий</label><textarea className="textarea" placeholder="Опишите дополнительные детали" value={comment} onChange={e=>setComment(e.target.value)} disabled={busy||locked}/></div><div className="field"><label>Вложение</label><div className="request-upload"><button type="button" className="secondary" disabled={busy||locked} onClick={()=>fileInput.current?.click()}>Выбрать файл</button><span className="request-file-name">{file?.name||"Файл не выбран"}</span><input ref={fileInput} hidden type="file" accept=".pdf,.png,.jpg,.jpeg,.docx" disabled={busy||locked} onChange={e=>{const selected=e.currentTarget.files?.[0]||null;setFile(selected);setFileError(requestFileError(selected))}}/></div><div className="caption">PDF, PNG, JPEG или DOCX · до 10 МБ</div>{fileError&&<div className="caption" style={{color:"#a24f58"}}>{fileError}</div>}</div>{error&&<div className="caption" role="alert" style={{color:"#a24f58"}}>{error}</div>}<div className="modal-actions"><button type="button" className="secondary" onClick={dismiss}>Отмена</button><button className="primary" disabled={busy||!!fileError||!types.length}>{busy?"Отправляем…":locked?"Повторить отправку":"Отправить заявку"}</button></div></form></div>}

function OnlineClasses(){
  const [search,setSearch]=useState("");
  const today=moscowDate();
  const until=shiftDate(today,20);
  const {data,error,reload}=useCachedQuery(`online-lessons:${today}`,async()=>{
    const response=await api<{lessons:ApiLesson[]}>(`/schedule/period?from=${today}&to=${until}`);
    return response.lessons.filter(lesson=>lesson.lessonStatus!=="canceled"&&safeLessonLink(lesson.remoteLessonLink)&&Date.parse(lesson.endAt)>Date.now())
      .sort((a,b)=>Date.parse(a.startAt)-Date.parse(b.startAt)||a.lessonID-b.lessonID);
  });
  const next=data?.[0];
  const link=safeLessonLink(next?.remoteLessonLink);
  const displayed=(data??[]).filter(lesson=>`${lesson.course?.title||lesson.topic} ${lesson.teacher?.teacherName||""}`.toLocaleLowerCase("ru").includes(search.trim().toLocaleLowerCase("ru")));
  return <div className="stack">
    <section className="activity-hero"><div><div className="label">Следующая онлайн-пара</div>
      <h2>{next?.course?.title||next?.topic||(data?"Онлайн-пары пока не запланированы":"Загружаем расписание…")}</h2>
      {next&&<><p className="caption">{next.teacher?.teacherName||"Преподаватель не указан"}</p>
      <div className="chips" style={{marginTop:14}}><span className="chip">{requestDate(next.startAt)} · МСК</span></div></>}
      {link&&<div className="filters" style={{marginTop:16}}><a className="primary" style={{textDecoration:"none"}} href={link} target="_blank" rel="noopener noreferrer">Подключиться</a></div>}
      {error&&<p className="caption" role="alert">{error} <button className="secondary" onClick={()=>void reload()}>Повторить</button></p>}
      </div><div className="hero-stat"><Icon name="user" size={34}/><strong style={{fontSize:28}}>{data?.length??"—"}</strong><div className="row-name">онлайн-пар в расписании</div><div className="caption">На ближайшие три недели</div></div>
    </section>
    <section className="card"><div className="card-head"><div><h3 className="card-title">Онлайн-занятия</h3><div className="caption">Пары, для которых опубликована ссылка</div></div>
      <input className="input" style={{width:260,maxWidth:"100%"}} value={search} onChange={e=>setSearch(e.target.value)} placeholder="Найти занятие" aria-label="Поиск онлайн-занятий"/></div>
    <div className="course-grid">{displayed.map(lesson=><a className="course" key={lesson.lessonID} href={safeLessonLink(lesson.remoteLessonLink)||undefined}
      target="_blank" rel="noopener noreferrer" style={{textDecoration:"none",color:"inherit"}}>
      <div className="course-top"><span className="course-icon violet"><Icon name="user"/></span><span className="status blue">Онлайн</span></div>
      <div className="course-title">{lesson.course?.title||lesson.topic}</div><div className="row-sub">{lesson.teacher?.teacherName||"Преподаватель не указан"} · {requestDate(lesson.startAt)} МСК</div>
      <div className="task"><div className="row-name">Перейти на занятие →</div></div></a>)}
      {!displayed.length&&<div className="card caption" style={{textAlign:"center"}}>{data?"Занятия не найдены":"Расписание загружается…"}</div>}</div></section>
    <section className="card"><div className="card-head"><h3 className="card-title">Правила онлайн-занятий</h3><span className="status mint">Учебное расписание</span></div>
      <div className="info-grid"><Info label="Ссылка" value="Публикуется в расписании"/><Info label="Время" value="Московское"/><Info label="Подключение" value="Открывается на платформе преподавателя"/></div>
    </section>
  </div>;
}

function Communication({accountID,onSummary}:{accountID:string;onSummary:(summary:TodaySummary)=>void}){
  const[tab,setTab]=useState<"messages"|"live">(()=>typeof window!=="undefined"&&window.location.hash==="#chats/online"?"live":"messages");
  const {data}=useCachedQuery(`chat-summary:${accountID}`,async()=>{
    const [profile,notices]=await Promise.all([api<ApiProfile>("/profile/me"),allPages<ApiNotification>("/notifications")]);
    return {accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:"Группа не указана",
        hasUnread:notices.items.some(n=>!n.notificationReadAt)};
  });
  useEffect(()=>{if(data)onSummary(data)},[data,onSummary]);
  function choose(value:"messages"|"live"){
    setTab(value);
    window.history.replaceState(null,"",window.location.pathname+window.location.search+(value==="live"?"#chats/online":"#chats"));
  }
  return <div className="stack"><div className="tabs" style={{width:"fit-content",maxWidth:"100%"}}>
    <button className={`tab ${tab==="messages"?"active":""}`} onClick={()=>choose("messages")}>Сообщения</button>
    <button className={`tab ${tab==="live"?"active":""}`} onClick={()=>choose("live")}>Онлайн-занятия</button>
  </div>{tab==="messages"?<Chat accountID={accountID}/>:<OnlineClasses/>}</div>;
}


export default function StudentDashboard(){
  const[screen,setScreen]=useState<Screen>("login");
  const[role,setRole]=useState<UserRole>("student");
  const[user,setUser]=useState<User|null>(null);
  const[todaySummary,setTodaySummary]=useState<TodaySummary|null>(null);
  const[restoring,setRestoring]=useState(true);
  const[toast,setToast]=useState("");
  useEffect(()=>{let live=true;const unsubscribe=onSessionExpired(()=>{if(live){setUser(null);setScreen("login");setRole("student")}});refresh().then(account=>{if(live){setUser(account);setRole(account.role.toLowerCase() as UserRole);setScreen(window.location.hash==="#grades"?"grades":window.location.hash==="#schedule"?"schedule":window.location.hash==="#deadlines"?"deadlines":(window.location.hash==="#requests"||window.location.hash==="#requests/new"||window.location.hash==="#deanery-question")?"requests":window.location.hash==="#notifications"?"notifications":(window.location.hash==="#learning"||window.location.hash==="#learning-physical")?"learning":window.location.hash==="#profile"?"profile":window.location.hash==="#activities"?"activities":window.location.hash==="#campus"?"campus":window.location.hash==="#services"?"services":window.location.hash==="#career"?"career":window.location.hash==="#finance"?"finance":window.location.hash==="#portfolio"?"portfolio":window.location.hash==="#support"?"support":isNewsHash(window.location.hash)?"news":(window.location.hash==="#chats"||window.location.hash==="#chats/online")?"chat":"today")}}).catch(error=>{if(live&&!(error instanceof ApiError&&error.status===401))setToast("Сервер временно недоступен. Повторите вход.")}).finally(()=>{if(live)setRestoring(false)});return()=>{live=false;unsubscribe()}},[]);
  const[moreOpen,setMoreOpen]=useState(false);
  const unreadChats=useUnreadChats(user?.role==="STUDENT"?user.accountID:"");
  useEffect(()=>{if(!toast)return;const t=window.setTimeout(()=>setToast(""),1800);return()=>window.clearTimeout(t)},[toast]);
  const go=(s:Screen,newsID?:number)=>{setScreen(s);setMoreOpen(false);window.history.replaceState(null,"",window.location.pathname+window.location.search+(s==="grades"?"#grades":s==="today"?"#today":s==="schedule"?"#schedule":s==="deadlines"?"#deadlines":s==="requests"?"#requests":s==="notifications"?"#notifications":s==="learning"?"#learning":s==="profile"?"#profile":s==="activities"?"#activities":s==="campus"?"#campus":s==="services"?"#services":s==="career"?"#career":s==="finance"?"#finance":s==="portfolio"?"#portfolio":s==="support"?"#support":s==="chat"?"#chats":s==="news"?(newsID?`#news/${newsID}`:"#news"):""));window.scrollTo({top:0,behavior:"smooth"})};
  const openRequestForm=(hash:"#deanery-question"|"#requests/new")=>{
    go("requests");
    window.history.replaceState(null,"",window.location.pathname+window.location.search+hash);
  };
  const askDean=()=>openRequestForm("#deanery-question");
  const signIn=(account:User)=>{setUser(account);setRole(account.role.toLowerCase() as UserRole);if(account.role!=="STUDENT"){setScreen("today");return}if(account.role==="STUDENT"&&(window.location.hash==="#chats"||window.location.hash==="#chats/online")){setScreen("chat");return}if(account.role==="STUDENT"&&isNewsHash(window.location.hash)){setScreen("news");return}if(account.role==="STUDENT"&&window.location.hash==="#learning-physical"){setScreen("learning");return}if(account.role==="STUDENT"&&(window.location.hash==="#requests/new"||window.location.hash==="#deanery-question")){setScreen("requests");return}go(window.location.hash==="#grades"?"grades":window.location.hash==="#schedule"?"schedule":window.location.hash==="#deadlines"?"deadlines":(window.location.hash==="#requests"||window.location.hash==="#requests/new"||window.location.hash==="#deanery-question")?"requests":window.location.hash==="#notifications"?"notifications":(window.location.hash==="#learning"||window.location.hash==="#learning-physical")?"learning":window.location.hash==="#profile"?"profile":window.location.hash==="#activities"?"activities":window.location.hash==="#campus"?"campus":window.location.hash==="#services"?"services":window.location.hash==="#career"?"career":window.location.hash==="#finance"?"finance":window.location.hash==="#portfolio"?"portfolio":window.location.hash==="#support"?"support":isNewsHash(window.location.hash)?"news":(window.location.hash==="#chats"||window.location.hash==="#chats/online")?"chat":"today")};
  const signOut=async()=>{try{await serverLogout();setUser(null);setRole("student");go("login")}catch(error){setToast(error instanceof Error?error.message:"Не удалось выйти")}};
  if(restoring)return <div className="app"><style>{css}</style><div className="login"><div className="login-card"><div className="login-logo">С</div><p className="caption">Восстанавливаем сессию…</p></div></div></div>;
  if(screen==="login")return <div className="app"><style>{css}</style><Login login={signIn}/></div>;
  if(role!=="student")return <Suspense fallback={<div className="app"><style>{css}</style><div className="login"><p className="caption" role="status">Загружаем кабинет…</p></div></div>}>
    <RoleDashboard role={role} logout={signOut} displayName={user?.displayName} accountID={user?.accountID||""}/>
  </Suspense>;
  const hour=Number(new Intl.DateTimeFormat("en-GB",{timeZone:"Europe/Moscow",hour:"2-digit",hourCycle:"h23"}).format(new Date()));
  const greeting=hour>=5&&hour<12?"Доброе утро":hour>=12&&hour<18?"Добрый день":hour>=18?"Добрый вечер":"Здравствуйте";
  const title=screen==="today"?[new Intl.DateTimeFormat("ru-RU",{timeZone:"Europe/Moscow",weekday:"long",day:"numeric",month:"long"}).format(new Date()),`${greeting}, ${user?.displayName.split(/\s+/)[0]||"студент"}`]:screen==="grades"?["Результаты обучения","Успеваемость"]:screen==="schedule"?["Занятия вашей группы","Расписание"]:titles[screen];
  const initials=user?.displayName.split(/\s+/).slice(0,2).map(part=>part[0]).join("")||"—";
  const summary=todaySummary?.accountID===user?.accountID?todaySummary:null;
  const connectedScreen=screen==="today"||screen==="grades"||screen==="schedule"||screen==="deadlines"||screen==="requests"||screen==="notifications"||screen==="learning"||screen==="profile"||screen==="activities"||screen==="news"||screen==="chat"||screen==="campus"||screen==="career"||screen==="portfolio";
  const mobile=nav.slice(0,4);
  const moreItems=nav.filter(x=>!mobile.some(m=>m.id===x.id)).concat([{id:"profile" as Screen,label:"Профиль",icon:"user" as IconName}]);
  return <div className="app">
    <style>{css}</style>
    <div className="shell">
      <aside className="sidebar">
        <div className="brand"><div className="brand-mark">С</div><div className="brand-copy"><div className="brand-title">Студент</div><div className="sub">учебный кабинет</div></div></div>
        <nav className="nav">{nav.map(x=><button key={x.id} className={`nav-btn ${screen===x.id?"active":""}`} onClick={()=>go(x.id)}><Icon name={x.icon}/><span className="nav-label">{x.label}</span>{x.id==="chat"&&unreadChats&&<ChatBadge/>}</button>)}</nav>
        <button className="profile-mini" onClick={()=>go("profile")}><div className="avatar">{connectedScreen?initials:"АМ"}</div><div className="profile-copy"><div className="brand-title" style={{fontSize:13}}>{user?.displayName||"Студент"}</div><div className="sub">{connectedScreen?(summary?.groupLabel||"Загружаем группу…"):"К3142 · 2 курс"}</div></div></button>
      </aside>
      <main className="main">
        <header className="topbar"><div><div className="eyebrow">{title[0]}</div><h1 className="page-title">{title[1]}</h1></div><div className="top-actions"><button className="icon-btn" aria-label="Уведомления" onClick={()=>go("notifications")}><Icon name="bell"/>{(!connectedScreen||summary?.hasUnread)&&<span className="dot"/>}</button><button className="avatar" aria-label="Профиль" onClick={()=>go("profile")}>{connectedScreen?initials:"АМ"}</button></div></header>
        {screen==="today"&&<Today key={user?.accountID} go={go} toast={setToast} onSummary={setTodaySummary}/>} {screen==="schedule"&&<Schedule key={user?.accountID} toast={setToast} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="deadlines"&&<Deadlines key={user?.accountID} toast={setToast} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="grades"&&<Grades key={user?.accountID} toast={setToast} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="requests"&&<Requests key={user?.accountID} toast={setToast} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="learning"&&<Learning key={user?.accountID} toast={setToast} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="news"&&<News key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="chat"&&<Communication key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="activities"&&<Activities key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary} openSport={()=>{go("learning");window.history.replaceState(null,"",window.location.pathname+window.location.search+"#learning-physical")}}/>} {screen==="campus"&&<Campus key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary} askDean={askDean}/>} {screen==="career"&&<Career key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="finance"&&<Finance toast={setToast}/>} {screen==="portfolio"&&<Portfolio key={user?.accountID} accountID={user?.accountID||""} onSummary={setTodaySummary} addIcon={<Icon name="plus"/>}/>} {screen==="services"&&<Services go={go} askDean={askDean} requestDocument={()=>openRequestForm("#requests/new")}/>} {screen==="support"&&<Support key={user?.accountID} accountID={user?.accountID||""}/>} {screen==="notifications"&&<Notifications key={user?.accountID} toast={setToast} go={go} accountID={user?.accountID||""} onSummary={setTodaySummary}/>} {screen==="profile"&&<Profile key={user?.accountID} logout={signOut} go={go} accountID={user?.accountID||""} onSummary={setTodaySummary} initials={initials}/>} 
      </main>
    </div>
    <nav className="bottom-nav">{mobile.map(x=><button key={x.id} className={`bottom-btn ${screen===x.id?"active":""}`} onClick={()=>go(x.id)}><Icon name={x.icon} size={20}/><span>{x.id==="schedule"?"Пары":x.id==="grades"?"Оценки":x.label}</span></button>)}<button className={`bottom-btn ${moreOpen||moreItems.some(x=>x.id===screen)?"active":""}`} onClick={()=>setMoreOpen(true)}><Icon name="grid" size={20}/><span>Ещё{unreadChats&&<ChatBadge/>}</span></button></nav>
    {moreOpen&&<div className="more-bg" onMouseDown={()=>setMoreOpen(false)}><div className="more-sheet" onMouseDown={e=>e.stopPropagation()}><div className="card-head"><h3 className="card-title">Все разделы</h3><button className="icon-btn" onClick={()=>setMoreOpen(false)}>×</button></div><div className="more-grid">{moreItems.map(x=><button key={x.id} className="more-btn" onClick={()=>go(x.id)}><Icon name={x.icon}/><span>{x.label}{x.id==="chat"&&unreadChats&&<ChatBadge/>}</span></button>)}</div></div></div>}
    <div className={`toast ${toast?"show":""}`}>{toast}</div>
  </div>
}
