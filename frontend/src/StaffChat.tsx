import {ChatPanel} from './ChatPanel';
import type {ReactNode} from 'react';

const staffChatCSS=`
.role-chat .chat-layout{display:grid;grid-template-columns:minmax(190px,.7fr) minmax(0,1.3fr);gap:16px}
.role-chat .card{background:#fff;border:1px solid var(--line);border-radius:20px;box-shadow:var(--shadow)}
.role-chat .chat-list{padding:12px;max-height:650px;overflow:auto}
.role-chat .card-head,.role-chat .chat-head{display:flex;align-items:center;gap:10px}
.role-chat .card-head{justify-content:space-between;margin-bottom:10px}
.role-chat .card-title{font-size:17px;margin:0;font-weight:850}
.role-chat .chat-head{padding:14px;border-bottom:1px solid var(--line)}
.role-chat .chat-person{display:flex;align-items:center;gap:9px;width:100%;padding:8px;border:1px solid transparent;border-radius:12px;text-align:left;background:transparent;color:inherit;font:inherit}
.role-chat button.chat-person{cursor:pointer}
.role-chat .chat-person.active{background:var(--violet-soft);border-color:var(--line)}
.role-chat .mini-avatar{display:grid;place-content:center;flex:0 0 39px;width:39px;height:39px;border-radius:12px;color:#6759d9;background:var(--violet-soft);font-weight:850;font-size:11px}
.role-chat .row-name{font-size:13px;font-weight:800;overflow-wrap:anywhere}
.role-chat .row-sub,.role-chat .caption{font-size:12px;color:var(--muted)}
.role-chat .unread-count{margin-left:auto;border-radius:12px;padding:3px 6px;background:var(--violet);color:white;font-size:10px}
.role-chat .conversation{min-width:0;display:flex;flex-direction:column;min-height:520px}
.role-chat .messages{display:flex;flex:1;flex-direction:column;gap:9px;min-height:220px;max-height:530px;overflow-y:auto;padding:15px;background:#fbfaff}
.role-chat .bubble{max-width:78%;align-self:flex-start;padding:10px 12px;border-radius:14px;background:#fff;border:1px solid var(--line);font-size:13px}
.role-chat .bubble.mine{align-self:flex-end;background:var(--violet-soft)}
.role-chat .chat-compose,.role-chat .tabs{display:flex;gap:8px;align-items:center}
.role-chat .chat-compose{padding:12px;border-top:1px solid var(--line);flex-wrap:wrap}
.role-chat .input{min-width:0;padding:10px 12px;border:1px solid var(--line);border-radius:12px;font:inherit;color:inherit}
.role-chat .chat-compose .input{flex:1 1 160px}
.role-chat .primary,.role-chat .secondary,.role-chat .tab,.role-chat .icon-btn{border:1px solid var(--line);border-radius:12px;background:#fff;color:inherit;font:inherit;padding:8px 11px;cursor:pointer}
.role-chat .primary{background:var(--violet);color:white;font-weight:800}
.role-chat .tab.active,.role-chat .task{background:var(--violet-soft)}
.role-chat .task{padding:10px 14px}
.role-chat .icon-btn{display:grid;place-content:center;width:36px;height:36px;padding:0}
.role-chat button:disabled{opacity:.5;cursor:default}
.role-chat .list{display:grid;gap:4px}
@media(max-width:760px){.role-chat .chat-layout{grid-template-columns:1fr}.role-chat .chat-list{max-height:290px}}
`;

export function StaffChat({accountID,add,info}:{accountID:string;add:ReactNode;info:ReactNode}){
    return <div className="role-chat"><style>{staffChatCSS}</style><ChatPanel accountID={accountID} icons={{add,info}}/></div>;
}
