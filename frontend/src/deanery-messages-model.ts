import {allPages,api} from '../api';
export type Person={accountID:string;displayName:string;role:string};
export type Conversation={conversationID:string;participants:string[];updatedAt:string;members?:Person[];unread?:number};
export type ChatMessage={messageID:string;conversationID:string;senderAccountID:string;text:string;createdAt:string;readAt:string|null};
export async function loadDeaneryContacts(operator=false){return (await api<{items:Person[]}>(operator?'/support/operators':'/messages/deanery-contacts')).items}
export async function loadInbox(accountID:string){const page=await allPages<Conversation>('/messages/conversations');return Promise.all(page.items.map(async conversation=>{const items=conversation.members??(await api<{items:Person[]}>(`/messages/conversations/${conversation.conversationID}/participants`)).items;return {...conversation,person:items.find(p=>p.accountID!==accountID)||{accountID:'',displayName:'Участник диалога',role:''}}}))}
export async function loadDialog(id:string){const page=await allPages<ChatMessage>(`/messages/conversations/${id}/messages`);return page.items}
export async function sendDialogMessage(accountID:string,conversationID:string,text:string){let id=conversationID;if(!id){const result=await api<Conversation>('/messages/conversations','POST',{participantAccountID:accountID});id=result.conversationID}await api(`/messages/conversations/${id}/messages`,'POST',{text});return id}
export async function markDialogRead(id:string, through:string){return api(`/messages/conversations/${id}/read?through=${encodeURIComponent(through)}`,'POST')}
