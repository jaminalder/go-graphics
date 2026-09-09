const {defineConfig}=require('@playwright/test');
module.exports=defineConfig({testDir:'.',testMatch:'journey.spec.js',timeout:90000,workers:1,use:{baseURL:'http://127.0.0.1:8080',browserName:'chromium',trace:'retain-on-failure'},webServer:{command:'bash deploy/scripts/browser-server.sh',cwd:'../..',url:'http://127.0.0.1:8080/health/live',timeout:30000,reuseExistingServer:false}});
