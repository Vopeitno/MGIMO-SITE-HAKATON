import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const modelSource = readFileSync(new URL('./src/today-model.ts', import.meta.url), 'utf8');
const modelJS = ts.transpileModule(modelSource, { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
const { buildTodayModel, moscowDate, safeLessonLink } = await import(`data:text/javascript;base64,${Buffer.from(modelJS).toString('base64')}`);
const now = new Date('2026-11-19T06:00:00Z');
const lesson = (id, startAt, status = 'active') => ({ lessonID: id, topic: `Тема ${id}`, startAt, endAt: '2026-11-19T09:00:00Z', lessonStatus: status, teacher: { teacherName: 'Вымышленный преподаватель' }, classroom: { classroomNumber: 101 }, building: { building: 'Учебный корпус' }, course: { title: 'Курс из API' } });

test('Пустые ответы не подменяются демонстрационными занятиями и оценками', () => {
  const m = buildTodayModel({ nextLessons: [], appointments: [], deadlines: [] }, [], [], [], now);
  assert.equal(m.average, '—'); assert.equal(m.progress, 0); assert.equal(m.courseCount, 0);
  assert.deepEqual(m.lessons, []); assert.deepEqual(m.requests, []); assert.deepEqual(m.deadlines, []);
  assert.equal(m.hero.title, 'Занятий нет');
});
test('Успеваемость: нулевой балл учитывается, отсутствие оценки не означает ноль', () => {
  const m = buildTodayModel({ nextLessons: [], appointments: [], deadlines: [] }, [], [{ courseID: 'a' }, { courseID: 'b' }, { courseID: 'c' }], [
    { courseID: 'a', score: 0, credit: false }, { courseID: 'b', score: 60, credit: true }, { courseID: 'c', score: null, credit: null },
    { courseID: 'foreign', score: 100, credit: true },
  ], now);
  assert.equal(m.average, '30'); assert.equal(m.completed, 1); assert.equal(m.courseCount, 3); assert.ok(Math.abs(m.progress - 100 / 3) < 1e-10);
});
test('Ближайшее занятие выбирается по времени, отменённые и прошедшие исключаются', () => {
  const m = buildTodayModel({ nextLessons: [lesson(1, '2026-11-19T08:00:00Z'), lesson(2, '2026-11-19T07:00:00Z'), lesson(3, '2026-11-19T06:30:00Z', 'canceled'), lesson(4, '2026-11-19T05:00:00Z')], appointments: [], deadlines: [] }, [], [], [], now);
  assert.match(m.hero.when, /^10:00–12:00/); assert.match(m.hero.when, /через 60 мин\./);
});
test('Москва, месяц дедлайна и полные идентификаторы берутся из данных', () => {
  assert.equal(moscowDate(new Date('2026-11-18T21:30:00Z')), '2026-11-19');
  const m = buildTodayModel({ nextLessons: [], appointments: [{ appoinmentID: '12345678-1234-4123-8123-123456789012', appointmentTitle: 'Название из БД', appointmentStatus: 'proccess', created_at: '2026-11-19T01:00:00Z' }], deadlines: [{ deadlineID: 1, deadlineTitle: 'Задание из БД', courseID: 'a', deadlineEndAt: '2027-01-04T20:59:00Z' }] }, [], [{ courseID: 'a', title: 'Дисциплина из БД' }], [], now);
  assert.equal(m.requests[0].id, '12345678-1234-4123-8123-123456789012'); assert.equal(m.requests[0].displayId, '12345678');
  assert.equal(m.requests[0].status, 'В обработке'); assert.equal(m.deadlines[0].month, 'янв'); assert.match(m.deadlines[0].sub, /Дисциплина из БД/);
});
test('Ссылка на занятие допускает только HTTP и HTTPS', () => {
  for (const raw of ['javascript:alert(1)', 'data:text/html,bad', '/relative', 'file:///etc/passwd']) assert.equal(safeLessonLink(raw), null);
  assert.equal(safeLessonLink('https://example.test/lesson'), 'https://example.test/lesson');
});

function compileView(source, data) {
  const file = ts.createSourceFile('view.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const names = new Set(['Icon', 'Quick', 'Deadline', 'LessonList', 'RequestList', 'Today']);
  const variables = new Set(['iconPaths', 'lessons', 'initialRequests']);
  const parts = [];
  for (const node of file.statements) {
    if (ts.isFunctionDeclaration(node) && names.has(node.name?.text)) parts.push(node.getText(file));
    if (ts.isVariableStatement(node) && node.declarationList.declarations.some(d => variables.has(d.name.getText(file)))) parts.push(node.getText(file));
  }
  const js = ts.transpileModule(parts.join('\n'), { compilerOptions: { jsx: ts.JsxEmit.React, module: ts.ModuleKind.None, target: ts.ScriptTarget.ES2023 } }).outputText;
  return new Function('React', 'useEffect', 'useTodayData', 'api', 'appointmentStatus', 'safeLessonLink', `${js}\nreturn {Today,lessons,initialRequests:typeof initialRequests==='undefined'?[]:initialRequests};`)(React, React.useEffect, () => ({ data, error: '' }), async () => { throw new Error('Запрос во время отрисовки'); }, () => 'Создана', safeLessonLink);
}
test('Разметка «Сегодня»: теги, классы и стили совпадают с исходной при одинаковом количестве записей', () => {
  const baseline = execFileSync('git', ['show', 'HEAD:src/StudentDashboard.tsx'], { encoding: 'utf8' });
  const original = compileView(baseline, null);
  const data = {
    hero: { status: 'По расписанию', when: 'Время из БД', title: 'Курс из БД', room: 'Аудитория 101', teacher: 'Преподаватель', format: 'Очно' },
    lessons: original.lessons, requests: original.initialRequests, average: '85', progress: 78, completed: 18, courseCount: 23, progressStatus: 'Все зачтено',
    deadlines: [{ id: 1, date: '18', month: 'ноя', title: 'Задание 1', sub: 'Срок 1' }, { id: 2, date: '21', month: 'ноя', title: 'Задание 2', sub: 'Срок 2' }],
  };
  const current = compileView(readFileSync(new URL('./src/StudentDashboard.tsx', import.meta.url), 'utf8'), data);
  const props = { go() {}, modal() {}, toast() {}, requestItems: original.initialRequests };
  const shape = html => html.replace(/<!--[\s\S]*?-->/g, '').replace(/>[^<]*</g, '><');
  assert.equal(shape(renderToStaticMarkup(React.createElement(current.Today, props))), shape(renderToStaticMarkup(React.createElement(original.Today, props))));
});
test('CSS сохранён, кроме согласованных быстрых кнопок и элементов формы заявки', () => {
  const original = execFileSync('git', ['show', 'HEAD:src/StudentDashboard.tsx'], { encoding: 'utf8' });
  const current = readFileSync(new URL('./src/StudentDashboard.tsx', import.meta.url), 'utf8');
  const css = source => source.match(/const css\s*=\s*(?:String\.raw\s*)?`([\s\S]*?)`;/)[1];
  const actualCSS = css(current);
  assert.ok(actualCSS.includes('@container quick-actions (max-width:400px){.quick-grid{grid-template-columns:1fr}}'));
  const controls=actualCSS.match(/\n\/\* Request form controls \*\/[\s\S]*?\/\* End request form controls \*\/\n/)[0];
  assert.ok(controls.includes('background-position:right 16px center'));assert.ok(controls.includes('padding-right:44px'));
  const restoredCSS = actualCSS
    .replace(/\n\/\* Grade alignment \*\/[\s\S]*?\/\* End grade alignment \*\/\n/, '\n')
    .replace(controls,'')
    .replace('.quick-title{font-size:12px;font-weight:850;line-height:1.3;overflow-wrap:normal;word-break:normal}', '.quick-title{font-size:12px;font-weight:850;line-height:1.3;overflow-wrap:anywhere}')
    .replace('.quick-sub{margin-top:3px;color:#736e80;font-size:10px;line-height:1.35;overflow-wrap:normal;word-break:normal}', '.quick-sub{margin-top:3px;color:#736e80;font-size:10px;line-height:1.35;overflow-wrap:anywhere}')
    .replace('.quick-card{container-type:inline-size;container-name:quick-actions}\n@container quick-actions (max-width:400px){.quick-grid{grid-template-columns:1fr}}\n', '');
  assert.equal(restoredCSS, css(original));
  const file = ts.createSourceFile('view.tsx', original, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const changed = new Set(['Today', 'Login', 'Deadline', 'LessonList', 'RequestList', 'Grades', 'ScoreRing', 'Schedule', 'Deadlines', 'Requests', 'RequestModal', 'Notifications', 'Learning', 'Profile', 'Activities', 'News', 'Chat', 'OnlineClasses', 'Communication', 'Campus', 'Services', 'Support', 'Career', 'Portfolio', 'Finance']);
  for (const node of file.statements) {
    if (ts.isFunctionDeclaration(node) && node.name && !changed.has(node.name.text) && !node.modifiers?.some(m => m.kind === ts.SyntaxKind.ExportKeyword)) assert.ok(current.includes(node.getText(file)), `Изменён несогласованный компонент ${node.name.text}`);
  }
});
