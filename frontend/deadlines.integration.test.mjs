import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
const require = createRequire(import.meta.url);
const compile = path => ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
const url = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
test('«Дедлайны»: полный список двух студентов и реальные курсы из облачной БД', { skip: !process.env.API_TEST_ORIGIN }, async () => {
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
    const apiURL = url(compile('./api.ts')); const todayURL=url(compile('./src/today-model.ts'));
    const modelURL=url(compile('./src/deadlines-model.ts').replace(/from ['"]\.\/today-model['"]/g,`from '${todayURL}'`));
    const loaderSource=compile('./src/useDeadlinesData.ts').replace(/from ['"]\.\/useCachedQuery['"]/g, "from 'data:text/javascript,export function useCachedQuery(){return {data:null,ready:false,error:%22%22,reload(){},update(){}}}'").replace(/from ['"]\.\.\/api['"]/g,`from '${apiURL}'`).replace(/from ['"]\.\/deadlines-model['"]/g,`from '${modelURL}'`).replace(/from ['"]react['"]/g,`from '${pathToFileURL(require.resolve('react')).href}'`);
    const client=await import(apiURL);const {loadDeadlinesData}=await import(url(loaderSource));
    for(const key of ['STUDENT1','STUDENT2']){
      replies.clear();const user=await client.login(process.env[`DEMO_${key}_EMAIL`],process.env[`DEMO_${key}_PASSWORD`]);
      try{
        const data=await loadDeadlinesData(user.accountID);const raw=replies.get('/api/v1/MGIMO/academic/deadlines');const courses=replies.get('/api/v1/MGIMO/academic/subjects').items;const profile=replies.get('/api/v1/MGIMO/profile/me');
        assert.equal(data.summary.accountID,user.accountID);assert.equal(data.summary.groupLabel,`Группа ${profile.student.group.groupNumber}`);assert.equal(data.items.length,raw.total);
        for(const item of data.items){const actual=raw.items.find(d=>d.deadlineID===item.id);assert.equal(item.title,actual.deadlineTitle);assert.equal(item.description,actual.deadlineDescription);assert.equal(item.endAt,actual.deadlineEndAt);assert.equal(item.submissionURL,actual.submissionURL??null);assert.equal(item.course,courses.find(c=>c.courseID===actual.courseID).title)}
      }finally{await client.logout()}
    }
    const teacherUser=await client.login(process.env.DEMO_TEACHER_EMAIL,process.env.DEMO_TEACHER_PASSWORD);
    try{
      assert.equal(teacherUser.role,'TEACHER');
      const teacherLoader=url(compile('./src/teacher-deadlines-model.ts').replace(/from ['"]\.\.\/api['"]/g,`from '${apiURL}'`));
      const {loadTeacherDeadlines}=await import(teacherLoader);const assignments=await loadTeacherDeadlines();const courses=replies.get('/api/v1/MGIMO/academic/subjects').items;
      assert.ok(assignments.every(d=>courses.some(c=>c.courseID===d.courseID&&c.title===d.courseTitle)));assert.ok(assignments.every(d=>d.submissionURL?.startsWith('https://')));
    }finally{await client.logout()}
  }finally{globalThis.fetch=original}
});
