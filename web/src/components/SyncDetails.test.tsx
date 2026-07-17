import{renderToStaticMarkup}from'react-dom/server';
import{describe,expect,it}from'vitest';
import{SyncDetails,isTerminalSyncStatus}from'./SyncDetails';

describe('SyncDetails',()=>{
 it('treats partial jobs as terminal failures',()=>{
  const markup=renderToStaticMarkup(<SyncDetails job={{
   id:'job-partial',type:'manual',status:'partial',progress:100,
   message:'Sync completed with errors',created_at:'2026-07-17T00:00:00Z',
   started_at:'2026-07-17T00:00:00Z',ended_at:'2026-07-17T00:01:00Z',
   error:'1 repository failed',
  }} repositories={['acme/pulse']}/>);

  expect(isTerminalSyncStatus('partial')).toBe(true);
  expect(markup).toContain('Sync completed with errors');
  expect(markup).toContain('syncstatus failed');
  expect(markup).toContain('Ended');
  expect(markup).toContain('<details class="syncdetails">');
  expect(markup).not.toContain('class="spin"');
 });
});
