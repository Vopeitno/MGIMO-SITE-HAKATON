import {useCachedQuery} from './useCachedQuery';
import { allPages, api } from '../api';
import { buildScheduleModel, weekPeriod } from './schedule-model';
import type { LessonChange } from './schedule-model';
import type { ApiLesson, ApiNotification, ApiProfile, TodaySummary } from './today-model';

export async function loadScheduleData(accountID:string,date:string,view:'week'|'day'){
 const {from,to}=weekPeriod(date);
 const path=view==='day'?`/schedule?date=${date}`:`/schedule/period?from=${from}&to=${to}`;
 const [schedule,changes,profile,notifications]=await Promise.all([
  api<{lessons:ApiLesson[]}>(path),allPages<LessonChange>('/schedule/changes'),api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications'),
 ]);
 return {...buildScheduleModel(schedule.lessons,changes.items,date,view),summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary};
}
export function useScheduleData(accountID:string,date:string,view:'week'|'day'){
  return useCachedQuery(`schedule:${accountID}:${date}:${view}`, () => loadScheduleData(accountID,date,view), 0);
}
