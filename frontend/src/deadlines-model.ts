import { moscowDate } from './today-model';
import type { ApiCourse } from './today-model';
export type Deadline = {deadlineID:number;courseID?:string|null;deadlineTitle:string;deadlineDescription:string;submissionURL?:string|null;deadlineStartAt?:string|null;deadlineEndAt?:string|null};
const format=new Intl.DateTimeFormat('ru-RU',{timeZone:'Europe/Moscow',day:'numeric',month:'long',year:'numeric',hour:'2-digit',minute:'2-digit'});
const valid=(value?:string|null)=>Boolean(value&&Number.isFinite(Date.parse(value)));
export function deadlineSubmissionURL(raw?:string|null):string|null{
 if(!raw)return null;
 try{const url=new URL(raw);return url.protocol==='https:'&&!url.username&&!url.password?url.href:null}catch{return null}
}
export function buildDeadlinesModel(deadlines:Deadline[],courses:ApiCourse[],now=new Date()){
 const courseByID=new Map(courses.map(c=>[c.courseID,c.title]));
 return deadlines.slice().sort((a,b)=>(valid(a.deadlineEndAt)?Date.parse(a.deadlineEndAt!):Infinity)-(valid(b.deadlineEndAt)?Date.parse(b.deadlineEndAt!):Infinity)||a.deadlineID-b.deadlineID).map(d=>{
  const end=valid(d.deadlineEndAt)?Date.parse(d.deadlineEndAt!):null;
  const start=valid(d.deadlineStartAt)?Date.parse(d.deadlineStartAt!):null;
  const pending=start!==null&&start>now.getTime();
  const remaining=end===null?null:end-now.getTime();
  const overdue=remaining!==null&&remaining<0;
  const urgent=!pending&&remaining!==null&&remaining<=48*60*60*1000;
  const left=pending?`Начнётся ${format.format(new Date(start!))}`:remaining===null?'Срок не указан':overdue?'Срок истёк':remaining<60*60*1000?`Осталось ${Math.max(1,Math.ceil(remaining/60000))} мин.`:remaining<24*60*60*1000?`Осталось ${Math.ceil(remaining/3600000)} ч.`:`Осталось ${Math.ceil(remaining/86400000)} дн.`;
  return{id:d.deadlineID,submissionURL:deadlineSubmissionURL(d.submissionURL),title:d.deadlineTitle,description:d.deadlineDescription,course:courseByID.get(d.courseID||'')||'Дисциплина недоступна',due:end===null?'Срок не указан':`до ${format.format(new Date(end))}`,urgent,overdue,pending,left,endAt:d.deadlineEndAt};
 });
}
const escape=(value:string)=>Array.from(value.replace(/\\/g,'\\\\').replace(/\r\n|\r|\n/g,'\\n').replace(/;/g,'\\;').replace(/,/g,'\\,')).filter(char=>char.charCodeAt(0)>=32&&char.charCodeAt(0)!==127||char==='\t').join('');
function fold(line:string){const out:string[]=[];let current='';let size=0;for(const char of line){const bytes=new TextEncoder().encode(char).length;if(size+bytes>75){out.push(current);current=' ';size=1}current+=char;size+=bytes}out.push(current);return out.join('\r\n')}
const stamp=(date:Date)=>date.toISOString().replace(/[-:]/g,'').replace(/\.\d{3}Z$/,'Z');
// Export actual due instants as calendar reminders; no submission time is invented.
export function deadlinesCalendar(items:ReturnType<typeof buildDeadlinesModel>,accountID:string,now=new Date()){
 const dated=items.filter(item=>valid(item.endAt));
 const lines=['BEGIN:VCALENDAR','VERSION:2.0','PRODID:-//Digital Deanery//Deadlines//RU','CALSCALE:GREGORIAN'];
 for(const item of dated)lines.push('BEGIN:VEVENT',`UID:deadline-${item.id}-${escape(accountID)}@digital-deanery.invalid`,`DTSTAMP:${stamp(now)}`,`DTSTART:${stamp(new Date(item.endAt!))}`,`SUMMARY:${escape(item.title)}`,`DESCRIPTION:${escape(item.course+'\n'+item.description)}`,'TRANSP:TRANSPARENT','END:VEVENT');
 lines.push('END:VCALENDAR');return{content:lines.map(fold).join('\r\n')+'\r\n',count:dated.length,filename:`Дедлайны-${moscowDate(now)}.ics`};
}
