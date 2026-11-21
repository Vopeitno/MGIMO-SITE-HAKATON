import {useCachedQuery} from './useCachedQuery';
import {allPages,api} from '../api';
import {requestRow} from './requests-model';
import type {Building,RequestRecord,RequestType} from './requests-model';
import type {ApiNotification,ApiProfile,TodaySummary} from './today-model';
export async function loadRequestsData(accountID:string){
 const[requests,types,buildings,profile,notifications]=await Promise.all([allPages<RequestRecord>('/requests'),allPages<RequestType>('/requests/types?activeOnly=true'),allPages<Building>('/buildings'),api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications')]);
 return {records:requests.items,items:requests.items.map(requestRow),types:types.items,buildings:buildings.items,summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary};
}
export function useRequestsData(accountID:string,revision:number){
  return useCachedQuery(`requests:${accountID}`, () => loadRequestsData(accountID), revision);
}
