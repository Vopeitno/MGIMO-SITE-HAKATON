import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
const require = createRequire(import.meta.url);
const compile = path => ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
const url = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
test('«Успеваемость»: баллы, преподаватели и группы двух студентов читаются из облачной БД', { skip: !process.env.API_TEST_ORIGIN }, async () => {
  const original = globalThis.fetch; let cookie = ''; const replies = new Map();
  globalThis.fetch = async (path, init = {}) => {
    const target = new URL(path, process.env.API_TEST_ORIGIN); const headers = new Headers(init.headers); headers.set('Origin', process.env.API_TEST_ORIGIN);
    if (cookie) headers.set('Cookie', cookie);
    const result = await original(target, { ...init, headers }); const setCookie = result.headers.get('set-cookie');
    if (setCookie) cookie = setCookie.includes('Max-Age=0') ? '' : setCookie.split(';')[0];
    if ((!init.method || init.method === 'GET') && result.ok) replies.set(target.pathname, await result.clone().json());
    return result;
  };
  try {
    const apiURL = url(compile('./api.ts')); const modelURL = url(compile('./src/grades-model.ts'));
    const loaderSource = compile('./src/useGradesData.ts').replace(/from ['"]\.\/useCachedQuery['"]/g, "from 'data:text/javascript,export function useCachedQuery(){return {data:null,ready:false,error:%22%22,reload(){},update(){}}}'").replace(/from ['"]\.\.\/api['"]/g, `from '${apiURL}'`).replace(/from ['"]\.\/grades-model['"]/g, `from '${modelURL}'`).replace(/from ['"]react['"]/g, `from '${pathToFileURL(require.resolve('react')).href}'`);
    const client = await import(apiURL); const { loadGradesData } = await import(url(loaderSource));
    for (const key of ['STUDENT1', 'STUDENT2']) {
      replies.clear(); const user = await client.login(process.env[`DEMO_${key}_EMAIL`], process.env[`DEMO_${key}_PASSWORD`]);
      const data = await loadGradesData(user.accountID); const courses = replies.get('/api/v1/MGIMO/academic/subjects').items; const grades = replies.get('/api/v1/MGIMO/academic/grades').items;
      const profile = replies.get('/api/v1/MGIMO/profile/me'); assert.equal(data.summary.accountID, user.accountID); assert.equal(data.summary.groupLabel, `Группа ${profile.student.group.groupNumber}`);
      assert.deepEqual(data.periods.years, Array.from({length:Math.max(...data.periods.years)},(_,i)=>i+1)); assert.ok(data.periods.years.includes(data.periods.currentYear));
      assert.equal(data.total, courses.length); assert.ok(data.subjects.length > 0);
      const catalog = await client.api('/courses?limit=100&offset=0');
      assert.ok(catalog.items.every(c => Array.isArray(c.teachers)));

      for (const row of data.subjects) {
        const course = courses.find(c => c.courseID === row.id); const grade = grades.find(g => g.courseID === row.id);
        assert.equal(row.s, grade?.score ?? null); assert.equal(row.t, course.teachers.map(t => t.teacherName).join(', ')); assert.ok(course.teachers.length > 0);
        if (grade) assert.equal(grade.studentID, profile.student.studentID);
        for (const teacher of course.teachers) assert.deepEqual(Object.keys(teacher).sort(), ['teacherID', 'teacherName']);
        const details = await client.api(`/academic/subjects/${row.id}/results`); assert.ok(Array.isArray(details.course.teachers));
        assert.ok(details.scores.every(g => g.studentID === profile.student.studentID));
      }
      await client.logout();
    }
  } finally { globalThis.fetch = original; }
});
