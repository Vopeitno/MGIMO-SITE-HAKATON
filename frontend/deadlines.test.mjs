import {readFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
const compile=s=>ts.transpileModule(s,{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext}}).outputText;
const url=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const todayURL=url(compile(readFileSync(new URL('./src/today-model.ts',import.meta.url),'utf8')));
const {buildDeadlinesModel,deadlinesCalendar,deadlineSubmissionURL}=await import(url(compile(readFileSync(new URL('./src/deadlines-model.ts',import.meta.url),'utf8')).replace(/from ['"]\.\/today-model['"]/g,`from '${todayURL}'`)));
const now=new Date('2026-11-19T21:00:00Z');
const deadline=(id,end,start=null)=>({deadlineID:id,courseID:'c',deadlineTitle:`Задание ${id}`,deadlineDescription:'Описание',deadlineEndAt:end,deadlineStartAt:start});
test('Дедлайны: реальные сроки, Москва, просрочка, отсутствие дат и будущее начало',()=>{
 const items=buildDeadlinesModel([deadline(4,null),deadline(1,'2026-11-19T20:59:00Z'),deadline(3,'2026-11-25T21:00:00Z','2026-11-24T21:00:00Z'),deadline(2,'2026-11-20T21:00:00Z')],[{courseID:'c',title:'Название из базы'}],now);
 assert.deepEqual(items.map(x=>x.id),[1,2,3,4]);assert.equal(items[0].left,'Срок истёк');assert.ok(items[0].urgent);assert.ok(items[1].urgent);assert.match(items[1].due,/21 ноября 2026/);assert.equal(items[1].course,'Название из базы');assert.ok(items[2].pending);assert.ok(!items[2].urgent);assert.equal(items[3].left,'Срок не указан');assert.ok(!items[3].urgent);
 assert.deepEqual(buildDeadlinesModel([],[],now),[]);
});
test('Календарь: только реальные даты, безопасный текст, UTC и корректные переносы UTF-8',()=>{
 const raw=deadline(1,'2026-11-20T21:00:00Z');raw.deadlineTitle='Длинное название '.repeat(30)+'\r\nBEGIN:VEVENT';raw.deadlineDescription='Текст; с, разделителями\\и\nстроками';
 const result=deadlinesCalendar(buildDeadlinesModel([raw,deadline(2,null)],[],now),'account',now);
 assert.equal(result.count,1);assert.ok(result.content.includes('DTSTART:20261120T210000Z'));assert.ok(result.filename.endsWith('.ics'));
 const unfolded=result.content.replace(/\r\n /g,'');assert.equal((unfolded.match(/\r\nBEGIN:VEVENT\r\n/g)||[]).length,1);assert.ok(unfolded.includes('\\nBEGIN:VEVENT'));assert.ok(unfolded.includes('Текст\\; с\\, разделителями\\\\и\\nстроками'));
 for(const line of result.content.split('\r\n'))assert.ok(new TextEncoder().encode(line).length<=75);
 assert.ok(result.content.endsWith('END:VCALENDAR\r\n'));
});

function view(src,data,filter){
 const file=ts.createSourceFile('view.tsx',src,ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX);const parts=[];
 for(const node of file.statements){if(ts.isFunctionDeclaration(node)&&['Icon','Deadlines'].includes(node.name?.text))parts.push(node.getText(file));if(ts.isVariableStatement(node)&&node.declarationList.declarations.some(d=>d.name.getText(file)==='iconPaths'))parts.push(node.getText(file))}
 const js=ts.transpileModule(parts.join('\n'),{compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.None,jsx:ts.JsxEmit.React}}).outputText;
 return new Function('React','useState','useEffect','useDeadlinesData','deadlinesCalendar',`${js}\nreturn Deadlines`)(React,initial=>React.useState(filter??initial),React.useEffect,()=>({data,error:''}),deadlinesCalendar);
}
test('«Дедлайны»: исходное оформление сохранено, фильтр «Выполнено» доступен',()=>{
 const baseline=execFileSync('git',['show','HEAD:src/StudentDashboard.tsx'],{encoding:'utf8'});const current=readFileSync(new URL('./src/StudentDashboard.tsx',import.meta.url),'utf8');
 const data={items:[1,2,3,4].map(id=>({id,title:'Задание',course:'Курс из БД',due:'до указанного срока',urgent:id===1,pending:false,overdue:false,left:'Срок из БД'})),summary:{accountID:'a'}};
 const props={toast(){},accountID:'a',onSummary(){}};const html=renderToStaticMarkup(React.createElement(view(current,data),props));
 const shape=html=>html.replace(/(<button class="check-btn ?"[^>]*>)<svg[\s\S]*?<\/svg>(<\/button>)/g,'$1$2').replace(/(<div class="row-name"[^>]*>)<button[^>]*>([^<]*)<\/button>(<\/div>)/g,'$1$2$3').replace(/<!--[\s\S]*?-->/g,'').replace(/class="check-btn "/g,'class="check-btn"').replace(/ (disabled|aria-label|aria-pressed|title)="[^"]*"/g,'').replace(/>[^<]*</g,'><');
 assert.equal(shape(html),shape(renderToStaticMarkup(React.createElement(view(baseline,null),props))));
 assert.match(html,/<button class="filter " aria-pressed="false">Выполнено<\/button>/);
 const completed=renderToStaticMarkup(React.createElement(view(current,data,'Выполнено'),props));
 assert.match(completed,/<button class="filter active" aria-pressed="true">Выполнено<\/button>/);
 assert.match(completed,/Портал пока их не получает/);
 assert.doesNotMatch(completed,/class="deadline-row/);
 assert.equal((html.match(/aria-label="Ссылка на сдачу ещё не указана"/g)||[]).length,4);assert.ok(!html.includes('86%'));assert.ok(!current.includes('done-deadlines'));
});

test('Переход к заданию принимает HTTPS без учётных данных, не подменяет отсутствующую ссылку',()=>{
 for(const raw of [null,undefined,'','javascript:alert(1)','data:text/html,bad','http://lms.example.test/','//lms.example.test/','https://login:secret@lms.example.test/task'])assert.equal(deadlineSubmissionURL(raw),null);
 const link='https://lms.example.test/mod/assign/view.php?id=42';assert.equal(deadlineSubmissionURL(link),link);
 const raw=deadline(1,'2026-11-20T21:00:00Z');raw.submissionURL=link;assert.equal(buildDeadlinesModel([raw],[],now)[0].submissionURL,link);
});
