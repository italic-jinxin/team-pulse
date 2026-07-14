import{useEffect,useState}from'react';

export type NotificationPrefs={
 highRisk:boolean;
 syncFailed:boolean;
 syncSuccess:boolean;
 weeklyReminder:boolean;
 weeklyReminderDay:string;
 weeklyReminderTime:string;
};

export type RiskRules={
 waitingReviewHours:number;
 stalePrDays:number;
 largePrLines:number;
 ciFailureThreshold:number;
};

export type ReportTemplate='executive'|'engineering'|'risk'|'standup';

export const defaultNotifications:NotificationPrefs={
 highRisk:true,
 syncFailed:true,
 syncSuccess:true,
 weeklyReminder:false,
 weeklyReminderDay:'Friday',
 weeklyReminderTime:'16:00',
};

export const defaultRiskRules:RiskRules={
 waitingReviewHours:48,
 stalePrDays:5,
 largePrLines:800,
 ciFailureThreshold:1,
};

export function usePreference<T>(key:string,initial:T){
 const[state,setState]=useState<T>(()=>{
  try{
   const raw=localStorage.getItem(key);
   if(!raw)return initial;
   const parsed=JSON.parse(raw) as T;
   if(initial&&typeof initial==='object'&&!Array.isArray(initial)&&parsed&&typeof parsed==='object'&&!Array.isArray(parsed)){
    return {...initial,...parsed};
   }
   return parsed;
  }catch{return initial;}
 });
 useEffect(()=>{localStorage.setItem(key,JSON.stringify(state));},[key,state]);
 return[state,setState]as const;
}

export function notify(title:string,body:string){
 if(typeof Notification==='undefined')return;
 if(Notification.permission==='granted')new Notification(title,{body});
}

export async function requestNotificationPermission(){
 if(typeof Notification==='undefined')return'unsupported';
 if(Notification.permission==='default')return Notification.requestPermission();
 return Notification.permission;
}
