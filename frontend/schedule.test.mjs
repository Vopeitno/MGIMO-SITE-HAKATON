import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
const source=readFileSync(new URL('./src/StudentDashboard.tsx',import.meta.url),'utf8');
const compile=s=>ts.transpileModule(s,{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext,jsx:ts.JsxEmit.React}}).outputText;
const url=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const todayURL=url(compile(readFileSync(new URL('./src/today-model.ts',import.meta.url),'utf8')));
const {moscowDate}=await import(todayURL);
const {buildScheduleModel,shiftDate,weekPeriod}=await import(url(compile(readFileSync(new URL('./src/schedule-model.ts',import.meta.url),'utf8')).replace(/from ['"]\.\/today-model['"]/g,`from '${todayURL}'`)));
test('Неделя начинается в понедельник, включает воскресенье и границы месяца/года',()=>{
 assert.deepEqual(weekPeriod('2027-01-01'),{from:'2026-12-28',to:'2027-01-03'});
 assert.deepEqual(weekPeriod('2026-11-22'),{from:'2026-11-16',to:'2026-11-22'});
 assert.equal(shiftDate('2028-02-28',1),'2028-02-29');assert.equal(shiftDate('2027-01-01',-1),'2026-12-31');
});
const lesson=(id,start,status='scheduled',link=null)=>({lessonID:id,topic:`Тема ${id}`,startAt:start,endAt:'2026-11-19T12:00:00Z',lessonStatus:status,remoteLessonLink:link,teacher:{teacherName:'Преподаватель'},classroom:{classroomNumber:304},building:{building:'Корпус'}});
test('Серверные занятия: сортировка, Москва, выходные, отмена, перенос и безопасная ссылка',()=>{
 const lessons=[lesson(1,'2026-11-18T21:30:00Z','canceled','https://example.test/canceled'),lesson(2,'2026-11-21T07:00:00Z','rescheduled','javascript:alert(1)'),lesson(3,'2026-11-19T07:00:00Z','scheduled','https://example.test/lesson'),lesson(4,'2026-11-23T07:00:00Z')];
 const m=buildScheduleModel(lessons,[{lessonChangeID:1,lessonID:2,changedAt:'2026-11-19T09:00:00Z',reason:'Причина из базы'},{lessonChangeID:2,lessonID:4,changedAt:'2026-11-19T09:00:00Z',reason:'Другой период'}],'2026-11-19','week');
 assert.deepEqual(m.lessons.map(l=>l.id),[1,3,2]);assert.equal(m.week.length,6);assert.equal(m.week[3].items[0].t,'00:30');assert.ok(m.week[3].items[0].canceled);assert.equal(m.week[3].items[0].remoteLessonLink,null);assert.ok(m.week[5].items[0].changed);assert.equal(m.week[5].items[0].remoteLessonLink,null);
 assert.deepEqual(m.changes.map(c=>c.reason),['Причина из базы']);assert.equal(m.changes[0].status,'Изменено');
 assert.deepEqual(buildScheduleModel(lessons,[],'2026-11-19','day').lessons.map(l=>l.id),[1,3]);
 const moved=buildScheduleModel([lesson(4,'2026-11-23T07:00:00Z')],[{lessonChangeID:3,lessonID:4,changedAt:'2026-11-19T09:00:00Z',oldStartAt:'2026-11-19T07:00:00Z',newStartAt:'2026-11-23T07:00:00Z',reason:'Перенос на следующую неделю'}],'2026-11-19','week');assert.equal(moved.lessons.length,0);assert.equal(moved.changes[0].reason,'Перенос на следующую неделю');
 const empty=buildScheduleModel([],[],'2027-01-05','week');assert.equal(empty.week.length,5);assert.ok(empty.week.every(d=>d.items.length===0));assert.equal(empty.changes.length,0);
});
const pickerSource=readFileSync(new URL('./src/ScheduleDatePicker.tsx',import.meta.url),'utf8');
const pickerJS=compile(pickerSource.replace(/^import .*;\n/gm,'').replace('export function ','function '));
const ScheduleDatePicker=new Function('React','useId','useLayoutEffect','useRef','useState','shiftDate','weekPeriod','moscowDate',
 `${pickerJS}\nreturn ScheduleDatePicker`)(React,React.useId,React.useLayoutEffect,React.useRef,React.useState,shiftDate,weekPeriod,moscowDate);
function view(src,data){
 const file=ts.createSourceFile('view.tsx',src,ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);const parts=[];
 for(const node of file.statements){if(ts.isFunctionDeclaration(node)&&['Icon','Schedule','LessonList'].includes(node.name?.text))parts.push(node.getText(file));if(ts.isVariableStatement(node)&&node.declarationList.declarations.some(d=>['week','lessons','iconPaths'].includes(d.name.getText(file))))parts.push(node.getText(file))}
 const js=ts.transpileModule(parts.join('\n'),{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.None,jsx:ts.JsxEmit.React}}).outputText;
 return new Function('React','useState','useEffect','useScheduleData','moscowDate','shiftDate','weekPeriod','safeLessonLink','ScheduleDatePicker',`${js}\nreturn {Schedule,week:typeof week==='undefined'?[]:week}`)(React,React.useState,React.useEffect,()=>({data,error:''}),moscowDate,shiftDate,weekPeriod,()=>null,ScheduleDatePicker);
}
test('«Расписание» сохраняет исходные теги, классы и размеры вне кнопки даты и календаря',()=>{
 const baseline=execFileSync('git',['show','HEAD:src/StudentDashboard.tsx'],{encoding:'utf8'});const original=view(baseline,null);
 const data={week:original.week.map((d,di)=>({...d,items:d.items.map((x,i)=>({id:di*10+i,t:x[0],n:x[1],r:x[2],format:x[3],changed:di===0&&i===1,canceled:false}))})),changes:[{id:1,title:'Название из БД',reason:'Причина',when:'Дата',status:'Изменено'}],summary:{accountID:'a'}};
 const props={toast(){},accountID:'a',onSummary(){}};const shape=html=>html.replace(/<!--[\s\S]*?-->/g,'').replace(/>[^<]*</g,'><').replace(/value="[^"]*"/g,'value=""');
 const current=renderToStaticMarkup(React.createElement(view(source,data).Schedule,props));
 const previous=renderToStaticMarkup(React.createElement(original.Schedule,props));
 assert.match(current,/<time dateTime="\d{4}-\d{2}-\d{2}">\d+ [а-яё]+ \d{4}/i);
 assert.equal(
  shape(current.replace(/<button[^>]*schedule-date-trigger[^>]*>[\s\S]*?<\/button><dialog[\s\S]*?<\/dialog>/,'')),
  shape(previous.replace(/<input[^>]*type="date"[^>]*\/>/,''))
 );
});
