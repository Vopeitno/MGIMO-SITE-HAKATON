import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const source = readFileSync(new URL('./src/grades-model.ts', import.meta.url), 'utf8');
const js = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
const { buildGradesModel, gradesForYear } = await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`);
const course = (id, title = id) => ({ courseID: id, title, semester: 1, teachers: [{ teacherID: 't', teacherName: 'Имя из сервера' }] });
test('Все доступные курсы показаны, ноль не заменяется отсутствием оценки', () => {
  const data = buildGradesModel([course('a'), course('b'), course('c')], [
    { courseID: 'a', score: 0, credit: false }, { courseID: 'b', score: 60, credit: true }, { courseID: 'other', score: 100, credit: true },
  ]);
  assert.equal(data.subjects.length, 3); assert.equal(data.subjects[0].s, 0); assert.equal(data.subjects[2].s, null);
  assert.equal(data.subjects[0].status, 'Незачёт'); assert.equal(data.subjects[1].status, 'Зачтено'); assert.equal(data.subjects[2].status, 'Нет оценки');
  assert.equal(data.average, '30'); assert.equal(data.completed, 1); assert.equal(data.total, 3);
  assert.equal(data.subjects[0].t, 'Имя из сервера');
});
test('Пустой список и отсутствие оценок не порождают демонстрационные показатели', () => {
  assert.deepEqual(buildGradesModel([], []), { subjects: [], average: '—', completed: 0, total: 0 });
  const data = buildGradesModel([course('a')], [{ courseID: 'a', score: null, credit: null }]);
  assert.equal(data.average, '—'); assert.equal(data.subjects[0].mark, '—'); assert.equal(data.completed, 0);
});
test('Список, средний балл и зачёты зависят от курса, включая последний семестр каждого года', () => {
  const rows = [1, 2, 3, 4, 5, 6].map(semester => ({ ...course(String(semester)), semester }));
  const data = buildGradesModel(rows, [
    { courseID: '1', score: 70, credit: true }, { courseID: '2', score: 80, credit: true },
    { courseID: '3', score: 90, credit: true }, { courseID: '4', score: 100, credit: true },
  ]);
  assert.deepEqual(gradesForYear(data, '1').subjects.map(s => s.id), ['1', '2']);
  assert.equal(gradesForYear(data, '1').average, '75');
  assert.deepEqual(gradesForYear(data, '2').subjects.map(s => s.id), ['3', '4']);
  assert.equal(gradesForYear(data, '2').average, '95');
  const year3 = gradesForYear(data, '3');
  assert.deepEqual(year3.subjects.map(s => s.id), ['5', '6']);
  assert.equal(year3.completed, 0);
  assert.equal(year3.average, '—');
  assert.equal(gradesForYear(data, '4').total, 0);
  assert.equal(gradesForYear(data, 'all').total, 6);
});
function compileView(source, data) {
  const file = ts.createSourceFile('view.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const names = new Set(['Icon', 'ScoreRing', 'Grades']);
  const parts = [];
  for (const node of file.statements) {
    if (ts.isFunctionDeclaration(node) && names.has(node.name?.text)) parts.push(node.getText(file));
    if (ts.isVariableStatement(node) && node.declarationList.declarations.some(d => ['iconPaths', 'subjects'].includes(d.name.getText(file)))) parts.push(node.getText(file));
  }
  const js = ts.transpileModule(parts.join('\n'), { compilerOptions: { jsx: ts.JsxEmit.React, target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.None } }).outputText;
  return new Function('React', 'useState', 'useEffect', 'useGradesData', 'api', 'downloadFile', 'moscowDate', 'gradesForYear', `${js}\nreturn {Grades,subjects:typeof subjects==='undefined'?[]:subjects};`)(React, React.useState, React.useEffect, () => ({ data, error: '' }), async () => { throw new Error('Запрос во время отрисовки'); }, async () => {}, () => '2026-11-19', gradesForYear);
}
test('«Успеваемость» сохраняет исходные теги, классы и стили при тех же записях', () => {
  const baseline = execFileSync('git', ['show', 'HEAD:src/StudentDashboard.tsx'], { encoding: 'utf8' });
  const original = compileView(baseline, null);
  const data = { subjects: original.subjects.map((s, i) => ({ id: String(i), ...s, semester: 1, credit: true, status: 'Зачтено', mark: '✓' })), average: '85', completed: 18, total: 23, periods:{currentYear:1,years:[1,2]}, summary: { accountID: 'a', groupLabel: 'Группа 101', hasUnread: false } };
  const current = compileView(readFileSync(new URL('./src/StudentDashboard.tsx', import.meta.url), 'utf8'), data);
  // Only the explicitly approved transcript-period control is new; remaining structure stays identical.
  const shape = html => html.replace(/<div class="filters"><select[\s\S]*?<\/select>(<button[\s\S]*?<\/button>)<\/div>/g, '$1').replace(/<!--[\s\S]*?-->/g, '').replace(/>[^<]*</g, '><');
  const props = { toast() {}, accountID: 'a', onSummary() {} };
  assert.equal(shape(renderToStaticMarkup(React.createElement(current.Grades, props))), shape(renderToStaticMarkup(React.createElement(original.Grades, props))));
});
test('Отсутствующая оценка отображается прочерком в исходном индикаторе', () => {
  const current = compileView(readFileSync(new URL('./src/StudentDashboard.tsx', import.meta.url), 'utf8'), { ...buildGradesModel([course('a')], []), periods:{currentYear:1,years:[1,2]}, summary: { accountID: 'a' } });
  const html = renderToStaticMarkup(React.createElement(current.Grades, { toast() {}, accountID: 'a', onSummary() {} }));
  assert.ok(html.includes('aria-label="Оценка не выставлена"')); assert.ok(html.includes('Нет оценки')); assert.ok(!html.includes('>86%<'));
});
