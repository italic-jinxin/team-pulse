import{useEffect,useId,useRef,useState}from'react';
import{ChevronDown}from'lucide-react';

export type DropdownOption={value:string;label:string};

export function Dropdown({label,value,options,onChange,ariaLabel}:{label?:string;value:string;options:DropdownOption[];onChange:(value:string)=>void;ariaLabel?:string}){
 const[open,setOpen]=useState(false);
 const buttonRef=useRef<HTMLButtonElement|null>(null);
 const menuRef=useRef<HTMLDivElement|null>(null);
 const id=useId();
 const selected=options.find(option=>option.value===value)||options[0];

 useEffect(()=>{
  if(!open)return;
  const onPointerDown=(event:PointerEvent)=>{
   const target=event.target as Node;
   if(buttonRef.current?.contains(target)||menuRef.current?.contains(target))return;
   setOpen(false);
  };
  const onKeyDown=(event:KeyboardEvent)=>{
   if(event.key==='Escape'){setOpen(false);buttonRef.current?.focus();}
  };
  document.addEventListener('pointerdown',onPointerDown);
  document.addEventListener('keydown',onKeyDown);
  return()=>{document.removeEventListener('pointerdown',onPointerDown);document.removeEventListener('keydown',onKeyDown);};
 },[open]);

 const move=(direction:1|-1)=>{
  const index=Math.max(0,options.findIndex(option=>option.value===value));
  const next=options[(index+direction+options.length)%options.length];
  if(next)onChange(next.value);
 };

 return <div className="dropdown-field">
  {label&&<span className="dropdown-label">{label}</span>}
  <div className="dropdown">
   <button ref={buttonRef} type="button" className="dropdown-trigger" aria-label={ariaLabel||label} aria-haspopup="listbox" aria-expanded={open} aria-controls={id} onClick={()=>setOpen(current=>!current)} onKeyDown={event=>{if(event.key==='ArrowDown'){event.preventDefault();if(!open)setOpen(true);else move(1);}if(event.key==='ArrowUp'){event.preventDefault();if(!open)setOpen(true);else move(-1);}}}>
    <span>{selected?.label||value}</span><ChevronDown size={16}/>
   </button>
   {open&&<div ref={menuRef} className="dropdown-menu" id={id} role="listbox" aria-label={ariaLabel||label}>
    {options.map(option=><button key={option.value} type="button" role="option" aria-selected={option.value===value} className={option.value===value?'selected':''} onClick={()=>{onChange(option.value);setOpen(false);buttonRef.current?.focus();}}>{option.label}</button>)}
   </div>}
  </div>
 </div>;
}
