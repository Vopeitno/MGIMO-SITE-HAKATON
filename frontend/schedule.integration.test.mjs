import {readFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
const require=createRequire(import.meta.url);
const compile=p=>ts.transpileModule(readFileSync(new URL(p,import.meta.url),'utf8'),{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext}}).outputText;
const url=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
test('Расписание двух студентов: день/неделя, пустой будущий период, собственная группа и реальные изменения',{skip:!process.env.API_TEST_ORIGIN},async()=>{
 const original=globalThis.fetch;let cookie='';const replies=new Map();
 globalThis.fetch=async(path,init={})=>{const target=new URL(path,process.env.API_TEST_ORIGIN);const headers=new Headers(init.headers);headers.set('Origin',process.env.API_TEST_ORIGIN);if(cookie)headers.set('Cookie',cookie);const result=await original(target,{...init,headers});const setCookie=result.headers.get('set-cookie');if(setCookie)cookie=setCookie.includes('Max-Age=0')?'':setCookie.split(';')[0];if((!init.method||init.method==='GET')&&result.ok)replies.set(target.pathname,await result.clone().json());return result};
 try{
  const apiURL=url(compile('./api.ts'));const todayURL=url(compile('./src/today-model.ts'));
  const modelURL=url(compile('./src/schedule-model.ts').replace(/from ['"]\.\/today-model['"]/g,`from '${todayURL}'`));
  const loaderURL=url(compile('./src/useScheduleData.ts').replace(/from ['"]\.\/useCachedQuery['"]/g, "from 'data:text/javascript,export function useCachedQuery(){return {data:null,ready:false,error:%22%22,reload(){},update(){}}}'").replace(/from ['"]\.\.\/api['"]/g,`from '${apiURL}'`).replace(/from ['"]\.\/schedule-model['"]/g,`from '${modelURL}'`).replace(/from ['"]react['"]/g,`from '${pathToFileURL(require.resolve('react')).href}'`));
  const client=await import(apiURL);const {loadScheduleData}=await import(loaderURL);const {moscowDate}=await import(todayURL);const {weekPeriod}=await import(modelURL);
  for(const key of ['STUDENT1','STUDENT2']){
   const user=await client.login(process.env[`DEMO_${key}_EMAIL`],process.env[`DEMO_${key}_PASSWORD`]);
   try{for(const view of ['day','week']){const data=await loadScheduleData(user.accountID,moscowDate(),view);const raw=replies.get('/api/v1/MGIMO/schedule'+(view==='week'?'/period':''));const profile=replies.get('/api/v1/MGIMO/profile/me');assert.equal(data.summary.accountID,user.accountID);assert.equal(data.summary.groupLabel,`Группа ${profile.student.group.groupNumber}`);assert.deepEqual(data.lessons.map(l=>l.id),raw.lessons.map(l=>l.lessonID));assert.ok(raw.lessons.every(l=>l.groupID===profile.student.groupID));const ids=new Set(raw.lessons.map(l=>l.lessonID));const period=view==='week'?weekPeriod(moscowDate()):{from:moscowDate(),to:moscowDate()};const inPeriod=v=>v&&moscowDate(new Date(v))>=period.from&&moscowDate(new Date(v))<=period.to;assert.equal(data.changes.length,replies.get('/api/v1/MGIMO/schedule/changes').items.filter(c=>ids.has(c.lessonID)||inPeriod(c.oldStartAt)||inPeriod(c.newStartAt)).length)}
    const future=await loadScheduleData(user.accountID,'2099-01-05','week');assert.equal(future.lessons.length,0);assert.equal(future.changes.length,0);assert.ok(future.week.every(d=>d.items.length===0));
   }finally{await client.logout()}
  }
 }finally{globalThis.fetch=original}
});
