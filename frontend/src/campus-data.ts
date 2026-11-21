import {allPages,api} from '../api';
import {useCachedQuery} from './useCachedQuery';
import type {ApiNotification,TodaySummary} from './today-model';

export type CampusBuilding={buildingID:string;building:string;buildingAddress?:string|null};
export type CampusRoom={classroomID:number;classroomNumber:number;buildingID:string;building:string;available:boolean};
export type RoomFilter={buildingID:string;from:string;to:string};
export type RoomResult={items:CampusRoom[];available:number;total:number};

export function campusAddress(building?:CampusBuilding):string {
    const address=building?.buildingAddress?.trim()??'';
    return /вымышлен|тестов|учебный адрес/i.test(address)?'':address;
}

export type CampusMapProvider='yandex'|'google'|'apple'|'2gis';

// Public destination coordinates for the three addresses in the demo building directory.
const campusDestinationCoordinates: ReadonlyMap<string,string> = new Map([
    ['Москва, проспект Вернадского, 76','37.491638,55.669706'],
    ['Москва, проспект Вернадского, 76, корпус 3','37.486302,55.673407'],
    ['Одинцово, Новоспортивная улица, 3','37.277371,55.680661'],
]);

export function campusRoute(destination?:CampusBuilding,provider:CampusMapProvider='yandex') {
    const address=campusAddress(destination);
    if(!address)return null;
    if(provider==='2gis'){
        const point=campusDestinationCoordinates.get(address);
        return point?'https://2gis.ru/moscow/directions/tab/pedestrian/points/|'+point
            :'https://2gis.ru/search/'+encodeURIComponent(address);
    }
    if(provider==='google'){
        return 'https://www.google.com/maps/dir/?'+new URLSearchParams({api:'1',destination:address,travelmode:'walking',dir_action:'navigate'});
    }
    if(provider==='apple'){
        return 'https://maps.apple.com/directions?'+new URLSearchParams({destination:address,mode:'walking'});
    }
    return 'https://yandex.ru/maps/?'+new URLSearchParams({mode:'routes',rtext:'~'+address,rtt:'pd'});
}

export function roomQuery(filter:RoomFilter) {
    const params=new URLSearchParams({from:filter.from+':00+03:00',to:filter.to+':00+03:00'});
    if(filter.buildingID)params.set('buildingID',filter.buildingID);
    return '/campus/rooms?'+params.toString();
}

export function useCampusData(accountID:string){
    return useCachedQuery(`campus:${accountID}`,async()=>{
        const [buildings,profile,notices]=await Promise.all([
            allPages<CampusBuilding>('/buildings'),
            api<{student?:{studentNumber?:number;group?:{groupNumber:number}}}>('/profile/me').catch(()=>null),
            allPages<ApiNotification>('/notifications').catch(()=>null),
        ]);
        const student=profile?.student;
        return{buildings:buildings.items,studentNumber:student?.studentNumber??null,
            summary:{accountID,groupLabel:student?.group?`Группа ${student.group.groupNumber}`:'Группа не указана',
                hasUnread:notices?.items.some(item=>!item.notificationReadAt)??false} satisfies TodaySummary};
    });
}

export type CampusService = 'benefits' | 'lost-reports' | 'resource-slots';

export const campusServiceTitles: Record<CampusService, string> = {
    benefits: 'Льготы и бонусы', 'lost-reports': 'Бюро находок', 'resource-slots': 'Мои ресурсы',
};

