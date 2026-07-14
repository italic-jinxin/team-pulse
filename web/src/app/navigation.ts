import{Activity,AlertTriangle,FileText,Gauge,GitPullRequest,Settings as SettingsIcon,Users,FolderGit2}from'lucide-react';

export const pages=['Overview','Activity','Pull Requests','Team','Repositories','Risks','Reports','Settings']as const;
export type PageName=typeof pages[number];

export const nav= [
 ['Overview',Gauge],
 ['Activity',Activity],
 ['Pull Requests',GitPullRequest],
 ['Team',Users],
 ['Repositories',FolderGit2],
 ['Risks',AlertTriangle],
 ['Reports',FileText],
 ['Settings',SettingsIcon],
]as const;

export function subtitle(page:PageName){
 return {
  Overview:'A focused read on engineering flow, attention, and weekly output',
  Activity:'Recent work across synced repositories',
  'Pull Requests':'Review flow, CI health, and merge readiness',
  Team:'Contribution signals without turning people into a leaderboard',
  Repositories:'Monitor repository activity, CI health, sync state, and open work',
  Risks:'Stale reviews, failing checks, and work that needs a human nudge',
  Reports:'Generate and copy weekly summaries for stakeholders',
  Settings:'Connect GitHub, pick repositories, and monitor sync progress',
 }[page];
}
