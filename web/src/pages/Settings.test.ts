import{describe,expect,it}from'vitest';
import{persistedRepositorySelection}from'./Settings';

describe('persistedRepositorySelection',()=>{
 it('trusts an intentionally empty server selection',()=>{
  expect(persistedRepositorySelection([
   {id:1,selected:false},
   {id:2,selected:false},
   {id:3,selected:false},
  ])).toEqual([]);
 });

 it('returns only repositories selected by the server',()=>{
  expect(persistedRepositorySelection([
   {id:1,selected:true},
   {id:2,selected:false},
   {id:3,selected:true},
  ])).toEqual([1,3]);
 });
});
