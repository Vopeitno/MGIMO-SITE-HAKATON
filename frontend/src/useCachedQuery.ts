import {useCallback, useEffect, useRef, useSyncExternalStore} from 'react';
import {getDataContext, onDataContextChange} from '../api';

type Snapshot<T> = {data: T | null; error: string; ready: boolean};
type Entry = {snapshot: Snapshot<unknown>; signature?: string; pending?: Promise<void>};
const empty: Snapshot<never> = {data: null, error: '', ready: false};
const entries = new Map<string, Entry>();
const listeners = new Map<string, Set<() => void>>();
const maxEntries = 100;

function notify(key: string) {
    listeners.get(key)?.forEach(fn => fn());
}

onDataContextChange(() => {
    entries.clear();
    for (const key of listeners.keys()) notify(key);
});

function stateFor(key: string): Snapshot<unknown> {
    return entries.get(key)?.snapshot ?? empty;
}

function entryFor(key: string) {
    const existing = entries.get(key);
    if (existing) return existing;
    if (entries.size >= maxEntries) {
        const unused = [...entries].find(([id, item]) => !listeners.has(id) && !item.pending);
        if (unused) entries.delete(unused[0]);
    }
    const entry: Entry = {snapshot: empty};
    entries.set(key, entry);
    return entry;
}

function publish(key: string, entry: Entry, data: unknown) {
    if (entries.get(key) !== entry) return;
    const signature = JSON.stringify(data);
    if (entry.snapshot.ready && entry.signature === signature && !entry.snapshot.error) return;
    entry.signature = signature;
    entry.snapshot = {data, error: '', ready: true};
    notify(key);
}

function revalidate<T>(key: string, load: () => Promise<T>, fallback: string) {
    const entry = entryFor(key);
    if (entry.pending) return entry.pending;
    const task = Promise.resolve().then(load).then(data => publish(key, entry, data)).catch(error => {
        if (entry.snapshot.ready || entries.get(key) !== entry) return;
        entry.snapshot = {data: null, error: error instanceof Error ? error.message : fallback, ready: false};
        notify(key);
    }).finally(() => {entry.pending = undefined;});
    entry.pending = task;
    return task;
}

// Cache belongs to the current login and stays in memory across view unmounts.
// A failed background fetch leaves the last successful result on screen.
export function useCachedQuery<T>(key: string | null, load: () => Promise<T>, revision = 0, fallback = 'Не удалось загрузить данные', pollMs = 5000) {
    const context = getDataContext();
    const fullKey = key && context ? context + ':' + key : null;
    const latest = useRef(load);
    useEffect(() => {latest.current = load;}, [load]);

    const subscribe = useCallback((fn: () => void) => {
        if (!fullKey) return () => {};
        const group = listeners.get(fullKey) ?? new Set();
        group.add(fn);
        listeners.set(fullKey, group);
        return () => {
            group.delete(fn);
            if (!group.size) listeners.delete(fullKey);
        };
    }, [fullKey]);
    const getSnapshot = useCallback(() => fullKey ? stateFor(fullKey) as Snapshot<T> : empty, [fullKey]);
    const state = useSyncExternalStore(subscribe, getSnapshot, () => empty);
    const reload = useCallback(async () => {
        if (fullKey && getDataContext() === context) await revalidate(fullKey, () => latest.current(), fallback);
    }, [context, fallback, fullKey]);

    useEffect(() => {
        if (!fullKey) return;
        void reload();
        const refreshVisible = () => {if (document.visibilityState === 'visible') void reload();};
        const timer = pollMs > 0 ? window.setInterval(refreshVisible, pollMs) : undefined;
        document.addEventListener('visibilitychange', refreshVisible);
        return () => {
            window.clearInterval(timer);
            document.removeEventListener('visibilitychange', refreshVisible);
        };
    }, [fullKey, pollMs, reload, revision]);

    const update = useCallback((value: T | ((previous: T | null) => T)) => {
        if (!fullKey || getDataContext() !== context) return;
        const entry = entryFor(fullKey);
        const data = typeof value === 'function' ? (value as (previous: T | null) => T)(entry.snapshot.data as T | null) : value;
        // Discard pre-write background responses before updating the visible value.
        const next: Entry = {snapshot: entry.snapshot, signature: entry.signature};
        entries.set(fullKey, next);
        publish(fullKey, next, data);
    }, [context, fullKey]);

    return {...state, reload, update};
}
