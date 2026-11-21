import { buildTodayModel, moscowDate } from './today-model';
import type { ApiLesson, LessonView } from './today-model';
export type LessonChange = {lessonChangeID:number;lessonID:number;changedAt:string;reason:string;oldStartAt?:string|null;newStartAt?:string|null;oldRoomID?:number|null;newRoomID?:number|null;oldLessonStatus?:string|null;newLessonStatus?:string|null};
export function shiftDate(date:string,days:number){const next=new Date(`${date}T12:00:00Z`);next.setUTCDate(next.getUTCDate()+days);return next.toISOString().slice(0,10)}
export function weekPeriod(date:string){const day=new Date(`${date}T12:00:00Z`).getUTCDay();const from=shiftDate(date,-((day+6)%7));return {from,to:shiftDate(from,6)}}
const dayName=new Intl.DateTimeFormat('ru-RU',{timeZone:'Europe/Moscow',weekday:'short'});
const changeDate=new Intl.DateTimeFormat('ru-RU',{timeZone:'Europe/Moscow',day:'numeric',month:'long',hour:'2-digit',minute:'2-digit'});
export function buildScheduleModel(lessons:ApiLesson[],changes:LessonChange[],date:string,view:'week'|'day',now=new Date()){
 const {from,to}=view==='week'?weekPeriod(date):{from:date,to:date};
 const actual=lessons.filter(l=>Number.isFinite(Date.parse(l.startAt))&&moscowDate(new Date(l.startAt))>=from&&moscowDate(new Date(l.startAt))<=to).sort((a,b)=>Date.parse(a.startAt)-Date.parse(b.startAt)||a.lessonID-b.lessonID);
 const mapped=buildTodayModel({user:{accountID:''},nextLessons:[],appointments:[],deadlines:[]},actual,[],[],now).lessons;
 const byID=new Map(mapped.map(l=>[l.id,l]));
 const week=Array.from({length:7},(_,i)=>{const dayDate=shiftDate(from,i);const dt=new Date(`${dayDate}T12:00:00Z`);const label=dayName.format(dt);return{day:label[0].toUpperCase()+label.slice(1),date:String(dt.getUTCDate()),iso:dayDate,items:actual.filter(l=>moscowDate(new Date(l.startAt))===dayDate).map(l=>({...byID.get(l.lessonID)!,changed:l.lessonStatus==='rescheduled',canceled:l.lessonStatus==='canceled'}))}}).filter((d,i)=>i<5||d.items.length>0);
 const relevant=new Set(actual.map(l=>l.lessonID));
 const inPeriod=(value?:string|null)=>Boolean(value&&Number.isFinite(Date.parse(value))&&moscowDate(new Date(value))>=from&&moscowDate(new Date(value))<=to);
 return {week,lessons:mapped,changes:changes.filter(c=>relevant.has(c.lessonID)||inPeriod(c.oldStartAt)||inPeriod(c.newStartAt)).sort((a,b)=>Date.parse(b.changedAt)-Date.parse(a.changedAt)).map(c=>({id:c.lessonChangeID,title:byID.get(c.lessonID)?.n||`Занятие № ${c.lessonID}`,reason:c.reason,when:Number.isFinite(Date.parse(c.changedAt))?changeDate.format(new Date(c.changedAt)):'Дата не указана',status:'Изменено'}))};
}
export type ScheduleLesson = LessonView & {changed:boolean;canceled:boolean};
