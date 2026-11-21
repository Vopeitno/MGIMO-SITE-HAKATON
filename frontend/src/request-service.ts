import {api,ApiError} from '../api';
import type {RequestRecord} from './requests-model';
export type RequestDraft={appointmentDraftID:string;draftStatus:string};
export async function submitRequestDraft(id:string,buildingID:string|null){
 try{return await api<RequestRecord>(`/requests/drafts/${id}/submit`,'POST',{buildingID})}
 catch(error){if(error instanceof ApiError&&error.status===409){return api<RequestRecord>(`/requests/${id}`)}throw error}
}
