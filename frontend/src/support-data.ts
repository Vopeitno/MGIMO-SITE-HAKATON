import {allPages, api} from '../api';
import {useCachedQuery} from './useCachedQuery';

export type SupportArticle = {id: string; question: string; answer: string; updatedAt: string};
export const ticketCategories = {
    study: 'Учебный процесс', schedule: 'Расписание', technical: 'Техническая проблема',
    dormitory: 'Общежитие', other: 'Другое', consultation: 'Очная консультация',
};
export type TicketCategory = keyof typeof ticketCategories;
export type TicketPriority = 'normal' | 'urgent';
export type Ticket = {id: string; category: TicketCategory; priority: TicketPriority; assigned: boolean;
    assignedToMe: boolean; studentName: string | null; studentNumber: number | null;
    createdAt: string; updatedAt: string; closedAt: string | null};
export type TicketThread = Ticket & {messages: {id: string; fromStudent: boolean; message: string; createdAt: string}[]};

export function useSupportArticles() {
    return useCachedQuery('support-articles', () => allPages<SupportArticle>('/support/articles').then(page => page.items));
}

export async function createSupportTicket(category: TicketCategory, priority: TicketPriority, message: string) {
    return api<{id: string}>('/support/tickets', 'POST', {category, priority, message});
}
