import {useCachedQuery} from './useCachedQuery';
import {allPages,api} from '../api';import {notificationView} from './notifications-model';import type {Notification} from './notifications-model';import type {ApiProfile,TodaySummary} from './today-model';
export async function loadNotificationsData(accountID:string){const[notifications,profile]=await Promise.all([allPages<Notification>('/notifications'),api<ApiProfile>('/profile/me')]);return{records:notifications.items,items:notifications.items.map(notificationView),summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary}}
export async function markNotificationsRead(ids:string[]){const pending=[...ids];const workers=Array.from({length:Math.min(3,pending.length)},async()=>{while(pending.length){const id=pending.shift();if(id)await api(`/notifications/${id}/read`,'PATCH')}});const results=await Promise.allSettled(workers);if(results.some(r=>r.status==='rejected'))throw new Error('Не все уведомления удалось отметить прочитанными. Список обновлён, повторите действие.');}
export function useNotificationsData(accountID:string,revision:number){
  return useCachedQuery(`notifications:${accountID}`, () => loadNotificationsData(accountID), revision);
}
