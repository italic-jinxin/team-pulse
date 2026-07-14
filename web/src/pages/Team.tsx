import{useMemo,useState}from'react';
import type{Row}from'../lib/api';
import{ago,initials}from'../lib/format';
import{Empty}from'../components/ui';
import{Dropdown}from'../components/Dropdown';

type MemberStats=Row&{commits:number;pull_requests:number;reviews:number;activity_count:number};
type MemberSummary={
 activeDays:number;
 avgPerActiveDay:string;
 repoCount:number;
 topRepos:{name:string;count:number;percent:number}[];
 mix:{label:string;count:number;percent:number}[];
 recent:Row[];
 focus:string;
 collaboration:string;
 recommendation:string;
 lastActive:string;
};

export function Team({rows,activity}:{rows:Row[];activity:Row[]}){
 const[range,setRange]=useState('30');
 const members=useMemo(()=>deriveMembers(rows,activity,range),[rows,activity,range]);
 const[selectedLogin,setSelectedLogin]=useState(String(rows[0]?.login||''));
 const selected=useMemo(()=>members.find(row=>row.login===selectedLogin)||members[0],[members,selectedLogin]);
 const selectedActivity=useMemo(()=>selected?activityFor(selected.login,activity,range):[],[selected?.login,activity,range]);
 const summary=useMemo(()=>selected?buildSummary(selected,selectedActivity):null,[selected,selectedActivity]);
 return <div className="split">
  <section className="card"><div className="card-header"><h2>Team Members</h2><span>{members.length} active · {rangeLabel(range)}</span></div><div className="filters team-filters"><Dropdown value={range} onChange={setRange} ariaLabel="Team activity time range" options={[{value:'1',label:'Today'},{value:'7',label:'7 days'},{value:'30',label:'30 days'},{value:'all',label:'All time'}]}/></div>{members.length?<ul className="member-list">{members.map(member=><li className={'member-item member-select '+(selected?.login===member.login?'selected':'')} key={member.login} onClick={()=>setSelectedLogin(String(member.login))}><div className="member-top"><span className="member-title">{member.login}</span><span className="pill blue">{focusLabel(member)}</span></div><div className="member-desc">{member.commits||0} commits · {member.pull_requests||0} PRs · {member.reviews||0} reviews · {member.activity_count||0} total events · last active {ago(member.last_active_at)}</div></li>)}</ul>:<Empty title="No members found" text="No contributor activity in this time range."/>}</section>
  <div className="grid">
   <section className="card"><div className="card-header"><h2>Member Summary</h2><span>{selected?.login||'—'}</span></div>{selected&&summary?<div className="detail-panel member-summary">
    <div className="activity-row"><div className="avatar">{initials(selected.login)}</div><div className="activity-copy"><h3>{summary.focus}</h3><p>{summary.recommendation}</p></div></div>
    <div className="kpi-strip member-kpis"><div className="mini-kpi"><strong>{selected.commits||0}</strong><span>Commits</span></div><div className="mini-kpi"><strong>{selected.pull_requests||0}</strong><span>PRs</span></div><div className="mini-kpi"><strong>{selected.reviews||0}</strong><span>Reviews</span></div><div className="mini-kpi"><strong>{summary.activeDays}</strong><span>Active days</span></div></div>
    <div className="summary-grid">
     <div className="insight-box"><span>Cadence</span><strong>{summary.avgPerActiveDay} events / active day</strong><p>{summary.lastActive}</p></div>
     <div className="insight-box"><span>Collaboration mode</span><strong>{summary.collaboration}</strong><p>{selected.reviews||0} reviews across {selected.pull_requests||0} PR events.</p></div>
    </div>
    <div className="summary-section"><h3>Primary repositories</h3>{summary.topRepos.length?<div className="repo-chips">{summary.topRepos.map(repo=><span key={repo.name}><b>{repo.name}</b><i>{repo.count}</i></span>)}</div>:<p>No repository activity in this range.</p>}</div>
    <div className="summary-section"><h3>Activity mix</h3><div className="activity-mix">{summary.mix.map(item=><div className="mix-row" key={item.label}><span>{item.label}</span><div className="bar"><span style={{width:`${item.percent}%`}}/></div><strong>{item.count}</strong></div>)}</div></div>
    <div className="summary-section"><h3>Recent signals</h3>{summary.recent.length?<ul className="recent-mini">{summary.recent.map((item,index)=><li key={item.id||index}><b>{activityLabel(item.type)}</b><span>{item.repository||'Unknown repo'} · {ago(item.occurred_at)}</span><p>{item.title||'Repository activity'}</p></li>)}</ul>:<p>No recent events in this range.</p>}</div>
   </div>:<Empty title="Select a member" text="Choose a contributor for details." compact/>}</section>
   <section className="card"><div className="card-header"><h2>Collaboration</h2><span>Review interactions</span></div><div className="card-body stack"><div className="tag-row"><span className="pill blue">Reviews · {selected?.reviews||0}</span><span className="pill neutral">Pull requests · {selected?.pull_requests||0}</span><span className="pill neutral">Commits · {selected?.commits||0}</span><span className="pill neutral">Repos · {summary?.repoCount||0}</span></div><p className="settings-note">{summary?.collaboration||'Select a contributor to inspect collaboration signals.'}</p></div></section>
  </div>
 </div>;
}

function deriveMembers(rows:Row[],activity:Row[],range:string):MemberStats[]{
 const byLogin=new Map<string,MemberStats>();
 rows.forEach(row=>byLogin.set(String(row.login),{...row,commits:0,pull_requests:0,reviews:0,activity_count:0}));
 const filtered=activity.filter(item=>inRange(item.occurred_at,range));
 filtered.forEach(item=>{
  const login=String(item.actor||'GitHub');
  const current=byLogin.get(login)||{login,avatar_url:'',commits:0,pull_requests:0,reviews:0,last_active_at:'',activity_count:0};
  const type=String(item.type||'');
  if(type.includes('commit'))current.commits+=1;
  if(type.includes('pr.')&&!type.includes('review'))current.pull_requests+=1;
  if(type.includes('review'))current.reviews+=1;
  current.activity_count+=1;
  if(!current.last_active_at||String(item.occurred_at)>String(current.last_active_at))current.last_active_at=item.occurred_at;
  byLogin.set(login,current);
 });
 const derived=Array.from(byLogin.values()).filter(member=>range==='all'||member.activity_count>0);
 if(filtered.length===0&&range==='all'){
  return rows.map(row=>({...row,commits:Number(row.commits||0),pull_requests:Number(row.pull_requests||0),reviews:Number(row.reviews||0),activity_count:Number(row.commits||0)+Number(row.pull_requests||0)+Number(row.reviews||0)}));
 }
 return derived.sort((a,b)=>Number(b.activity_count||0)-Number(a.activity_count||0)||String(b.last_active_at||'').localeCompare(String(a.last_active_at||'')));
}

function inRange(value:string,range:string){
 if(range==='all')return true;
 const time=new Date(value).getTime();
 return Number.isFinite(time)&&Date.now()-time<=Number(range)*864e5;
}

function activityFor(login:unknown,activity:Row[],range:string){
 return activity.filter(item=>String(item.actor||'')===String(login)&&inRange(item.occurred_at,range)).sort((a,b)=>String(b.occurred_at||'').localeCompare(String(a.occurred_at||'')));
}

function buildSummary(member:MemberStats,events:Row[]):MemberSummary{
 const repoCounts=countBy(events,event=>String(event.repository||'Unknown repo'));
 const typeCounts=countBy(events,event=>bucketType(String(event.type||'')));
 const activeDays=new Set(events.map(event=>String(event.occurred_at||'').slice(0,10)).filter(Boolean)).size;
 const total=Number(member.activity_count||events.length||0);
 const maxType=Math.max(1,...Array.from(typeCounts.values()));
 const maxRepo=Math.max(1,...Array.from(repoCounts.values()));
 const reviews=Number(member.reviews||0);
 const prs=Number(member.pull_requests||0);
 const commits=Number(member.commits||0);
 return {
  activeDays,
  avgPerActiveDay:activeDays?Number(total/activeDays).toFixed(1):'0',
  repoCount:repoCounts.size,
  topRepos:Array.from(repoCounts.entries()).sort((a,b)=>b[1]-a[1]).slice(0,4).map(([name,count])=>({name,count,percent:Math.round(count/maxRepo*100)})),
  mix:['Commits','Pull requests','Reviews','Other'].map(label=>{const count=typeCounts.get(label)||0;return{label,count,percent:Math.round(count/maxType*100)}}),
  recent:events.slice(0,4),
  focus:focusLabel(member),
  collaboration:collaborationMode(commits,prs,reviews),
  recommendation:summaryCopy(member,events.length),
  lastActive:member.last_active_at?`Last active ${ago(member.last_active_at)}.`:'No recent activity captured.',
 };
}

function countBy<T>(items:T[],keyFor:(item:T)=>string){
 const counts=new Map<string,number>();
 items.forEach(item=>counts.set(keyFor(item),(counts.get(keyFor(item))||0)+1));
 return counts;
}

function bucketType(type:string){
 if(type.includes('commit'))return'Commits';
 if(type.includes('review'))return'Reviews';
 if(type.includes('pr.'))return'Pull requests';
 return'Other';
}

function activityLabel(value:unknown){
 const type=String(value||'activity');
 if(type.includes('commit'))return'Commit';
 if(type.includes('review'))return'Review';
 if(type.includes('merged'))return'PR merged';
 if(type.includes('pr.'))return'PR update';
 return type.replaceAll('.',' ');
}

function rangeLabel(range:string){
 if(range==='1')return'today';
 if(range==='7')return'last 7 days';
 if(range==='30')return'last 30 days';
 return'all time';
}

function focusLabel(member:Row){
 if(Number(member.reviews||0)>Number(member.commits||0))return'Review focus';
 if(Number(member.pull_requests||0)>0)return'Implementation';
 return'Activity';
}

function focusCopy(member:Row){
 return `${member.login} is currently visible through ${member.commits||0} commits, ${member.pull_requests||0} pull requests, and ${member.reviews||0} reviews in synced repositories.`;
}

function collaborationMode(commits:number,prs:number,reviews:number){
 const total=Math.max(1,commits+prs+reviews);
 if(reviews/total>=0.45)return'Review-heavy contributor; likely helping unblock teammates.';
 if((commits+prs)/total>=0.7&&reviews<=1)return'Implementation-heavy contributor; review load appears low.';
 if(prs>0&&reviews>0)return'Balanced builder/reviewer activity.';
 if(commits>0)return'Code contribution visible; limited PR/review signals in this range.';
 return'Low visible activity in the selected range.';
}

function summaryCopy(member:Row,eventCount:number){
 const commits=Number(member.commits||0),prs=Number(member.pull_requests||0),reviews=Number(member.reviews||0);
 if(eventCount===0)return`${member.login} has no captured GitHub events in this time range. Expand the range or sync more repositories.`;
 if(reviews>commits+prs)return`${member.login} is primarily contributing through reviews. This is useful for flow health; check whether review load is concentrated on this person.`;
 if(prs>=2&&reviews===0)return`${member.login} is driving implementation work, but review participation is low in this range. Consider balancing review ownership.`;
 if(commits>=5)return`${member.login} is actively shipping code. Look at recent repositories and PR signals to understand current delivery focus.`;
 return focusCopy(member);
}
