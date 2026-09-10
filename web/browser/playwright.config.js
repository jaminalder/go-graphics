const {defineConfig}=require('@playwright/test');
const baseURL='http://'+(process.env.ART_BROWSER_ADDR||'127.0.0.1:8280');
module.exports=defineConfig({testDir:'.',testMatch:'journey.spec.js',timeout:90000,workers:1,use:{baseURL,browserName:'chromium',trace:'retain-on-failure'},webServer:{command:'bash deploy/scripts/browser-server.sh',cwd:'../..',url:baseURL+'/health/live',timeout:30000,reuseExistingServer:false}});
