const stop = new Set(['а','в','во','где','для','до','если','за','и','из','или','как','на','не','о','об','от','по','при','с','у','что','это','я','мне','можно']);
const concepts = [
    ['справк'], ['заявк','заявлен'], ['выписк','транскрипт'],
    ['оценк','успеваем','аттестац','балл'], ['физкультур','физр','спорт'],
    ['дедлайн','срок','домашн','дз'], ['разговор','чат','переписк','сообщен'],
    ['заброни','брон','резервир'], ['потер','пропаж','находк'],
    ['расписан','заняти'], ['куар','qr'], ['сотрудник','оператор','поддержк'],
    ['оформ','получ','заказ','запрос'],
];
function words(value:string) {
    return value.toLowerCase().replaceAll('ё','е').match(/[\p{L}\p{N}]+/gu)??[];
}
const classPeriodWords=new Set(['пар','пара','пары','пару','паре','парой','парам','парах','парами']);
function stem(value:string) {
    if(classPeriodWords.has(value))value='занятие';
    const concept=concepts.findIndex(group=>group.some(root=>value.startsWith(root)));
    if(concept!==-1)return '#'+concept;
    return value.length>4?value.replace(/(?:иями|ями|ами|ого|ему|ому|ыми|ими|иям|иях|ить|ать|ять|еть|ой|ей|ам|ям|ом|ем|ах|ях|ия|ие|ию|ии|ый|ая|ое|ые|у|ю|а|я|ы|и|е|о|ь)$/u,''):value;
}
export function findFAQ<T extends {question:string;answer:string}>(articles:T[], query:string):T[] {
    const terms=[...new Set(words(query).filter(word=>!stop.has(word)))];
    if(!terms.length)return articles;
    const score=(term:string, tokens:string[])=>{
        if(tokens.includes(term))return 4;
        const root=stem(term);
        return tokens.some(token=>stem(token)===root || (term.length>=4&&token.startsWith(term)))?2:0;
    };
    const ranked=articles.map((article,index)=>{
        const title=words(article.question), answer=words(article.answer);
        const scores=terms.map(term=>score(term,title)*3+score(term,answer));
        return {article,index,total:scores.reduce((sum,value)=>sum+value,0),matched:scores.filter(Boolean).length};
    }).filter(row=>row.total>0);
    const maximum=Math.max(0,...ranked.map(row=>row.matched));
    return ranked.filter(row=>row.matched===maximum)
        .sort((a,b)=>b.total-a.total||a.index-b.index).map(row=>row.article);
}
