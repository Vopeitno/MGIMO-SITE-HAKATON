import {useCachedQuery} from './useCachedQuery';
import {allPages,api} from '../api';
import {buildDeadlinesModel} from './deadlines-model';
import type {Deadline} from './deadlines-model';
import type {ApiCourse,ApiProfile,ApiNotification,TodaySummary} from './today-model';
export async function loadDeadlinesData(accountID:string){
 const[deadlines,courses,profile,notifications]=await Promise.all([allPages<Deadline>('/academic/deadlines'),allPages<ApiCourse>('/academic/subjects'),api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications')]);
 return {items:buildDeadlinesModel(deadlines.items,courses.items),summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary};
}
export function useDeadlinesData(accountID:string){
  return useCachedQuery(`deadlines:${accountID}`, () => loadDeadlinesData(accountID), 0);
}
