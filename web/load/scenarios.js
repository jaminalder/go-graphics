import http from 'k6/http';
import { sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { settings, execution, choice, summaryText, retryDelay, operations, outcomeNames, statusNames, limitNames, breakdown } from './config.js';
import { retryRead } from './retry.js';

const config = settings(__ENV);
const unexpected = new Rate('unexpected_responses');
const protocolErrors = new Counter('protocol_errors');
const throttled = new Counter('responses_throttled');
const busy = new Counter('responses_busy');
const conflicts = new Counter('responses_conflict');
const accepted = new Counter('actions_accepted');
const batches = new Counter('batches_completed');
const downloads = new Counter('downloads_completed');
const ready = new Counter('renders_ready');
const failed = new Counter('renders_failed');
const timedOut = new Counter('renders_timeout');
const observationError = new Counter('renders_observation_error');
const completion = new Trend('render_completion_ms', true);
const completionSuccess = new Rate('render_completion_success');
const outcomes = new Counter('response_outcomes');
const journeys = new Counter('journeys_completed');
const scriptErrors = new Counter('script_errors');
const imageRetries = new Counter('image_retries');
const statusRetries = new Counter('status_retries');
const retryExhausted = new Counter('observation_retry_exhausted');
const statuses = new Counter('response_statuses');
const responseLimits = new Counter('response_limits');
const breakdownThresholds = {};
for (const operation of operations) {
  for (const outcome of outcomeNames) breakdownThresholds[`response_outcomes{operation:${operation},outcome:${outcome}}`] = [];
  for (const status of statusNames) breakdownThresholds[`response_statuses{operation:${operation},status_code:${status}}`] = [];
  for (const limit of limitNames) breakdownThresholds[`response_limits{operation:${operation},limit:${limit}}`] = [];
}

export const options = {
  scenarios: { [config.scenario]: execution(config) },
  hosts: JSON.parse(__ENV.LOAD_HOSTS || '{}'),
  noCookiesReset: true,
  maxRedirects: 0,
  teardownTimeout: '65s',
  userAgent: 'singular-seed-load/k6',
  systemTags: ['status', 'method', 'name', 'scenario', 'expected_response', 'error_code'],
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'max'],
  thresholds: {
    ...breakdownThresholds,
    unexpected_responses: ['rate<0.01'],
    protocol_errors: ['count==0'],
    script_errors: ['count==0'],
    ...(config.scenario === 'browse' ? { journeys_completed: ['count>0'] } : {}),
    ...(config.scenario === 'browse' ? {} : { actions_accepted: ['count>0'], batches_completed: ['count>0'] }),
    ...(config.scenario === 'browse' ? {} : { render_completion_success: ['rate>0.95'] }),
    'render_completion_ms{kind:preview}': [],
    'render_completion_ms{kind:download}': [],
  },
};

function problem(message) {
  protocolErrors.add(1);
  // Never include cookies, CSRF or unique navigation URLs in reports.
  throw new Error(message);
}

function pathURL(path) {
  if (path.startsWith(`${config.origin}/`)) path = path.slice(config.origin.length);
  if (!path.startsWith('/') || path.startsWith('//') || /[\r\n\\]/.test(path)) problem('Response supplied an unsafe/off-origin URL');
  return config.origin + path;
}

function request(method, path, data, name, binary = false, deadline = Infinity) {
  const response = http.request(method, pathURL(path), data, {
    redirects: 0, timeout: `${Math.max(1, Math.min(15000, deadline - Date.now()))}ms`, responseType: binary ? 'binary' : 'text',
    headers: method === 'POST' ? { Origin: config.origin, 'Content-Type': 'application/x-www-form-urlencoded' } : {},
    tags: { name },
    responseCallback: http.expectedStatuses(200, 303, 429),
  });
  let outcome = 'unexpected';
  if ((method === 'GET' && response.status === 200) || (method === 'POST' && response.status === 303)) outcome = 'ok';
  else if (response.status === 429) { outcome = 'throttled'; throttled.add(1); }
  else if (method === 'POST' && response.status === 503 && typeof response.body === 'string' && response.body.includes('generation is busy; try again shortly')) { outcome = 'busy'; busy.add(1); }
  else if (method === 'POST' && response.status === 409) { outcome = 'conflict'; conflicts.add(1); }
  unexpected.add(outcome === 'unexpected', { operation: name });
  outcomes.add(1, { operation: name, outcome });
  statuses.add(1, { operation: name, status_code: statusNames.includes(String(response.status)) ? String(response.status) : 'other' });
  const limit = response.headers['X-Art-Limit'] || 'none';
  responseLimits.add(1, { operation: name, limit: limitNames.includes(limit) ? limit : 'other' });
  return { status: response.status, body: response.body, headers: response.headers, url: response.url,
    html: () => response.html(), loadOutcome: outcome };
}

function backoff(response, remaining = 30, attempt = 0) {
  const jitter = choice(config.seed, __VU, __ITER, 100 + (++retrySequence), 10000) / 10000;
  const delay = retryDelay(response.headers['Retry-After'], attempt, remaining, jitter);
  if (delay === null) return false;
  sleep(delay); return true;
}
let retrySequence = 0;

function transient(response) { return [0, 429, 502, 503, 504].includes(response.status); }

// Retry only idempotent reads. Count every HTTP failure even when a later retry succeeds.
function readWithRetry(path, name, binary, deadline, imageRequest) {
  return retryRead({ now: () => Date.now(), deadline,
    fetch: limit => request('GET', path, null, name, binary, limit), transient, backoff,
    retry: () => (imageRequest ? imageRetries : statusRetries).add(1, { operation: name }),
    exhausted: () => retryExhausted.add(1, { operation: name }),
  });
}

function form(response, suffix) {
  const found = response.html().find(`form[action$="${suffix}"]`).first();
  if (!found.size()) problem(`Missing ${suffix} form`);
  const fields = {};
  const inputs = found.find('input[type="hidden"]');
  for (let i = 0; i < inputs.size(); i++) { const element = inputs.eq(i); fields[element.attr('name')] = element.attr('value') || ''; }
  return { path: found.attr('action'), fields, element: found };
}

function follow(response, name) {
  const location = response.headers.Location;
  if (!location) problem('Successful action has no Location header');
  if (!/^\/(explorations\/|favourites|$)/.test(location)) problem('Unexpected action redirect');
  return request('GET', location, null, name);
}

function image(path, name, expectedSize = 0, deadline = Date.now() + config.timeout * 1000) {
  const response = readWithRetry(path, name, true, deadline, true);
  if (!response || response.loadOutcome !== 'ok') return false;
  const bytes = new Uint8Array(response.body);
  const png = [137, 80, 78, 71, 13, 10, 26, 10];
  const width = bytes.length >= 24 ? new DataView(response.body).getUint32(16) : 0;
  const height = bytes.length >= 24 ? new DataView(response.body).getUint32(20) : 0;
  if (bytes.length < 24 || png.some((v, i) => bytes[i] !== v) || (expectedSize && (width !== expectedSize || height !== expectedSize))) problem('Invalid PNG or unexpected rendition dimensions');
  return true;
}

function waitForImages(response, started, kind, artwork) {
  let current = response;
  const deadline = started + config.timeout * 1000;
  const result = (outcome) => {
    ({ ready, failed, timeout: timedOut, observation_error: observationError })[outcome].add(1, { kind, artwork });
    completionSuccess.add(outcome === 'ready', { kind, artwork });
    if (outcome === 'ready') {
      completion.add(Date.now() - started, { kind, artwork });
      (kind === 'download' ? downloads : batches).add(1, { artwork });
      return current;
    }
    return null;
  };
  while (Date.now() < deadline) {
    if (current.loadOutcome !== 'ok') {
      if (!transient(current)) return result('observation_error');
    } else {
      const doc = current.html();
      const paths = [];
      const elements = doc.find(kind === 'download' ? 'a[data-download]' : '.samples .sample img');
      for (let i = 0; i < elements.size(); i++) paths.push(elements.eq(i).attr(kind === 'download' ? 'href' : 'src'));
      if (paths.length === (kind === 'download' ? 1 : 4)) {
        if (!paths.every(path => image(path, kind === 'download' ? 'download_image' : 'preview_image', kind === 'download' ? 1200 : 600, deadline))) return result('observation_error');
        return result('ready');
      }
      if (!doc.find('main[data-events]').size()) return result('failed');
    }
    const left = (deadline - Date.now()) / 1000;
    if (transient(current)) { if (!backoff(current, left)) return result('observation_error'); statusRetries.add(1); }
    else sleep(Math.min(config.poll, Math.max(0, left)));
    // Fragment reads avoid refreshing session lifetime just to observe background work.
    const path = response.url.slice(config.origin.length).replace(/^\/fragments/, '');
    current = readWithRetry('/fragments' + path, kind === 'download' ? 'download_status' : 'batch_status', false, deadline, false);
    if (!current) return result('observation_error');
  }
  return result('timeout');
}

function submit(current, suffix, operation, renderKind, artwork) {
  const selected = form(current, suffix);
  const started = Date.now();
  const posted = request('POST', selected.path, selected.fields, operation);
  if (posted.loadOutcome !== 'ok') { backoff(posted); return null; }
  accepted.add(1, { operation, artwork });
  const page = follow(posted, renderKind === 'download' ? 'sample_page' : 'exploration_page');
  return renderKind ? waitForImages(page, started, renderKind, artwork) : page;
}

export function setup() {
  const response = http.get(config.origin + '/', { redirects: 0, timeout: '10s', tags: { name: 'preflight' } });
  if (response.status !== 200 || !response.html().find('a[href="/art/iris"]').size()) throw new Error('Target is not the expected studio at its canonical origin; nothing was submitted');
  return { started: Date.now() };
}

// k6 can finish a ramp early when all VUs reach zero. Preserve a bounded quiet tail.
export function teardown(data) {
  if (config.scenario === 'burst' && !config.iterations) sleep(Math.max(0, Math.min(60, (data.started + config.seconds * 1000 - Date.now()) / 1000)));
}

function browseJourney() {
  protocolErrors.add(0);
  const artwork = config.artworks[choice(config.seed, __VU, __ITER, 1, config.artworks.length)];
  for (const path of ['/', '/art/' + artwork]) {
    const page = request('GET', path, null, path === '/' ? 'gallery' : 'art_page');
    if (page.loadOutcome !== 'ok') { backoff(page); return; }
    const assets = [];
    const elements = page.html().find('img[src^="/assets/"]');
    for (let i = 0; i < elements.size(); i++) { const src = elements.eq(i).attr('src'); if (!assets.includes(src)) assets.push(src); }
    for (const src of assets.slice(0, 8)) {
      const response = request('GET', src, null, 'catalogue_asset', true);
      if (response.loadOutcome !== 'ok') { backoff(response); return; }
    }
  }
  if (config.imageKeys.length && !image('/images/' + config.imageKeys[choice(config.seed, __VU, __ITER, 2, config.imageKeys.length)], 'existing_bucket_image')) return;
  journeys.add(1);
  sleep(config.think);
}

function studioJourney() {
  protocolErrors.add(0);
  const artwork = config.artworks[choice(config.seed, __VU, __ITER, 3, config.artworks.length)];
  const step = __ITER + __VU;
  let page = request('GET', '/art/' + artwork, null, 'art_page');
  if (page.loadOutcome !== 'ok') { backoff(page); return; }
  const start = form(page, '/explorations');
  for (const [field, salt] of [['style', 4], ['colour', 5]]) {
    const values = [];
    const inputs = start.element.find(`input[name="${field}"]`);
    for (let i = 0; i < inputs.size(); i++) values.push(inputs.eq(i).attr('value') || '');
    if (!values.length) problem(`Missing ${field} choices`);
    start.fields[field] = values[choice(config.seed, __VU, __ITER, salt, values.length)];
  }
  const began = Date.now();
  const posted = request('POST', start.path, start.fields, 'enter');
  if (posted.loadOutcome !== 'ok') { backoff(posted); return; }
  accepted.add(1, { operation: 'enter', artwork });
  page = waitForImages(follow(posted, 'exploration_page'), began, 'preview', artwork);
  if (!page) return;
  if (config.scenario === 'burst') { sleep(config.think); return; }

  const favourited = config.favouriteEvery && step % config.favouriteEvery === 0;
  if (favourited) {
    const favourite = form(page, '/favourites');
    favourite.fields.on = 'yes';
    const res = request('POST', favourite.path, favourite.fields, 'favourite');
    if (res.loadOutcome !== 'ok') { backoff(res); return; }
    page = follow(res, 'exploration_page');
    if (page.loadOutcome !== 'ok') return;
  }
  const links = [];
  const elements = page.html().find('.sample a[id^="open-"]');
  for (let i = 0; i < elements.size(); i++) links.push(elements.eq(i).attr('href'));
  if (!links.length) problem('Completed batch has no sample links');
  page = request('GET', links[favourited ? 0 : choice(config.seed, __VU, __ITER, 6, links.length)], null, 'sample_page');
  if (page.loadOutcome !== 'ok') { backoff(page); return; }
  if (config.downloadEvery && step % config.downloadEvery === 0) {
    page = submit(page, '/download', 'prepare_download', 'download', artwork);
    if (!page) return;
  }
  // Exercise retention mutations without accumulating 24 favourite pins per VU.
  if (favourited) {
    const favourite = form(page, '/favourites'); favourite.fields.on = 'no';
    const response = request('POST', favourite.path, favourite.fields, 'unfavourite');
    if (response.loadOutcome !== 'ok') { backoff(response); return; }
    page = follow(response, 'sample_page');
    if (page.loadOutcome !== 'ok') return;
  }
  if (config.similarEvery && step % config.similarEvery === 0) {
    page = submit(page, '/similar', 'similar', 'preview', artwork);
    if (!page) return;
  }
  sleep(config.think);
}

// k6 may finish a run despite iteration exceptions; explicitly fail the script-error threshold.
export function browse() { scriptErrors.add(0); try { browseJourney(); } catch (error) { scriptErrors.add(1); throw error; } }
export function studio() { scriptErrors.add(0); try { studioJourney(); } catch (error) { scriptErrors.add(1); throw error; } }

export function handleSummary(data) {
  const text = summaryText(data, config);
  return { stdout: text, '/results/summary.json': JSON.stringify(data, null, 2), '/results/summary.txt': text, '/results/operations.json': JSON.stringify(breakdown(data), null, 2) };
}
