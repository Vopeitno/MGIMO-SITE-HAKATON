import {useCachedQuery} from './useCachedQuery';
import { allPages, api } from '../api';
import { buildTodayModel, moscowDate } from './today-model';
import type { ApiCourse, ApiDashboard, ApiGrade, ApiLesson, ApiNotification, ApiProfile } from './today-model';

export async function loadTodayData() {
  const [dashboard, schedule, courses, grades, profile, notifications] = await Promise.all([
    api<ApiDashboard>('/dashboard'),
    api<{ lessons: ApiLesson[] }>(`/schedule?date=${moscowDate()}`),
    allPages<ApiCourse>('/academic/subjects'),
    allPages<ApiGrade>('/academic/grades'),
    api<ApiProfile>('/profile/me'),
    allPages<ApiNotification>('/notifications'),
  ]);
  return buildTodayModel(dashboard, schedule.lessons, courses.items, grades.items, new Date(), profile, notifications.items);
}

export function useTodayData() {
  return useCachedQuery('today', () => loadTodayData(), 0);
}
