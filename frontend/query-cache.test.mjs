import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';

const moduleURL = value => 'data:text/javascript;base64,' + Buffer.from(value).toString('base64');
const authURL = moduleURL(`
let notify;
export const getDataContext=()=> 'local-student';
export const onDataContextChange=fn=> {notify=fn};
export const clear=()=>notify();
`);
const reactURL = import.meta.resolve('react');
const source = readFileSync(new URL('./src/useCachedQuery.ts', import.meta.url), 'utf8')
    + '\nexport {stateFor, revalidate};';
const js = ts.transpileModule(source, {compilerOptions:{target:ts.ScriptTarget.ES2023,module:ts.ModuleKind.ESNext}}).outputText
    .replace(/from ['"]react['"]/, `from '${reactURL}'`).replace(/from ['"]\.\.\/api['"]/, `from '${authURL}'`);
const {stateFor,revalidate} = await import(moduleURL(js));
const {clear} = await import(authURL);
const deferred = () => {
    let resolve;
    const promise = new Promise(done => {resolve=done;});
    return {promise,resolve};
};

test('При повторном открытии видны старые данные, новый ответ подставляется один раз', async () => {
    const key='student:schedule';
    await revalidate(key, async () => ['старая пара'], 'Ошибка');
    const next=deferred();
    let calls=0;
    const load = () => {calls++;return next.promise;};
    const first=revalidate(key,load,'Ошибка');
    const second=revalidate(key,load,'Ошибка');
    assert.deepEqual(stateFor(key).data,['старая пара']);
    next.resolve(['новая пара']);
    await Promise.all([first,second]);
    assert.equal(calls,1);
    assert.deepEqual(stateFor(key).data,['новая пара']);
    await revalidate(key, async () => {throw new Error('Офлайн');}, 'Ошибка');
    assert.deepEqual(stateFor(key),{data:['новая пара'],ready:true,error:''});
});

test('Ответ от прежней сессии не восстанавливает очищенный кэш', async () => {
    const key='old:profile';
    const result=deferred();
    const pending=revalidate(key,()=>result.promise,'Ошибка');
    clear();
    result.resolve({name:'Прежний аккаунт'});
    await pending;
    assert.deepEqual(stateFor(key),{data:null,ready:false,error:''});
});
