import {allPages} from '../api';
import type {Deadline} from './deadlines-model';
import type {ApiCourse} from './today-model';
export type Assignment=Deadline&{courseTitle:string;customSubmissionURL?:string|null};
export async function loadTeacherDeadlines(){
 const[deadlines,courses]=await Promise.all([allPages<Assignment>('/academic/deadlines'),allPages<ApiCourse>('/academic/subjects')]);
 const titles=new Map(courses.items.map(c=>[c.courseID,c.title]));
 return deadlines.items.map(d=>({...d,courseTitle:titles.get(d.courseID||'')||'Дисциплина недоступна'}));
}
