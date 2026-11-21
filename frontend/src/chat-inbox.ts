import {api} from '../api';
import {loadInbox} from './deanery-messages-model';
import {useCachedQuery} from './useCachedQuery';

export type InboxItem={id:string;title:string;subtitle:string;kind:'private'|'group';unread:number;when:string};
export type Group={groupID:string;title:string;kind:'academic'|'custom';unread:number;updatedAt:string;memberCount:number};
export const roleNames:Record<string,string>={STUDENT:'Студент',TEACHER:'Преподаватель',DEAN:'Деканат',SUPPORT:'Поддержка',ADMIN:'Администратор'};
export const groupItem=(item:Group):InboxItem=>({
    id:item.groupID,title:item.title,subtitle:item.kind==='academic'?'Учебный чат':`Участников: ${item.memberCount}`,
    kind:'group',unread:item.unread,when:item.updatedAt,
});

async function inboxFor(accountID:string):Promise<{items:InboxItem[];warning:string}>{
    const [direct,groups]=await Promise.allSettled([loadInbox(accountID),api<{items:Group[]}>('/messages/groups')]);
    if(direct.status==='rejected'&&groups.status==='rejected')throw direct.reason;
    const items:InboxItem[]=(direct.status==='fulfilled'?direct.value:[]).map(chat=>({
        id:chat.conversationID,title:chat.person.displayName,subtitle:roleNames[chat.person.role]??'Личный диалог',
        kind:'private',unread:chat.unread??0,when:chat.updatedAt,
    }));
    const groupItems=groups.status==='fulfilled'?groups.value.items.map(groupItem):[];
    return {items:[...items,...groupItems].sort((a,b)=>b.when.localeCompare(a.when)),
        warning:direct.status==='rejected'?'Личные диалоги временно недоступны':groups.status==='rejected'?'Групповые чаты временно недоступны':''};
}

export function useChatInbox(accountID:string, pollMs=5000) {
    return useCachedQuery(accountID?'chat-inbox:'+accountID:null,()=>inboxFor(accountID),0,undefined,pollMs);
}

export function useUnreadChats(accountID:string) {
    return useChatInbox(accountID).data?.items.some(item=>item.unread>0)??false;
}
