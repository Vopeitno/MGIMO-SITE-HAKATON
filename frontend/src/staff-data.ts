import {allPages} from '../api';
import {useCachedQuery} from './useCachedQuery';

export const staffInput='input staff-input';
export const staffSelect='input staff-input spaced-select';
export function formString(form:FormData,key:string){return String(form.get(key)??'').trim()}
export function useStaffList<T>(path:string|null){
    return useCachedQuery(path,()=>allPages<T>(path!).then(page=>page.items));
}
export type StaffGroup={courseGroupID:string;courseID:string;title:string;groupNumber:number;semester:number;studentCount:number};
export type RosterRow={studentID:string;studentName:string;studentNumber:number;groupNumber:number;courseGraduate:string|null;score:number|null;credit:boolean|null;editable:boolean};
export type StaffMaterial={id:string;title:string;description:string;course:string;courseID:string;groupNumber:number;url:string|null;hasFile:boolean;published:boolean};
export type Directory={buildings:{buildingID:string;building:string;buildingAddress:string|null}[];
    classrooms:{classroomID:number;buildingID:string;classroomNumber:number}[];teachers:{teacherID:string;teacherName:string}[]};
export const roleLabels={STUDENT:'Студент',TEACHER:'Преподаватель',DEAN:'Деканат',SUPPORT:'Поддержка',ADMIN:'Администратор'} as const;
