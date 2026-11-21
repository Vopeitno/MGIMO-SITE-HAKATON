import {useEffect} from 'react';
import {useCachedQuery} from './useCachedQuery';
import {allPages, api} from '../api';
import {moscowDate} from './today-model';
import type {ApiNotification, TodaySummary} from './today-model';

type StudentProfile = {
    studentID: string;
    studentNumber: number;
    studentName: string;
    studentEmail: string;
    studentPhoneNumber: string;
    studentStatus: string;
    studentEnrollmentDate: string;
    group?: {groupNumber: number} | null;
    faculty?: {title: string} | null;
    direction?: {directionName: string} | null;
};

function studyYear(enrollment: string) {
    const date = new Date(enrollment);
    if (!Number.isFinite(date.getTime())) return 'Курс не указан';
    const [year, month] = moscowDate().split('-').map(Number);
    const [enrollmentYear] = moscowDate(date).split('-').map(Number);
    return `${Math.max(1, year - enrollmentYear + (month >= 9 ? 1 : 0))}-й курс`;
}

export function useProfileData(accountID: string, onSummary: (summary: TodaySummary) => void) {
    const query = useCachedQuery(`profile:${accountID}`, async () => {
        const [result, notices] = await Promise.all([
            api<{student?: StudentProfile; recordBookNumber?: string | null}>('/profile/me'),
            allPages<ApiNotification>('/notifications').catch(() => null),
        ]);
        if (!result.student) throw new Error('Данные студента не найдены');
        return {profile: result.student, recordBookNumber: result.recordBookNumber, unread: notices?.items.some(n => !n.notificationReadAt) ?? false};
    });
    const data = query.data;
    useEffect(() => {
        if (!data) return;
        onSummary({accountID, groupLabel: data.profile.group ? `Группа ${data.profile.group.groupNumber}` : 'Группа не указана', hasUnread: data.unread});
    }, [accountID,data,onSummary]);
    return {
        profile: data?.profile,
        course: data ? studyYear(data.profile.studentEnrollmentDate) : '—',
        recordBookNumber: data?.recordBookNumber,
        error: query.error,
        reload: query.reload,
    };
}
