import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';

test('Клиент работает через настоящий Vite proxy и Go API', { skip: !process.env.API_TEST_ORIGIN }, async () => {
  const original = globalThis.fetch;
  let cookie = '';
  globalThis.fetch = async (path, init = {}) => {
    const headers = new Headers(init.headers);
    if (cookie) headers.set('Cookie', cookie);
    const result = await original(new URL(path, process.env.API_TEST_ORIGIN), { ...init, headers });
    const setCookie = result.headers.get('set-cookie');
    if (setCookie) cookie = setCookie.includes('Max-Age=0') ? '' : setCookie.split(';')[0];
    return result;
  };
  try {
    const source = ts.transpileModule(readFileSync(new URL('./api.ts', import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
    const client = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
    const user = await client.login('student1@example.test', process.env.DEMO_PASSWORD);
    assert.equal(user.role, 'STUDENT');
    assert.ok(cookie.startsWith('mgimo_refresh='));
    const profile = await client.api('/profile/me');
    assert.equal(profile.student.studentEmail, user.email);
    assert.ok(!JSON.stringify(profile).includes('passport'));
    const before = cookie;
    assert.equal((await client.refresh()).accountID, user.accountID);
    assert.notEqual(cookie, before);
    const types = await client.allPages('/requests/types');
    assert.ok(types.items.length > 0);
    const draft = await client.api('/requests/drafts', 'POST', { appointmentTitle: 'Проверка клиента' });
    await client.api(`/requests/drafts/${draft.appointmentDraftID}`, 'PATCH', { appointmentTypeID: types.items[0].appointmentTypeID, appointmentDescription: 'Проверка через настоящий сервер' });
    const request = await client.api(`/requests/drafts/${draft.appointmentDraftID}/submit`, 'POST');
    assert.equal((await client.api(`/requests/${request.appoinmentID}`)).appointmentStatus, 'creating');
    const schedule = await client.api(`/schedule?date=${new Date().toISOString().slice(0, 10)}`);
    assert.ok(Array.isArray(schedule.lessons));
    if (schedule.lessons.length) assert.ok(schedule.lessons[0].teacher.teacherName);
    await client.logout();
    assert.equal(cookie, '');
    await assert.rejects(client.api('/profile/me'), e => e.status === 401);
  } finally { globalThis.fetch = original; }
});
