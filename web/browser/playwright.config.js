const {defineConfig}=require('@playwright/test');
const baseURL='http://'+(process.env.ART_BROWSER_ADDR||'127.0.0.1:8280');
module.exports=defineConfig({testDir:'.',testMatch:'journey.spec.js',globalTeardown:require.resolve('./teardown'),timeout:90000,workers:1,use:{baseURL,browserName:'chromium',trace:'retain-on-failure'},webServer:{command:'bash deploy/scripts/browser-server.sh',cwd:'../..',url:baseURL+'/',timeout:240000,reuseExistingServer:false,gracefulShutdown:{signal:'SIGTERM',timeout:30000}}});
