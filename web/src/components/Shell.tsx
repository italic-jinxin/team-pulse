import type{ReactNode}from'react';
import{CircleAlert,RefreshCw}from'lucide-react';
import{nav,type PageName,subtitle}from'../app/navigation';
import type{SyncJob}from'../lib/api';
import{SyncDetails}from'./SyncDetails';

export function Shell({page,setPage,connected,riskCount,openPRCount,repoCount,lastSync,refreshing,syncing,activeJob,syncSelection,error,onRefresh,onSync,children}:{page:PageName;setPage:(page:PageName)=>void;connected:boolean;riskCount:number;openPRCount:number;repoCount:number;lastSync:string;refreshing:boolean;syncing:boolean;activeJob?:SyncJob;syncSelection?:string[];error:string;onRefresh:()=>void;onSync:()=>void;children:ReactNode}){
 const syncLabel=!connected?'Connect GitHub':repoCount===0?'Choose repositories':syncing?'Syncing…':'Sync GitHub';
 return <div className="shell">
  <aside aria-label="Primary navigation">
   <div className="brand"><span>TP</span><div><strong>TeamPulse</strong><small>Local-first GitHub Insights</small></div></div>
   <div className="server-card"><div className="server-row"><strong>Local Server</strong><span className="status"><i/>Running</span></div><div className="server-url">http://127.0.0.1:19421</div></div>
   <nav>{nav.map(([name,Icon])=><button aria-current={page===name?'page':undefined} className={page===name?'active':''} onClick={()=>setPage(name)} key={name}><span><Icon size={16}/>{name}</span>{badge(name,riskCount,openPRCount,repoCount)}</button>)}</nav>
   <div className="connection">Data is stored locally.<br/>GitHub sync: {lastSync}</div>
  </aside>
  <main>
   <header>
    <div>
     <h1>{title(page)}</h1>
     <p>{subtitle(page)}</p>
    </div>
    <div className="actions"><span className="sync-text">Last sync: {lastSync}</span><button className="secondary" onClick={onRefresh} disabled={refreshing} aria-label="Refresh dashboard data"><RefreshCw size={16} className={refreshing?'spin':''}/>{refreshing?'Refreshing':'Server Status'}</button><button onClick={onSync} disabled={syncing}>{syncLabel}</button></div>
   </header>
   {error&&<div className="error" role="alert"><CircleAlert size={17}/><span>{error}</span></div>}
   {activeJob&&<SyncDetails job={activeJob} repositories={syncSelection||[]}/>}
   {children}
  </main>
 </div>;
}

function badge(name:PageName,risks:number,prs:number,repos:number){
 const value=name==='Risks'?risks:name==='Pull Requests'?prs:name==='Repositories'?repos:0;
 return value?<b>{value}</b>:null;
}

function title(page:PageName){
 return {
  Overview:'Engineering Overview',
  Activity:'Activity Timeline',
  'Pull Requests':'Pull Requests',
  Team:'Team Activity',
  Repositories:'Repositories',
  Risks:'Risk Signals',
  Reports:'Reports',
  Settings:'Settings',
 }[page];
}
