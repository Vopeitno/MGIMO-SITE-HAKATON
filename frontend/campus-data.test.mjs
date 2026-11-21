import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';

const code=ts.transpileModule(readFileSync(new URL('./src/campus-data.ts',import.meta.url),'utf8'),{
  compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2023},
}).outputText
  .replace(/from ['"]\.\.\/api['"]/g,"from 'data:text/javascript,export const allPages=()=>0;export const api=()=>0;'")
  .replace(/from ['"]\.\/useCachedQuery['"]/g,"from 'data:text/javascript,export const useCachedQuery=()=>0;'");
const {campusRoute,roomQuery}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));

test('Яндекс Карты по умолчанию получают только адрес назначения',()=>{
  const building={buildingID:'a',buildingAddress:'Москва, проспект Вернадского, 76'};
  assert.equal(campusRoute(),null);
  assert.equal(campusRoute({buildingAddress:'Вымышленный учебный адрес, 15'}),null);
  assert.equal(campusRoute({buildingAddress:'  '}),null);
  const url=new URL(campusRoute(building));
  assert.equal(url.hostname,'yandex.ru');
  assert.equal(url.searchParams.get('rtext'),'~'+building.buildingAddress);
  assert.equal(url.searchParams.get('rtt'),'pd');
});

test('Google Maps и Apple Maps получают адрес назначения пешего маршрута',()=>{
  const building={buildingAddress:'Одинцово, Новоспортивная улица, 3'};
  const google=new URL(campusRoute(building,'google'));
  assert.equal(google.hostname,'www.google.com');
  assert.equal(google.searchParams.has('origin'),false);
  assert.equal(google.searchParams.get('destination'),building.buildingAddress);
  assert.equal(google.searchParams.get('travelmode'),'walking');
  assert.equal(google.searchParams.get('dir_action'),'navigate');
  const apple=new URL(campusRoute(building,'apple'));
  assert.equal(apple.hostname,'maps.apple.com');
  assert.equal(apple.searchParams.has('source'),false);
  assert.equal(apple.searchParams.get('destination'),building.buildingAddress);
  assert.equal(apple.searchParams.get('mode'),'walking');
});

test('2ГИС получает только координаты корпуса и открывается в пешем режиме',()=>{
  const url=new URL(campusRoute({buildingAddress:'Москва, проспект Вернадского, 76, корпус 3'},'2gis'));
  assert.equal(url.hostname,'2gis.ru');
  assert.equal(decodeURIComponent(url.pathname),'/moscow/directions/tab/pedestrian/points/|37.486302,55.673407');
  assert.equal(url.search,'');
  const fallback=new URL(campusRoute({buildingAddress:'Москва, Садовая улица, 1'},'2gis'));
  assert.equal(decodeURIComponent(fallback.pathname),'/search/Москва, Садовая улица, 1');
});

test('Интервал проверки аудиторий передаётся в московском времени',()=>{
  const path=new URL(roomQuery({buildingID:'',from:'2026-11-20T19:00',to:'2026-11-20T20:30'}),'https://example.test');
  assert.equal(path.pathname,'/campus/rooms');
  assert.equal(path.searchParams.get('from'),'2026-11-20T19:00:00+03:00');
  assert.equal(path.searchParams.get('to'),'2026-11-20T20:30:00+03:00');
  assert.equal(path.searchParams.has('buildingID'),false);
});
