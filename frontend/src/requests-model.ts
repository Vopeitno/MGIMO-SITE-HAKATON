import {appointmentStatus} from './today-model';
export type RequestRecord={appoinmentID:string;appointmentTitle:string;appointmentDescription:string;appointmentStatus:string;appointmentTypeID:string|null;buildingID:string|null;fileUrlAttachment:string|null;created_at:string;updated_at?:string|null};
export type RequestType={appointmentTypeID:string;appointmentTypeTitle:string;isActive:boolean};
export type Building={buildingID:string;building:string;buildingAddress?:string|null};
export type RequestHistory={appointmentHistoryID:string;appointmentStatus:string;changedAt:string;reason:string|null};
const date=new Intl.DateTimeFormat('ru-RU',{timeZone:'Europe/Moscow',day:'numeric',month:'long',year:'numeric',hour:'2-digit',minute:'2-digit'});
export function requestDate(raw?:string|null){return raw&&Number.isFinite(Date.parse(raw))?date.format(new Date(raw)):'Дата не указана'}
export function requestRow(item:RequestRecord){return{id:item.appoinmentID,displayId:item.appoinmentID.slice(0,8),title:item.appointmentTitle,date:requestDate(item.created_at),status:appointmentStatus(item.appointmentStatus),tone:item.appointmentStatus==='ready'?'mint':item.appointmentStatus==='canceled'?'gray':'blue'}}
export function requestDescription(pickup:string,comment:string){return `Способ получения: ${pickup}\nКомментарий: ${comment.trim()||'Без дополнительных пожеланий'}`}
export function requestPickup(item:RequestRecord){const first=item.appointmentDescription.split('\n')[0];return first.startsWith('Способ получения: ')?first.slice('Способ получения: '.length):item.buildingID?'Забрать в деканате':'Не указан'}
export function requestFileError(file?:File|null){if(!file)return '';if(file.size>10*1024*1024)return 'Размер файла не должен превышать 10 МБ';if(!/\.(pdf|png|jpe?g|docx)$/i.test(file.name))return 'Разрешены PDF, PNG, JPEG или DOCX';if(!file.size)return 'Файл пуст';return ''}
