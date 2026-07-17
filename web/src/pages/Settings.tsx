import{useEffect,useRef,useState}from'react';
import{useMutation,useQuery,useQueryClient}from'@tanstack/react-query';
import{GitPullRequest,RefreshCw,Search}from'lucide-react';
import type{Row,SyncJob}from'../lib/api';
import{api,apiList,cleanError,queryKeys}from'../lib/api';
import{Empty}from'../components/ui';
import{Dropdown}from'../components/Dropdown';
import{SyncDetails}from'../components/SyncDetails';
import{defaultNotifications,requestNotificationPermission,usePreference,type NotificationPrefs}from'../lib/preferences';

const SYNC_POLL_MS=2500;
const SYNC_CLOSE_DELAY_MS=5000;

export function Settings({auth,jobs,externalJobId='',externalSelection=[],onJobStart}:{auth:Row;jobs:Row[];externalJobId?:string;externalSelection?:string[];onJobStart?:(id:string,selection:string[])=>void}){
 const queryClient=useQueryClient();
 const[token,setToken]=useState('');
 const[notifications,setNotifications]=usePreference<NotificationPrefs>('teampulse.notifications',defaultNotifications);
 const[permission,setPermission]=useState(typeof Notification==='undefined'?'unsupported':Notification.permission);
 const[selected,setSelected]=useState<number[]>([]);
 const[message,setMessage]=useState('');
 const[repoQuery,setRepoQuery]=useState('');
 const[activeJobId,setActiveJobId]=useState('');
 const[syncSelection,setSyncSelection]=useState<string[]>([]);
 const initializedSelection=useRef(false);

 const repositories=useQuery({
  queryKey:queryKeys.repositories,
  queryFn:()=>apiList<Row>('/repositories'),
  enabled:!!auth?.authenticated,
 });

 const activeJob=useQuery({
  queryKey:queryKeys.job(activeJobId),
  queryFn:()=>api<SyncJob>(`/sync-jobs/${activeJobId}`),
  enabled:!!activeJobId,
  refetchInterval:query=>{
   const status=(query.state.data as SyncJob|undefined)?.status;
   return isTerminalJob(status)?false:SYNC_POLL_MS;
  },
 });

 useEffect(()=>{
  if(initializedSelection.current||!repositories.data?.length)return;
  setSelected(persistedRepositorySelection(repositories.data));
  initializedSelection.current=true;
 },[repositories.data]);

 const connectToken=useMutation({
  mutationFn:()=>api('/github/auth/token',{method:'POST',json:{token}}),
  onSuccess:()=>{setToken('');setMessage('');queryClient.invalidateQueries({queryKey:queryKeys.dashboard});queryClient.invalidateQueries({queryKey:queryKeys.repositories});},
  onError:error=>setMessage(explainError(error)),
 });

 const syncRepositories=useMutation({
  mutationFn:async()=>{
   await api('/repositories/selection',{method:'PATCH',json:{repository_ids:selected}});
   return api<{job_id:string}>('/sync-jobs',{method:'POST',json:{repository_ids:selected}});
  },
  onSuccess:result=>{const names=repositories.data?.filter(repo=>selected.includes(Number(repo.id))).map(repo=>String(repo.full_name))||[];setActiveJobId(result.job_id);setSyncSelection(names);onJobStart?.(result.job_id,names);setMessage(`Sync started for ${selected.length} repositories`);queryClient.invalidateQueries({queryKey:queryKeys.dashboard});},
  onError:error=>setMessage(explainError(error)),
 });

 useEffect(()=>{
  const status=activeJob.data?.status;
 if(isTerminalJob(status)){
   queryClient.invalidateQueries({queryKey:queryKeys.dashboard});
   setMessage(status==='completed'?'Sync completed. Dashboard data has been refreshed.':status==='partial'?`Sync completed with errors. ${activeJob.data?.error||activeJob.data?.message||''}`:`Sync failed. ${activeJob.data?.error||activeJob.data?.message||'Check repository access and retry.'}`);
  }
 },[activeJob.data?.status,queryClient]);

 useEffect(()=>{
  if(!activeJob.data||!isTerminalJob(activeJob.data.status))return;
  const finishedJobId=activeJob.data.id;
  const timer=window.setTimeout(()=>{
   setActiveJobId(current=>current===finishedJobId?'':current);
   setSyncSelection([]);
  },SYNC_CLOSE_DELAY_MS);
  return()=>window.clearTimeout(timer);
 },[activeJob.data?.id,activeJob.data?.status]);

 useEffect(()=>{
  if(!externalJobId)return;
  setActiveJobId(externalJobId);
  setSyncSelection(externalSelection);
 },[externalJobId,externalSelection]);

 useEffect(()=>{
  if(activeJobId||externalJobId||!jobs?.length)return;
  const running=jobs.find(isRecentActiveJob);
  if(running?.id)setActiveJobId(String(running.id));
 },[activeJobId,externalJobId,jobs]);

 const repos=repositories.data||[];
 const selectedNames=repos.filter(repo=>selected.includes(Number(repo.id))).map(repo=>String(repo.full_name));
 const filtered=repos.filter(repo=>JSON.stringify(repo).toLowerCase().includes(repoQuery.toLowerCase()));
 const busy=repositories.isFetching||connectToken.isPending||syncRepositories.isPending;
 const toggle=(id:number)=>setSelected(current=>current.includes(id)?current.filter(item=>item!==id):current.length>=20?current:[...current,id]);
 const selectVisible=()=>setSelected(current=>Array.from(new Set([...current,...filtered.map(repo=>Number(repo.id))])).slice(0,20));

 return <div className="settings-grid">
  <section className="card">
   <div className="card-header"><h2>GitHub Connection</h2><span>{auth?.authenticated?'Connected':'Not connected'}</span></div>
   <div className="card-body form">
   <div className="settingsicon"><GitPullRequest size={22}/></div>
   <p>{auth?.authenticated?`Connected as ${auth.login} via ${auth.source}. You can now choose repositories to sync.`:'Paste a fine-grained PAT with repository read access. Tokens entered here stay in memory only.'}</p>
   <input type="password" placeholder="github_pat_…" value={token} onChange={event=>setToken(event.target.value)} aria-label="GitHub personal access token"/>
   <button disabled={!token.trim()||connectToken.isPending} onClick={()=>connectToken.mutate()}>{auth?.authenticated?'Update token':'Connect GitHub'}</button>
   {message&&<div className={message.startsWith('Sync started')?'inlinestatus':'inlineerror'}>{message}</div>}
   </div>
  </section>
  <section className="card form">
   <div className="card-header"><h2>Repository Sync</h2><span>{selected.length} selected</span></div>
   <div className="card-body form">
   <div className="repohead"><div><h2>Choose repositories</h2><p>Select repositories to sync from the last 30 days.</p></div><button className="secondary" onClick={()=>repositories.refetch()} disabled={!auth?.authenticated||repositories.isFetching}><RefreshCw size={15} className={repositories.isFetching?'spin':''}/>{repositories.isFetching?'Loading':'Reload'}</button></div>
   <div className="repotools">
    <label className="searchbox"><Search size={16}/><input value={repoQuery} onChange={event=>setRepoQuery(event.target.value)} placeholder="Search repositories"/></label>
    <div><button type="button" className="secondary compactbtn" disabled={!filtered.length} onClick={selectVisible}>Select visible</button><button type="button" className="secondary compactbtn" disabled={!selected.length} onClick={()=>setSelected([])}>Clear</button></div>
   </div>
   <div className="repolist">{filtered.map(repo=><label key={repo.id||repo.full_name}><input type="checkbox" checked={selected.includes(Number(repo.id))} onChange={()=>toggle(Number(repo.id))}/><span><b>{repo.full_name}</b><small>{repo.description||'No description'}{repo.private?' · Private':''}</small></span></label>)}{!repositories.isFetching&&!filtered.length&&<Empty title={repos.length?'No matching repositories':'No repositories loaded'} text={repos.length?'Try a different repository search.':auth?.authenticated?'Reload repositories. If this stays empty, confirm your token has repository read access.':'Connect GitHub with a fine-grained PAT first.'} compact/>}</div>
   <button disabled={!selected.length||busy} onClick={()=>syncRepositories.mutate()}>Sync {selected.length||''} repositories</button>
   {repositories.error&&<div className="inlineerror">{explainError(repositories.error)}</div>}
   {(activeJobId||syncRepositories.isPending)&&<SyncDetails job={activeJob.data} loading={activeJob.isFetching||syncRepositories.isPending} repositories={syncSelection.length?syncSelection:selectedNames}/>}
   <RecentJobs jobs={jobs}/>
   </div>
  </section>
  <SettingsCard title="Local Server" meta="Healthy" rows={[['Host','127.0.0.1','Local only'],['Port','19421','Default local port'],['Start at login','Launch TeamPulse automatically','Configure in macOS Login Items']]}/>
  <NotificationSettings prefs={notifications} setPrefs={setNotifications} permission={permission} setPermission={setPermission}/>
  <SettingsCard title="Data & Privacy" meta="Local-first" rows={[['Database','~/Library/Application Support/TeamPulse/teampulse.db','SQLite'],['Repository cloning','API-only mode','No local clones'],['Cloud AI analysis','Disabled','No report data uploaded']]}/>
 </div>;
}

export function persistedRepositorySelection(repositories:Row[]){
 return repositories.filter(repo=>repo.selected).map(repo=>Number(repo.id));
}

function NotificationSettings({prefs,setPrefs,permission,setPermission}:{prefs:NotificationPrefs;setPrefs:(value:NotificationPrefs)=>void;permission:string;setPermission:(value:string)=>void}){
 const set=(key:keyof NotificationPrefs,value:boolean|string)=>setPrefs({...prefs,[key]:value});
 return <section className="card"><div className="card-header"><h2>Notifications</h2><span>{permission}</span></div><div className="card-body stack">
  <div className="setting-row"><div className="setting-copy"><strong>Browser permission</strong><span>Required for local desktop notifications while TeamPulse is open.</span></div><button className="secondary compactbtn" onClick={async()=>setPermission(await requestNotificationPermission())}>Enable</button></div>
  <ToggleRow title="High risk detected" detail="Notify when a new high-priority risk appears." checked={prefs.highRisk} onChange={value=>set('highRisk',value)}/>
  <ToggleRow title="Sync failed" detail="Notify when GitHub sync fails and show the error in TeamPulse." checked={prefs.syncFailed} onChange={value=>set('syncFailed',value)}/>
  <ToggleRow title="Sync success" detail="Notify when a sync completes successfully." checked={prefs.syncSuccess} onChange={value=>set('syncSuccess',value)}/>
  <ToggleRow title="Weekly report reminder" detail="Remind you to generate a local weekly report." checked={prefs.weeklyReminder} onChange={value=>set('weeklyReminder',value)}/>
  <div className="notification-schedule"><Dropdown label="Day" value={prefs.weeklyReminderDay} onChange={value=>set('weeklyReminderDay',value)} options={['Monday','Tuesday','Wednesday','Thursday','Friday'].map(day=>({value:day,label:day}))}/><Dropdown label="Time" value={prefs.weeklyReminderTime} onChange={value=>set('weeklyReminderTime',value)} options={['09:00','12:00','16:00','17:00'].map(time=>({value:time,label:time}))}/></div>
 </div></section>;
}

function ToggleRow({title,detail,checked,onChange}:{title:string;detail:string;checked:boolean;onChange:(value:boolean)=>void}){
 return <div className="setting-row"><div className="setting-copy"><strong>{title}</strong><span>{detail}</span></div><button type="button" className={'toggle '+(checked?'':'off')} aria-pressed={checked} onClick={()=>onChange(!checked)}/></div>;
}

function SettingsCard({title,meta,rows}:{title:string;meta:string;rows:[string,string,string][]}){
 return <section className="card"><div className="card-header"><h2>{title}</h2><span>{meta}</span></div><div className="card-body">{rows.map(([label,value,note])=><div className="setting-row" key={label}><div className="setting-copy"><strong>{label}</strong><span>{value}</span></div><span className="pill neutral">{note}</span></div>)}</div></section>;
}

function RecentJobs({jobs}:{jobs:Row[]}){
 if(!jobs?.length)return null;
 return <div className="jobs"><h3>Recent sync jobs</h3>{jobs.slice(0,3).map(job=><div className="job" key={job.id}><b>{job.message||job.type}</b><span>{job.progress}% · {job.status}</span></div>)}</div>;
}

function isActiveJob(status?:string){return status==='pending'||status==='running';}
function isTerminalJob(status?:string){return status==='completed'||status==='partial'||status==='failed';}
function isRecentActiveJob(job:Row){
 if(!isActiveJob(String(job.status)))return false;
 const raw=String(job.started_at||job.created_at||'');
 const timestamp=new Date(raw).getTime();
 return Number.isFinite(timestamp)&&Date.now()-timestamp<2*60*60*1000;
}

function explainError(error:unknown){
 const message=cleanError(error);
 if(/rate limit|API 403/i.test(message))return`${message}. GitHub rate limit may be exhausted; wait for the reset window or use a token with higher limits.`;
 if(/connect GitHub|auth|token|401|rejected/i.test(message))return`${message}. Paste a fine-grained PAT with repository read access.`;
 if(/Sync failed/i.test(message))return`${message}. Check the failed repository name and GitHub token permissions, then retry sync.`;
 return message;
}
