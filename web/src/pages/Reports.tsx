import{useState}from'react';
import{useMutation,useQueryClient}from'@tanstack/react-query';
import{Download,FileText}from'lucide-react';
import type{Row}from'../lib/api';
import{api,queryKeys}from'../lib/api';
import{ago}from'../lib/format';
import{Empty,Stat}from'../components/ui';

export function Reports({rows}:{rows:Row[]}){
 const queryClient=useQueryClient();
 const[report,setReport]=useState<Row|null>(null);
 const generateReport=useMutation({
  mutationFn:()=>api<Row>('/reports',{method:'POST',json:{kind:'weekly',timezone:Intl.DateTimeFormat().resolvedOptions().timeZone||'Asia/Shanghai'}}),
  onSuccess:newReport=>{setReport(newReport);queryClient.invalidateQueries({queryKey:queryKeys.dashboard});},
 });
 const loadReport=useMutation({
  mutationFn:(id:string)=>api<Row>(`/reports/${id}`),
  onSuccess:setReport,
 });

 const content=String(report?.markdown||'Generate or select a report to preview it here.');
 return <>
  <section className="grid metrics">
   <Stat label="Reports Generated" value={rows.length} detail="Stored locally"/>
   <Stat label="Last Generated" value={rows[0]?.created_at?ago(rows[0].created_at):'—'} detail="Latest local report"/>
   <Stat label="Export Format" value="MD" detail="Markdown is default"/>
   <Stat label="Local Only" value="Yes" detail="No report data uploaded"/>
  </section>
  <div className="split">
   <div>
    <section className="card report-options"><div className="card-header"><h2>Weekly Report</h2><span>Fixed template</span></div><div className="card-body"><p className="settings-note">{Intl.DateTimeFormat().resolvedOptions().timeZone||'Asia/Shanghai'} · selected repositories · previous natural week</p></div></section>
    <div className="actions report-actions"><button disabled={generateReport.isPending} onClick={()=>generateReport.mutate()}><FileText size={16}/>{generateReport.isPending?'Generating':'Generate Weekly Report'}</button><button className="secondary" disabled={!report} onClick={()=>navigator.clipboard.writeText(content)}>Copy Markdown</button><button className="secondary" disabled={!report} onClick={()=>report&&downloadReport(String(report.id))}><Download size={16}/>Download .md</button></div>
    <section className="card report"><pre>{content}</pre></section>
   </div>
   <section className="card"><div className="card-header"><h2>Report History</h2><span>{rows.length} local files</span></div>{rows.length?<ul className="member-list">{rows.map(row=><li className="member-item member-select" key={row.id} onClick={()=>loadReport.mutate(String(row.id))}><div className="member-top"><span className="member-title">{row.title}</span><span className="pill success">Ready</span></div><div className="member-desc">{ago(row.created_at)} · Markdown</div></li>)}</ul>:<Empty title="No reports yet" text="Generate a weekly report after syncing repositories." compact/>}</section>
  </div>
 </>;
}

function downloadReport(id:string){
 const anchor=document.createElement('a');
 anchor.href=`/api/reports/${encodeURIComponent(id)}/download`;
 anchor.click();
}
