import {useCachedQuery} from './useCachedQuery';
import { allPages, api } from '../api';
import { buildGradesModel } from './grades-model';
import type { CourseGrade, GradeCourse } from './grades-model';
import type { ApiNotification, ApiProfile, TodaySummary } from './today-model';

export async function loadGradesData(accountID: string) {
  const [courses, grades, profile, notifications, periods] = await Promise.all([
    allPages<GradeCourse>('/academic/subjects'),
    allPages<CourseGrade>('/academic/grades'),
    api<ApiProfile>('/profile/me'),
    allPages<ApiNotification>('/notifications'),
    api<{currentYear:number;years:number[]}>('/academic/transcript/periods'),
  ]);
  return {
    ...buildGradesModel(courses.items, grades.items),
    periods,
    summary: {
      accountID,
      groupLabel: profile.student?.group ? `Группа ${profile.student.group.groupNumber}` : 'Группа не указана',
      hasUnread: notifications.items.some(n => !n.notificationReadAt),
    } satisfies TodaySummary,
  };
}
export function useGradesData(accountID: string) {
  return useCachedQuery(`grades:${accountID}`, () => loadGradesData(accountID), 0);
}
