export type Row=Record<string,any>;

export type DashboardData={
 status:Row;
 repositories:Row[];
 activity:Row[];
 prs:Row[];
 members:Row[];
 risks:Row[];
 reports:Row[];
 jobs:Row[];
 auth:Row;
};

export type SyncJob={
 id:string;
 type:string;
 status:'pending'|'running'|'completed'|'failed'|string;
 progress:number;
 message:string;
 created_at:string;
 started_at?:string;
 ended_at?:string;
 error?:string;
};

type ApiInit=RequestInit&{json?:unknown};

export async function api<T=unknown>(path:string,init:ApiInit={}):Promise<T>{
 const{json,...request}=init;
 const response=await fetch('/api'+path,{
  ...request,
  headers:{'Content-Type':'application/json',...(request.headers||{})},
  body:json===undefined?request.body:JSON.stringify(json),
 });

 if(!response.ok){
  const payload=await response.json().catch(()=>({}));
  throw new Error(payload.error||response.statusText);
 }

 if(response.status===204)return null as T;
 return response.json() as Promise<T>;
}

export async function fetchDashboard():Promise<DashboardData>{
 const[status,repositories,activity,prs,members,risks,reports,jobs,auth]=await Promise.all([
  api<Row>('/app/status'),
  api<Row[]>('/repositories'),
  api<Row[]>('/activity'),
  api<Row[]>('/pull-requests'),
  api<Row[]>('/members'),
  api<Row[]>('/risks'),
  api<Row[]>('/reports'),
  api<Row[]>('/jobs'),
  api<Row>('/github/auth/status'),
 ]);

 return{status,repositories,activity,prs,members,risks,reports,jobs,auth};
}

export const queryKeys={
 dashboard:['dashboard']as const,
 repositories:['repositories']as const,
 job:(id:string)=>['job',id]as const,
 report:(id:string)=>['report',id]as const,
};

export function cleanError(error:unknown){
 return String(error instanceof Error?error.message:error).replace(/^Error:\s*/,'');
}
