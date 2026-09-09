const {test,expect}=require('@playwright/test');
// Browser contexts share one test-server IP. Honour its real admission limit.
async function enterStudio(page) {
 const start = page.url();
 for (let attempt = 0; attempt < 2; attempt++) {
  const responsePromise = page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/explorations');
  await page.getByRole('button', {name:'Enter the studio'}).click();
  const response = await responsePromise;
  if (response.status() !== 429) { expect(response.status()).toBeLessThan(400); return; }
  const seconds = Number(response.headers()['retry-after']);
  expect(seconds).toBeGreaterThan(0);
  expect(seconds).toBeLessThanOrEqual(10);
  await new Promise(resolve => setTimeout(resolve, seconds * 1000));
  await page.goto(start);
 }
 throw new Error('Studio admission did not recover after Retry-After');
}

test('gallery, four samples, favourite, download and recovery work with strict CSP',async({page,context})=>{
 const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error'&&m.text().includes('Content Security Policy'))errors.push(m.text());});
 await page.goto('/');await expect(page.getByRole('heading',{name:'Find an image of your own.'})).toBeVisible();expect(await context.cookies()).toHaveLength(0);
 await page.screenshot({path:'../../out/browser-gallery-desktop.png',fullPage:true});
 await page.getByRole('link',{name:'Explore Iris'}).click();await page.getByRole('radio',{name:/Fine fibres/}).check();await enterStudio(page);await expect(page.getByRole('heading',{name:'Where shall we begin?'})).toBeVisible();
 await page.getByRole('button',{name:'Generate four',exact:true}).click();await page.locator('.settings summary').click();await page.getByRole('radio',{name:/Earth/}).check();await expect(page.locator('.samples .sample img')).toHaveCount(4,{timeout:60000});await expect(page.getByRole('radio',{name:/Earth/})).toBeChecked();await expect(page.locator('.settings')).toHaveAttribute('open','');await page.getByRole('button',{name:'♡ Keep',exact:true}).first().click();await expect(page.getByRole('heading',{name:/Your favourites/})).toBeVisible();await expect.poll(()=>page.evaluate(()=>localStorage.getItem('art-forms-favourites-v1'))).toContain('recipe_version');
 const recipe=await page.evaluate(()=>localStorage.getItem('art-forms-favourites-v1'));
 await page.locator('.samples').getByRole('link',{name:'View image'}).first().click();await page.getByRole('button',{name:'Prepare download'}).click();await expect(page.getByRole('link',{name:'Download image · 1200 px'})).toBeVisible({timeout:60000});const href=await page.getByRole('link',{name:'Download image · 1200 px'}).getAttribute('href');const response=await page.request.get(href);expect(response.status()).toBe(200);expect(response.headers()['content-type']).toBe('image/png');expect((await response.body()).subarray(1,4).toString()).toBe('PNG');
 await page.getByRole('button',{name:'Prepare image for sharing',exact:true}).click();await expect(page.getByRole('button',{name:'Share image',exact:true})).toBeVisible();await page.getByRole('button',{name:'Share image',exact:true}).click();await expect(page.getByRole('status').last()).toContainText('File sharing is unavailable');
 // Simulate a slow connection without actually opening a native share sheet.
 await page.reload();
 await page.route('**'+href, async route => { await new Promise(resolve => setTimeout(resolve, 6000)); await route.continue(); });
 await page.evaluate(() => {
  Object.defineProperty(navigator, 'canShare', {configurable:true, value:() => true});
  Object.defineProperty(navigator, 'share', {configurable:true, value:async data => { window.shareProof={active:navigator.userActivation.isActive, count:data.files.length, size:data.files[0].size}; }});
 });
 await page.getByRole('button',{name:'Prepare image for sharing',exact:true}).click();
 await expect(page.getByRole('button',{name:'Share image',exact:true})).toBeVisible({timeout:15000});
 await page.getByRole('button',{name:'Share image',exact:true}).click();
 expect(await page.evaluate(() => window.shareProof)).toEqual({active:true,count:1,size:(await response.body()).length});
 await page.unroute('**'+href);
 expect(errors).toEqual([]);
 await page.getByRole('button',{name:'Clear this session'}).click();await page.goto('/recover');await page.locator('#recovery-data').fill(recipe);await page.getByRole('button',{name:'Restore favourites'}).click();await expect(page.getByRole('heading',{name:/Your favourites/})).toBeVisible();await expect(page.locator('.tray input[name="selected"]')).toHaveCount(1);
});
test('mobile and no-JavaScript retain ordinary forms',async({browser})=>{const context=await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:844}});const page=await context.newPage();await page.goto('http://127.0.0.1:8080/');await page.screenshot({path:'../../out/browser-gallery-mobile.png',fullPage:true});await page.getByRole('link',{name:'Explore Pools'}).click();await enterStudio(page);await page.getByRole('button',{name:'Generate four',exact:true}).click();await expect.poll(async()=>{await page.reload();return page.locator('.samples img').count();},{timeout:60000,intervals:[2000]}).toBe(4);await page.getByRole('button',{name:'♡ Keep',exact:true}).first().click();await expect(page.getByRole('heading',{name:/Your favourites/})).toBeVisible();await context.close();});
test('blocked browser storage leaves active exploration usable',async({page})=>{await page.addInitScript(()=>{Object.defineProperty(window,'localStorage',{get(){throw new DOMException('Blocked','SecurityError')}})});await page.goto('/recover');await expect(page.locator('#recovery-status')).toContainText('Browser storage is unavailable');await page.goto('/art/foam');await expect(page.getByRole('button',{name:'Enter the studio'})).toBeVisible();});
test('starting again preserves a saved backup and cross-tab notice',async({page,context})=>{await page.goto('/');const backup=JSON.stringify({version:1,recipes:[require('../catalog/manifest.json')[0].recipe]});await page.evaluate(value=>localStorage.setItem('art-forms-favourites-v1',value),backup);await page.goto('/art/iris');await enterStudio(page);await expect(page.getByRole('button',{name:'Generate four',exact:true})).toBeVisible();expect(await page.evaluate(()=>localStorage.getItem('art-forms-favourites-v1'))).toBe(backup);
 const otherTab=await context.newPage();await otherTab.goto('/recover');
 await otherTab.evaluate(()=>localStorage.removeItem('art-forms-favourites-v1'));
 await expect(page.locator('#notice')).toContainText('Saved favourites changed in another tab');
 await expect(page.getByRole('button',{name:'Generate four',exact:true})).toBeVisible();
 await otherTab.close();
});
