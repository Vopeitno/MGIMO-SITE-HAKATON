import {readFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import ts from 'typescript';
const require=createRequire(import.meta.url);
const compile=s=>`import React from '${pathToFileURL(require.resolve('react')).href}';\n`+ts.transpileModule(s,{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext,jsx:ts.JsxEmit.React}}).outputText;
const url=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const reactURL=pathToFileURL(require.resolve('react')).href;
const read=path=>readFileSync(new URL(path,import.meta.url),'utf8');
const todayURL=url(compile(read('./src/today-model.ts')));
const modelURL=url(compile(read('./src/deadlines-model.ts')).replace(/from ['"]\.\/today-model['"]/g,`from '${todayURL}'`));
const fakeAPI=url(`export async function allPages(path){return path==='/academic/subjects'?{items:[{courseID:'c',title:'Предмет из API'}]}:{items:[{deadlineID:1,courseID:'c',deadlineTitle:'Задание из API',submissionURL:'https://ed.mgimo.ru/',customSubmissionURL:null}]}};export async function api(){throw new Error('unexpected write during render')}`);
const loaderURL=url(compile(read('./src/teacher-deadlines-model.ts')).replace(/from ['"]\.\.\/api['"]/g,`from '${fakeAPI}'`));
const teacherURL=url(compile(read('./src/TeacherDeadlineLinks.tsx')).replace(/from ['"]\.\/useCachedQuery['"]/g, "from 'data:text/javascript,export function useCachedQuery(){return {data:null,ready:false,error:%22%22,reload(){},update(){}}}'").replace(/from ['"]react['"]/g,`from '${reactURL}'`).replace(/from ['"]\.\.\/api['"]/g,`from '${fakeAPI}'`).replace(/from ['"]\.\/deadlines-model['"]/g,`from '${modelURL}'`).replace(/from ['"]\.\/teacher-deadlines-model['"]/g,`from '${loaderURL}'`)+'\nexport {LinkForm};');
const {LinkForm}=await import(teacherURL);const {loadTeacherDeadlines}=await import(loaderURL);
test('Преподаватель получает задания и дисциплины только через API',async()=>{
 const data=await loadTeacherDeadlines();assert.equal(data.length,1);assert.equal(data[0].courseTitle,'Предмет из API');assert.equal(data[0].deadlineTitle,'Задание из API');assert.equal(data[0].customSubmissionURL,null);
});
test('Форма ссылки: сохранение, сброс на Moodle и отдельное открытие без доступа к исходному окну',()=>{
 const html=renderToStaticMarkup(React.createElement(LinkForm,{item:{deadlineID:1,deadlineTitle:'Задание',courseTitle:'Предмет',submissionURL:'https://lms.example.test/task',customSubmissionURL:'https://lms.example.test/task'},onSaved(){},toast(){}}));
 assert.ok(html.includes('Сохранить ссылку'));assert.ok(html.includes('Вернуть общий Moodle'));assert.ok(html.includes('type="url"'));assert.ok(html.includes('value="https://lms.example.test/task"'));assert.ok(html.includes('rel="noopener noreferrer"'));assert.ok(!html.includes('disabled=""'));
 const fallback=renderToStaticMarkup(React.createElement(LinkForm,{item:{deadlineID:1,deadlineTitle:'Задание',courseTitle:'Предмет',submissionURL:'https://ed.mgimo.ru/',customSubmissionURL:null},onSaved(){},toast(){}}));assert.ok(fallback.includes('placeholder="https://ed.mgimo.ru/"'));assert.ok(fallback.includes('disabled=""'));assert.ok(fallback.includes('value=""'));
});
test('Рабочие кабинеты сохраняют исходную таблицу стилей и маршруты преподавателя',()=>{
 const original=execFileSync('git',['show','HEAD:src/RoleDashboard.tsx'],{encoding:'utf8'});
 const current=read('./src/RoleDashboard.tsx');
 assert.equal(current.match(/const css=`([\s\S]*?)`;/)[1],original.match(/const css=`([\s\S]*?)`;/)[1]);
 assert.match(current,/active==="deadlines"[\s\S]*?<TeacherDeadlineLinks/);
 for(const name of ['role-shell','role-side','role-main','role-bottom'])assert.ok(current.includes(name));
 assert.match(current,/<AdminAccounts\/>/);
});
