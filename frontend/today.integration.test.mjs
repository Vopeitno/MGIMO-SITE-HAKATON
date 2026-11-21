import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const compile = path => ts.transpileModule(readFileSync(new URL(path, import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
const moduleURL = source => `data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;

test('«Сегодня»: настоящий загрузчик получает персональные данные через Go и облачную БД', { skip: !process.env.API_TEST_ORIGIN }, async () => {
  const originalFetch = globalThis.fetch;
  const replies = new Map();
  const paths = new Set();
  let cookie = '';
  globalThis.fetch = async (path, init = {}) => {
    const url = new URL(path, process.env.API_TEST_ORIGIN);
    paths.add(url.pathname);
    const headers = new Headers(init.headers);
    headers.set('Origin', process.env.API_TEST_ORIGIN);
    if (cookie) headers.set('Cookie', cookie);
    const response = await originalFetch(url, { ...init, headers });
    const setCookie = response.headers.get('set-cookie');
    if (setCookie) cookie = setCookie.includes('Max-Age=0') ? '' : setCookie.split(';')[0];
    if ((!init.method || init.method === 'GET') && response.ok) replies.set(url.pathname, await response.clone().json());
    return response;
  };
  try {
    const apiURL = moduleURL(compile('./api.ts'));
    const modelURL = moduleURL(compile('./src/today-model.ts'));
    const loaderSource = compile('./src/useTodayData.ts').replace(/from ['"]\.\/useCachedQuery['"]/g, "from 'data:text/javascript,export function useCachedQuery(){return {data:null,ready:false,error:%22%22,reload(){},update(){}}}'")
      .replace(/from ['"]\.\.\/api['"]/g, `from '${apiURL}'`)
      .replace(/from ['"]\.\/today-model['"]/g, `from '${modelURL}'`)
      .replace(/from ['"]react['"]/g, `from '${pathToFileURL(require.resolve('react')).href}'`);
    const client = await import(apiURL);
    const { loadTodayData } = await import(moduleURL(loaderSource));
    let firstAccount = '';
    for (const key of ['STUDENT1', 'STUDENT2']) {
      replies.clear(); paths.clear();
      const user = await client.login(process.env[`DEMO_${key}_EMAIL`], process.env[`DEMO_${key}_PASSWORD`]);
      assert.equal(user.role, 'STUDENT');
      const data = await loadTodayData();
      assert.equal(data.summary.accountID, user.accountID);
      const dashboard = replies.get('/api/v1/MGIMO/dashboard');
      const profile = replies.get('/api/v1/MGIMO/profile/me');
      const schedule = replies.get('/api/v1/MGIMO/schedule');
      assert.equal(data.summary.groupLabel, `Группа ${profile.student.group.groupNumber}`);
      assert.equal(data.lessons.length, schedule.lessons.length);
      assert.deepEqual(data.requests.map(r => r.id), dashboard.appointments.map(r => r.appoinmentID));
      assert.equal(data.courseCount, replies.get('/api/v1/MGIMO/academic/subjects').total);
      if (firstAccount) assert.notEqual(user.accountID, firstAccount); else firstAccount = user.accountID;
      for (const suffix of ['/dashboard', '/schedule', '/academic/subjects', '/academic/grades', '/profile/me', '/notifications']) assert.ok(paths.has('/api/v1/MGIMO' + suffix), `Не запрошен ${suffix}`);
      await client.logout();
    }
  } finally { globalThis.fetch = originalFetch; }
});
