export type Role = 'STUDENT' | 'TEACHER' | 'DEAN' | 'SUPPORT' | 'ADMIN';
export type User = {
    accountID: string;
    role: Role;
    email: string;
    displayName: string;
};
type Session = {
    accessToken: string;
    user: User;
};
export type Page<T> = {
    items: T[];
    total: number;
    limit: number;
    offset: number;
};
const prefix = '/api/v1/MGIMO';
let token: string | null = null;
let generation = 0;
let refreshing: Promise<User> | null = null;
const listeners = new Set<() => void>();
const dataListeners = new Set<() => void>();
const readsInFlight = new Map<string, Promise<unknown>>();
let dataContext = '';
export const getDataContext = () => dataContext;
export function onDataContextChange(listener: () => void) {
    dataListeners.add(listener);
    return () => {dataListeners.delete(listener);};
}
function setDataContext(user?: User) {
    const context = user ? `${generation}:${user.accountID}:${user.role}` : '';
    if (context === dataContext) return;
    dataContext = context;
    readsInFlight.clear();
    dataListeners.forEach(listener => listener());
}
export function onSessionExpired(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
export class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) { super(message); this.status = status; }
}
async function checked(response: Response) {
    if (!response.ok) {
        const body = await response.json().catch(() => null);
        throw new ApiError(response.status, body?.message || body?.error?.message || `Ошибка сервера (${response.status})`);
    }
    return response;
}
export async function refresh(): Promise<User> {
    if (!refreshing) {
        const current = generation;
        refreshing = fetch(prefix + '/auth/refresh', { method: 'POST', credentials: 'same-origin' })
            .then(checked).then(r => r.json()).then((session: Session) => {
            if (current !== generation)
                throw new Error('Сессия изменена');
            token = session.accessToken;
            setDataContext(session.user);
            return session.user;
        }).catch(error => {
            if (current === generation && error instanceof ApiError && error.status === 401) {
                token = null;
                setDataContext();
                listeners.forEach(listener => listener());
            }
            throw error;
        }).finally(() => { refreshing = null; });
    }
    return refreshing;
}
export async function login(email: string, password: string): Promise<User> {
    generation++;
    setDataContext();
    const attempt = generation;
    token = null;
    const response = await checked(await fetch(prefix + '/auth/login', {
        method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, password }),
    }));
    const session: Session = await response.json();
    if (attempt !== generation) throw new Error('Сессия изменена');
    token = session.accessToken;
    setDataContext(session.user);
    return session.user;
}
export async function response(path: string, init: RequestInit = {}, retry = true): Promise<Response> {
    const current = generation;
    const headers = new Headers(init.headers);
    if (token)
        headers.set('Authorization', `Bearer ${token}`);
    if (init.body && !(init.body instanceof FormData))
        headers.set('Content-Type', 'application/json');
    const result = await fetch(prefix + path, { ...init, headers, credentials: 'same-origin' });
    if (current !== generation) throw new Error('Сессия изменена');
    if (result.status === 401 && retry) {
        await refresh();
        return response(path, init, false);
    }
    if (result.status === 401) {
        token = null;
        setDataContext();
        listeners.forEach(listener => listener());
    }
    return checked(result);
}
export async function api<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
    const read = method === 'GET' && !!dataContext;
    const key = dataContext + ':' + path;
    if (read) {
        const pending = readsInFlight.get(key);
        if (pending) return pending as Promise<T>;
    }
    const load = response(path, { method, body: body === undefined ? undefined : body instanceof FormData ? body : JSON.stringify(body) })
        .then(result => result.status === 204 ? undefined as T : result.json() as Promise<T>);
    if (!read) {
        const result = await load;
        if (method !== 'GET' && key.startsWith(dataContext + ':')) readsInFlight.clear();
        return result;
    }
    readsInFlight.set(key, load);
    try {return await load;}
    finally {
        if (readsInFlight.get(key) === load) readsInFlight.delete(key);
    }
}
export async function logout() {
    await response('/auth/logout', { method: 'POST' });
    generation++;
    token = null;
    setDataContext();
}
export async function download(path: string, filename: string) {
    const result = await response(path);
    const url = URL.createObjectURL(await result.blob());
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
}
// Справочники и связанные записи нужны целиком, иначе запись за пределами
// первой страницы могла бы ошибочно выглядеть отсутствующей.
export async function allPages<T>(path: string): Promise<Page<T>> {
    const url = new URL(path, 'http://mgimo.local');
    url.searchParams.set('limit', '100');
    url.searchParams.set('offset', '0');
    const first = await api<Page<T>>(url.pathname + url.search);
    const items = [...first.items];
    let offset = first.offset + first.items.length;
    while (offset < first.total) {
        url.searchParams.set('offset', String(offset));
        const page = await api<Page<T>>(url.pathname + url.search);
        if (!page.items.length)
            throw new Error('Не удалось загрузить полный список. Повторите запрос.');
        items.push(...page.items);
        offset += page.items.length;
    }
    return { ...first, items, limit: items.length, offset: 0 };
}
