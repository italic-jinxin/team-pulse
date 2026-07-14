import type{Row}from'../lib/api';
import{ago}from'../lib/format';
import{Badge,Empty,Stat}from'../components/ui';

export function Repositories({rows,prs,activity}:{rows:Row[];prs:Row[];activity:Row[]}){
 const activeRepos=rows.filter(repo=>activity.some(item=>item.repository===repo.full_name));
 const degraded=rows.filter(repo=>repoCI(repo.full_name,prs)==='degraded');
 return <>
  <section className="grid metrics">
   <Stat label="Tracked Repositories" value={rows.length} detail={`${activeRepos.length} active in sync window`}/>
   <Stat label="PR CI Health" value={rows.length?`${Math.round((rows.length-degraded.length)/rows.length*100)}%`:'—'} detail={`${rows.length-degraded.length} of ${rows.length} without failing PR checks`}/>
   <Stat label="Open Pull Requests" value={prs.filter(pr=>pr.state==='open').length} detail="Across tracked repositories"/>
   <Stat label="CI Degraded" value={degraded.length} detail="Repositories with failing PR checks" tone={degraded.length?'warn':''}/>
  </section>
  <section className="card tablewrap"><div className="card-header"><h2>Repositories</h2><span>Local sync status</span></div>{rows.length?<table className="table"><thead><tr><th>Repository</th><th>Activity</th><th>Open PRs</th><th>Failed CI</th><th>Health</th><th>Last Activity</th></tr></thead><tbody>{rows.map(repo=>{const name=String(repo.full_name);const repoPRs=prs.filter(pr=>pr.repository===name);const open=repoPRs.filter(pr=>pr.state==='open').length;const failed=repoPRs.filter(pr=>String(pr.ci_state||'').includes('fail')).length;const repoActivity=activity.filter(item=>item.repository===name);const latest=latestActivity(repoActivity)||repo.updated_at;const events=repoActivity.length;const ci=repoCI(name,prs);return <tr key={name}><td><strong>{name}</strong><div className="meta">{repo.description||repo.default_branch||'Synced repository'}</div></td><td>{activityLevel(events)}</td><td>{open}</td><td>{failed}</td><td><Badge value={ci==='degraded'?'Needs attention':'Healthy'}/></td><td>{ago(latest)}</td></tr>;})}</tbody></table>:<Empty title="No tracked repositories" text="Connect GitHub in Settings and sync repositories."/>}</section>
 </>;
}

function latestActivity(rows:Row[]){
 return rows.map(row=>String(row.occurred_at||'')).sort().reverse()[0];
}

function repoCI(repo:string,prs:Row[]){
 return prs.some(pr=>pr.repository===repo&&String(pr.ci_state||'').includes('fail'))?'degraded':'passing';
}

function activityLevel(events:number){
 if(events>=20)return'High';
 if(events>=5)return'Medium';
 if(events>0)return'Low';
 return'None';
}
