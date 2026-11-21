import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import ts from 'typescript';

const source=readFileSync(new URL('./src/faq-search.ts',import.meta.url),'utf8');
const js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {findFAQ}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
const articles=[
  {question:'Где посмотреть статус заявления на справку?',answer:'Откройте раздел «Заявки».'},
  {question:'Как найти номер зачётной книжки?',answer:'Откройте профиль.'},
  {question:'Как записаться на физкультуру?',answer:'Откройте календарь в «Обучении».'},
];
test('Поиск FAQ узнаёт падежи, близкие слова и замену ё на е',()=>{
  for(const query of ['справка','заявка','статус заявки','как получить справку'])assert.equal(findFAQ(articles,query)[0],articles[0]);
  assert.equal(findFAQ(articles,'зачетная книжка')[0],articles[1]);
  assert.equal(findFAQ(articles,'физра')[0],articles[2]);
  assert.deepEqual(findFAQ(articles,'бессмысленный запрос'),[]);
});
