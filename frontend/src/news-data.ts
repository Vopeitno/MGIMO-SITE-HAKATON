import {allPages, api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import type {ApiNotification, ApiProfile, TodaySummary} from './today-model';

export const newsLabels = {
    important: 'Важно',
    opportunities: 'Возможности',
    campus: 'Кампус',
    events: 'События',
};
export type NewsCategory = keyof typeof newsLabels;
export type NewsArticle = {
    newsID: number;
    newsTitle: string;
    newsDescription: string;
    newsCategory?: NewsCategory | null;
    deaneryID?: number | null;
    createdAt: string | null;
    updatedAt: string | null;
};

export function newsIDFromHash(hash: string) {
    const value = Number(/^#news\/(\d{1,10})$/.exec(hash)?.[1]);
    return Number.isInteger(value) && value > 0 && value <= 2147483647 ? value : null;
}

export function isNewsHash(hash: string) {
    return hash === '#news' || newsIDFromHash(hash) !== null;
}

export function newsExcerpt(body: string) {
    const plain = Array.from(body.trim().replace(/\s+/g, ' '));
    return plain.length > 180 ? plain.slice(0, 179).join('').trimEnd() + '…' : plain.join('');
}

export function useNewsData(accountID: string) {
    return useCachedQuery(`news:${accountID}`, async () => {
        const [page, profile, notices] = await Promise.all([
            allPages<NewsArticle>('/news'),
            api<ApiProfile>('/profile/me').catch(()=>null),
            allPages<ApiNotification>('/notifications').catch(()=>null),
        ]);
        return {
            items: page.items,
            summary: {
                accountID,
                groupLabel: profile?.student?.group ? `Группа ${profile.student.group.groupNumber}` : 'Группа не указана',
                hasUnread: notices?.items.some(item => !item.notificationReadAt)??false,
            } satisfies TodaySummary,
        };
    });
}
