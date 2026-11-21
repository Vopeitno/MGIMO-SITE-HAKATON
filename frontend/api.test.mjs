import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
const source = ts.transpileModule(readFileSync(new URL('./api.ts', import.meta.url), 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2023, module: ts.ModuleKind.ESNext } }).outputText;
let counter = 0;
const client = () => import(`data:text/javascript;base64,${Buffer.from(source + `\n// ${counter++}`).toString('base64')}`);
const user = { accountID: 'account', role: 'STUDENT', email: 'a@example.test', displayName: 'Студент' };
const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const session = (accessToken = 'token') => ({ accessToken, user });

test('Вход передаёт email/password, доверяет серверной роли и использует токен только в заголовке', async () => {
  const api = await client(); const calls = [];
  const original = globalThis.fetch;
  globalThis.fetch = async (path, init) => { calls.push({ path, init }); return json(path.endsWith('/login') ? { ...session(), user: { ...user, role: 'DEAN' } } : { ok: true }); };
  try {
    assert.equal((await api.login(user.email, 'password')).role, 'DEAN');
    await api.api('/profile/me');
    assert.deepEqual(JSON.parse(calls[0].init.body), { email: user.email, password: 'password' });
    assert.equal(calls[0].path, '/api/v1/MGIMO/auth/login');
    assert.equal(calls[1].init.headers.get('Authorization'), 'Bearer token');
    assert.ok(calls.every(c => c.init.credentials === 'same-origin'));
  } finally { globalThis.fetch = original; }
});
test('Параллельные 401 используют одну ротацию refresh и повторяют запрос с новым токеном', async () => {
  const api = await client(); let rotations = 0; const original = globalThis.fetch;
  globalThis.fetch = async (path, init) => {
    if (path.endsWith('/login')) return json(session('old'));
    if (path.endsWith('/refresh')) { rotations++; await new Promise(resolve => setTimeout(resolve, 10)); assert.equal(init.method, 'POST'); return json(session('new')); }
    return init.headers.get('Authorization') === 'Bearer new' ? json({ ok: true }) : json({ message: 'Истёк токен' }, 401);
  };
  try { await api.login(user.email, 'password'); await Promise.all([api.api('/profile/me'), api.api('/notifications')]); assert.equal(rotations, 1); }
  finally { globalThis.fetch = original; }
});
test('Отказ refresh завершает сессию и не зацикливает запросы', async () => {
  const api = await client(); let expired = 0; let calls = 0; const original = globalThis.fetch;
  api.onSessionExpired(() => expired++);
  globalThis.fetch = async () => { calls++; return json({ message: 'Сессия завершена' }, 401); };
  try { await assert.rejects(api.api('/profile/me'), /Сессия завершена/); assert.equal(calls, 2); assert.equal(expired, 1); }
  finally { globalThis.fetch = original; }
});
test('Выход вызывает сервер и удаляет access token; 204 не разбирается как JSON', async () => {
  const api = await client(); const calls = []; const original = globalThis.fetch;
  globalThis.fetch = async (path, init) => { calls.push({ path, init }); return path.endsWith('/login') ? json(session()) : path.endsWith('/logout') ? new Response(null, { status: 204 }) : json({}); };
  try { await api.login(user.email, 'password'); await api.logout(); await api.api('/profile/me'); assert.equal(calls[1].init.method, 'POST'); assert.equal(calls[1].path, '/api/v1/MGIMO/auth/logout'); assert.equal(calls[2].init.headers.has('Authorization'), false); }
  finally { globalThis.fetch = original; }
});
test('Вложения отправляются как FormData без принудительного Content-Type; ошибки сервера сохраняются', async () => {
  const api = await client(); const original = globalThis.fetch;
  globalThis.fetch = async (_path, init) => { assert.ok(init.body instanceof FormData); assert.equal(init.headers.has('Content-Type'), false); return json({ message: 'Нет свободных мест' }, 409); };
  try { await assert.rejects(api.api('/requests/id/attachment', 'POST', new FormData()), error => error.status === 409 && error.message === 'Нет свободных мест'); }
  finally { globalThis.fetch = original; }
});
test('Связанные записи загружаются со всех страниц', async () => {
  const api = await client(); const original = globalThis.fetch; const offsets = [];
  globalThis.fetch = async path => { const offset = Number(new URL(path, 'http://localhost').searchParams.get('offset')); offsets.push(offset); return json({ items: [{ id: offset }], total: 3, limit: 1, offset }); };
  try { const page = await api.allPages('/sports/registrations/me'); assert.equal(page.items.length, 3); assert.deepEqual(offsets, [0, 1, 2]); }
  finally { globalThis.fetch = original; }
});
