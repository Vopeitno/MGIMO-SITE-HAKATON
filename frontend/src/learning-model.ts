import {buildGradesModel} from './grades-model';
import type {CourseGrade,GradeCourse} from './grades-model';
import type {Deadline} from './deadlines-model';
import {requestDate} from './requests-model';
export type LearningCourse=GradeCourse&{description?:string|null;hours:number};
export function buildLearningModel(courses:LearningCourse[],grades:CourseGrade[],deadlines:Deadline[],now=new Date()){
 const results=new Map(buildGradesModel(courses,grades).subjects.map(g=>[g.id,g]));
 return courses.slice().sort((a,b)=>a.semester-b.semester||a.title.localeCompare(b.title,'ru')||a.courseID.localeCompare(b.courseID)).map((course,index)=>{
  const grade=results.get(course.courseID)!;
  const dated=deadlines.filter(d=>d.courseID===course.courseID&&d.deadlineEndAt&&Number.isFinite(Date.parse(d.deadlineEndAt))).sort((a,b)=>Date.parse(a.deadlineEndAt!)-Date.parse(b.deadlineEndAt!)||a.deadlineID-b.deadlineID);
  const next=dated.find(d=>Date.parse(d.deadlineEndAt!)>=now.getTime())??dated.at(-1);
  const undated=deadlines.find(d=>d.courseID===course.courseID&&!d.deadlineEndAt);
  return{id:course.courseID,n:course.title,t:grade.t,p:grade.s,credit:grade.credit,c:['violet','blue','mint','peach'][index%4],semester:course.semester,hours:course.hours,task:next?.deadlineTitle||undated?.deadlineTitle||'Задания пока не опубликованы',due:next?`${Date.parse(next.deadlineEndAt!)<now.getTime()?'Срок истёк':'Срок'}: ${requestDate(next.deadlineEndAt)}`:undated?'Срок не указан':'Дедлайнов пока нет'};
 });
}
