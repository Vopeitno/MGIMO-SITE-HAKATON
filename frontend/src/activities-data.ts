import {allPages,api} from '../api';
import type {ApiProfile,ApiNotification,TodaySummary} from './today-model';
import {useCachedQuery} from './useCachedQuery';

export type ActivityCategory = 'community' | 'career' | 'volunteering' | 'sport';
export const activityLabels: Record<ActivityCategory, string> = {
    community: 'Сообщества', career: 'Карьера', volunteering: 'Волонтёрство', sport: 'Спорт',
};
export type CampusEvent = {
    eventID: string; title: string; description: string | null; category: ActivityCategory;
    startAt: string; endAt: string; building: string; remaining: number; registered: boolean; attended?:boolean;
};
export type ActivitiesData = {
    events: CampusEvent[]; myTickets: CampusEvent[]; weekCount: number; visits: number; volunteerMinutes: number; registrations: number; interests: ActivityCategory[];
};
export function useActivitiesData(accountID:string) {
    return useCachedQuery('activities',async()=>{
        const [data,profile,notifications]=await Promise.all([api<ActivitiesData>('/activities'),api<ApiProfile>('/profile/me'),allPages<ApiNotification>('/notifications')]);
        return {...data,summary:{accountID,groupLabel:profile.student?.group?`Группа ${profile.student.group.groupNumber}`:'Группа не указана',hasUnread:notifications.items.some(n=>!n.notificationReadAt)} satisfies TodaySummary};
    });
}
