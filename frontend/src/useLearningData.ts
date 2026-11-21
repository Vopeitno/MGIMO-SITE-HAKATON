import {useCachedQuery} from './useCachedQuery';
import {allPages,api} from '../api';
import {buildLearningModel} from './learning-model';
import type {LearningCourse} from './learning-model';
import type {CourseGrade} from './grades-model';
import type {Deadline} from './deadlines-model';
import type {ApiNotification,ApiProfile,TodaySummary} from './today-model';
export type LearningResults={course:LearningCourse;scores:CourseGrade[];exams:{examID:string;examTitle:string;examStartAt:string;examStatus:string}[]};
export async function loadLearningData(accountID:string){
 const[courses,grades,deadlines,profile,notifications]=await Promise.all([allPages<LearningCourse>('/academic/subjects'),allPages<CourseGrade>('/academic/grades'),allPages<Deadline>('/academic/deadlines'),api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications')]);
 return{courses:buildLearningModel(courses.items,grades.items,deadlines.items),summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary};
}
export async function loadLearningDetails(id:string){const[results,deadlines]=await Promise.all([api<LearningResults>(`/academic/subjects/${encodeURIComponent(id)}/results`),allPages<Deadline>(`/academic/deadlines?courseID=${encodeURIComponent(id)}`)]);return{...results,deadlines:deadlines.items}}
export function useLearningData(accountID:string,revision=0){
  return useCachedQuery(`learning:${accountID}`, () => loadLearningData(accountID), revision);
}
