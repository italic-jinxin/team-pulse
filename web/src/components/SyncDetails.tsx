import{CheckCircle2,CircleAlert,Clock3,Loader2}from'lucide-react';
import type{SyncJob}from'../lib/api';
import{ago}from'../lib/format';

export function SyncDetails({job,loading=false,repositories=[]}:{job?:SyncJob;loading?:boolean;repositories?:string[]}){
 const status=job?.status||'starting';
 const progress=job?displayProgress(job):0;
 const terminal=status==='completed'||status==='failed';
 const info=parseSyncInfo(job?.message||'',repositories,progress);
 const progressLabel=[info.stage,info.currentRepo,info.repoProgress!=='—'?info.repoProgress:''].filter(Boolean).join(' · ');
 return <section className="syncpanel" aria-live="polite">
  <div className="syncsummary">
   <div className={'syncstatus '+statusTone(status)}>{statusIcon(status,loading)}</div>
   <div>
    <h3>{title(status)}</h3>
    <p>{job?.message||'Queueing sync job…'}</p>
   </div>
   <strong>{progress}%</strong>
  </div>
  <div className="progressbar sync-progressbar" aria-label={`Sync progress: ${progressLabel}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress} role="progressbar"><span style={{width:`${progress}%`}}/><em>{progressLabel}</em></div>
  <div className="sync-detail-grid">
   <Detail label="Current stage" value={info.stage}/>
   <Detail label="Current repository" value={info.currentRepo||'Detecting…'}/>
   <Detail label="Repository progress" value={info.repoProgress}/>
   <Detail label="Elapsed" value={job?.started_at?elapsed(job.started_at):'Waiting'}/>
  </div>
  <div className="syncmeta">
   <span>Job {job?.id||'pending'}</span>
   <span>{job?.started_at?`Started ${ago(job.started_at)}`:'Waiting to start'}</span>
   {terminal&&job?.ended_at?<span>Ended {ago(job.ended_at)}</span>:null}
  </div>
  {repositories.length>0&&<details className="syncdetails" open={!terminal}>
   <summary>{repositories.length} selected repositories</summary>
   <ol>{repositories.map((repo,index)=><li className={repo===info.currentRepo?'active':''} key={repo}><span>{index+1}</span>{repo}</li>)}</ol>
  </details>}
  {job?.error&&<div className="inlineerror">{syncFailureDetail(job.error)}</div>}
 </section>;
}

function Detail({label,value}:{label:string;value:string}){return <div className="sync-detail"><span>{label}</span><strong>{value}</strong></div>;}

export function displayProgress(job:SyncJob){
 const raw=Math.max(0,Math.min(100,Number(job.progress||0)));
 if(raw>0||job.status==='completed'||job.status==='failed')return raw;
 if(job.status!=='running'&&job.status!=='pending')return raw;
 const started=job.started_at?new Date(job.started_at).getTime():Date.now();
 const elapsedSeconds=Math.max(0,(Date.now()-started)/1000);
 return Math.min(90,Math.max(3,Math.round(elapsedSeconds/3)));
}

function parseSyncInfo(message:string,repositories:string[],progress:number){
 const currentRepo=extractRepo(message,repositories);
 const index=currentRepo?repositories.indexOf(currentRepo):-1;
 return {
  stage:stageFromMessage(message),
  currentRepo,
  repoProgress:index>=0?`${index+1} of ${repositories.length}`:repositories.length?`1 of ${repositories.length}`:'—',
  progress,
 };
}

function extractRepo(message:string,repositories:string[]){
 return repositories.find(repo=>message.includes(repo))||message.match(/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+/)?.[0]||'';
}

function stageFromMessage(message:string){
 if(/metadata/i.test(message))return'Reading repository metadata';
 if(/commit/i.test(message))return'Fetching commits';
 if(/pull request|PR #/i.test(message))return'Fetching pull requests and PR details';
 if(/risk/i.test(message))return'Scanning risk signals';
 if(/complete/i.test(message))return'Completed';
 if(/fail/i.test(message))return'Failed';
 if(/connect/i.test(message))return'Connecting to GitHub';
 if(/syncing|starting/i.test(message))return'Syncing repository';
 return'Preparing sync';
}

function elapsed(value:string){
 const started=new Date(value).getTime();
 if(!Number.isFinite(started))return'—';
 const seconds=Math.max(0,Math.floor((Date.now()-started)/1000));
 if(seconds<60)return`${seconds}s`;
 const minutes=Math.floor(seconds/60);
 return`${minutes}m ${seconds%60}s`;
}

function title(status:string){
 if(status==='completed')return'Sync complete';
 if(status==='failed')return'Sync failed';
 return'GitHub sync in progress';
}

function statusTone(status:string){
 if(status==='completed')return'complete';
 if(status==='failed')return'failed';
 return'active';
}

function statusIcon(status:string,loading:boolean){
 if(status==='completed')return <CheckCircle2 size={18}/>;
 if(status==='failed')return <CircleAlert size={18}/>;
 if(loading||status==='running')return <Loader2 size={18} className="spin"/>;
 return <Clock3 size={18}/>;
}

function syncFailureDetail(error:string){
 const repo=error.match(/^([^:]+\/[^:]+):/)?.[1];
 if(repo)return`Sync failed in ${repo}. ${error}. Check repository access, token scopes, or GitHub API rate limits, then retry.`;
 return`${error}. Check the failed repository, token permissions, or GitHub API rate limits, then retry.`;
}
