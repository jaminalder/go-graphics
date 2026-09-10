'use strict';
(() => {
 const notice=message=>{const el=document.querySelector('#notice');if(el)el.textContent=message;};
 const files=new Map();
 let requestedDownload=null;
 let focusedAction=null;
 function enhance(){
  document.querySelectorAll("[data-prepare-download]").forEach(button=>button.textContent="Download image");
  const main=document.querySelector("main");
  if(requestedDownload&&main?.dataset.sample===requestedDownload){const link=main.querySelector("a[data-download]");if(link){requestedDownload=null;link.click();}}
  else if(requestedDownload)requestedDownload=null;
  document.querySelectorAll('[data-back]').forEach(button=>button.hidden=false);
  document.querySelectorAll('[data-share]').forEach(button=>{
   button.hidden=false;
   const url=button.dataset.share;
   if(files.has(url)){button.disabled=false;return;}
   button.disabled=true;
   fetch(url).then(response=>{if(!response.ok)throw Error('unavailable');return response.blob();}).then(blob=>{
    // The native share call must happen on a fresh click, after the file is ready.
    files.set(url,new File([blob],'artwork.png',{type:'image/png'}));
    if(files.size>4)files.delete(files.keys().next().value);
    button.disabled=false;
   }).catch(()=>{button.disabled=false;});
  });
 }
 document.addEventListener('htmx:beforeRequest',event=>{const form=event.detail.elt;if(form?.matches('form[action$="/download"]'))requestedDownload=form.querySelector('[name=sample]').value;if(document.hidden&&event.detail.elt?.id==='main')event.preventDefault();});
 document.addEventListener('htmx:beforeSwap',event=>{if([400,403,409,410,413,429,503].includes(event.detail.xhr.status)){event.detail.shouldSwap=true;event.detail.isError=false;event.detail.target=document.body;}});
 document.addEventListener('htmx:sendError',()=>notice('Connection interrupted. Please reload the page to continue.'));
 document.addEventListener('htmx:beforeSwap',event=>{
  if(!event.detail.xhr.responseURL.includes('/fragments/'))return;
  const active=document.activeElement;
  focusedAction=active?.closest('main')?active.id:null;
 });
 document.addEventListener('htmx:afterSwap',()=>{
  if(focusedAction){document.getElementById(focusedAction)?.focus({preventScroll:true});focusedAction=null;}
 });
 document.addEventListener('htmx:afterSettle',enhance);
 document.addEventListener('click',async event=>{
  const button=event.target.closest('button');if(!button)return;
  if(button.hasAttribute('data-back')){history.back();return;}
  if(!button.dataset.share)return;
  const file=files.get(button.dataset.share);
  if(!file||!navigator.canShare||!navigator.canShare({files:[file]})){notice('Download the image to share it from your photos or files.');return;}
  try{await navigator.share({files:[file],title:'My artwork'});}catch(error){if(error.name!=='AbortError')notice('Sharing did not open. Download the image to share it.');}
 });
 enhance();
})();
