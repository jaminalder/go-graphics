const {execFileSync}=require('node:child_process');
const path=require('node:path');
module.exports=async()=>execFileSync('python3',[path.resolve(__dirname,'../../deploy/scripts/cleanup-browser.py')],{stdio:'inherit'});
