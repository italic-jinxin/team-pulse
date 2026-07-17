import{useEffect,useMemo,useState}from'react';
import{useMutation,useQueryClient}from'@tanstack/react-query';
import type{Row}from'../lib/api';
import{api,queryKeys}from'../lib/api';
import{Badge,Empty}from'../components/ui';

type RiskRules={waiting_review_hours:number;stale_pr_days:number;ci_failure_threshold:number};
const defaults:RiskRules={waiting_review_hours:48,stale_pr_days:5,ci_failure_threshold:1};

export function Risks({rows,settings}:{rows:Row[];settings:Row}){
 const[showLow,setShowLow]=useState(false);
 const[rules,setRules]=useState<RiskRules>(defaults);
 const queryClient=useQueryClient();
 useEffect(()=>{if(settings?.risk_rules)setRules(settings.risk_rules as RiskRules);},[settings]);
 const resolveRisk=useMutation({
  mutationFn:(id:string)=>api(`/risks/${encodeURIComponent(id)}`,{method:'PATCH',json:{status:'resolved'}}),
  onSuccess:()=>queryClient.invalidateQueries({queryKey:queryKeys.dashboard}),
 });
 const saveRules=useMutation({
  mutationFn:()=>api('/settings',{method:'PATCH',json:{version:Number(settings.version||1),changes:{risk_rules:rules}}}),
  onSuccess:()=>queryClient.invalidateQueries({queryKey:queryKeys.dashboard}),
 });

 const visibleRows=showLow?rows:rows.filter(risk=>risk.severity!=='low');
 const groups=useMemo(()=>groupRisks(visibleRows),[visibleRows]);
 return <div className="split">
  <section className="card tablewrap"><div className="card-header"><h2>Open Risk Signals</h2><span>{showLow?'All severities':'High / Medium only'}</span></div><div className="risk-toolbar"><div><strong>{groups.length} work items</strong><p>{rows.length} raw signals grouped by repository and PR.</p></div><button className="secondary compactbtn" onClick={()=>setShowLow(value=>!value)}>{showLow?'Hide low severity':'Show low severity'}</button></div>{groups.length?<table className="table"><thead><tr><th>Priority</th><th>Work item</th><th>Why this matters</th><th>Next step</th><th>Action</th></tr></thead><tbody>{groups.map(group=><tr key={group.key}><td><Badge value={group.severity}/></td><td><strong>{group.title}</strong><div className="meta">{group.repository}{group.pr_number?` #${group.pr_number}`:''} · {group.risks.length} signal{group.risks.length>1?'s':''}</div></td><td>{group.risks.slice(0,2).map(risk=><div key={risk.id}>{risk.reason}</div>)}</td><td>{group.nextStep}</td><td><button className="secondary compactbtn" disabled={resolveRisk.isPending} onClick={()=>group.risks.forEach(risk=>resolveRisk.mutate(String(risk.id)))}>Resolve group</button></td></tr>)}</tbody></table>:<Empty title="No priority risks" text={rows.length?'Low-severity risks are hidden. Use Show low severity to review them.':'No review waits, stale pull requests, or current-head CI failures are open.'}/>}</section>
  <div className="grid">
   <section className="card"><div className="card-header"><h2>Risk Rules</h2><span>Version {settings.version||1}</span></div><div className="card-body stack"><RuleInput title="Waiting for review" detail="Trigger after this many hours" value={rules.waiting_review_hours} suffix="hours" onChange={value=>setRules({...rules,waiting_review_hours:value})}/><RuleInput title="Stale pull request" detail="Trigger after this many inactive days" value={rules.stale_pr_days} suffix="days" onChange={value=>setRules({...rules,stale_pr_days:value})}/><RuleInput title="CI failure threshold" detail="Trigger after this many failed checks/runs" value={rules.ci_failure_threshold} suffix="failure(s)" onChange={value=>setRules({...rules,ci_failure_threshold:value})}/><button disabled={saveRules.isPending} onClick={()=>saveRules.mutate()}>{saveRules.isPending?'Saving':'Save risk rules'}</button>{saveRules.error?<p className="inlineerror">{String(saveRules.error)}</p>:null}</div></section>
   <section className="card"><div className="card-header"><h2>Risk Mix</h2><span>Current open signals</span></div><div className="focus-list">{riskCounts(rows).map(item=><div className="focus-row" key={item.label}><span>{item.label}</span><div className="bar"><span style={{width:`${item.percent}%`}}/></div><strong>{item.count}</strong></div>)}</div></section>
  </div>
 </div>;
}

type RiskGroup={key:string;repository:string;pr_number?:unknown;severity:string;title:string;nextStep:string;risks:Row[]};

function groupRisks(rows:Row[]):RiskGroup[]{
 const groups=new Map<string,RiskGroup>();
 rows.forEach(risk=>{
  const key=`${risk.repository||'unknown'}:${risk.pr_number||risk.type||risk.id}`;
  const current=groups.get(key);
  if(current){
   current.risks.push(risk);
   current.severity=highest(current.severity,String(risk.severity));
   current.title=titleFor(current);
   current.nextStep=nextStepFor(current.risks);
   return;
  }
  const group={key,repository:String(risk.repository||'Unknown repo'),pr_number:risk.pr_number,severity:String(risk.severity||'medium'),title:String(risk.reason||risk.type),nextStep:String(risk.suggested_action||'Review the signal'),risks:[risk]};
  group.title=titleFor(group);
  groups.set(key,group);
 });
 return Array.from(groups.values()).sort((a,b)=>rank(a.severity)-rank(b.severity)||b.risks.length-a.risks.length);
}

function highest(a:string,b:string){return rank(a)<=rank(b)?a:b;}
function rank(severity:string){return severity==='high'?1:severity==='medium'?2:3;}
function titleFor(group:RiskGroup){return group.pr_number?`PR #${group.pr_number} needs attention`:group.repository;}
function nextStepFor(risks:Row[]){return String(risks.find(risk=>risk.severity==='high')?.suggested_action||risks[0]?.suggested_action||'Review the grouped signals');}

function RuleInput({title,detail,value,suffix,onChange}:{title:string;detail:string;value:number;suffix:string;onChange:(value:number)=>void}){
 return <div className="setting-row"><div className="setting-copy"><strong>{title}</strong><span>{detail}</span></div><label className="number-field"><input type="number" min={1} value={value} onChange={event=>onChange(Math.max(1,Number(event.target.value)||1))}/><span>{suffix}</span></label></div>;
}

function riskCounts(rows:Row[]){
 const severities=['high','medium','low'];
 const max=Math.max(1,...severities.map(severity=>rows.filter(row=>row.severity===severity).length));
 return severities.map(label=>{const count=rows.filter(row=>row.severity===label).length;return{label,count,percent:Math.round(count/max*100)};});
}
