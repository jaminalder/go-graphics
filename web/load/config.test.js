import assert from 'node:assert/strict';
import test from 'node:test';
import { settings, execution, choice, summaryText, retryDelay, breakdown } from './config.js';
import { retryRead } from './retry.js';

test('burst drops to zero with bounded stages instead of continuing to admit work', () => {
  const c = settings({ LOAD_SCENARIO: 'burst', LOAD_USERS: '12', LOAD_SECONDS: '60' });
  const scenario = execution(c);
  assert.equal(scenario.executor, 'ramping-vus');
  assert.equal(scenario.stages[1].target, 12);
  assert.equal(scenario.stages.at(-1).target, 0);
  assert.equal(scenario.stages.reduce((s, x) => s + Number.parseInt(x.duration), 0), 60);
  assert.equal(scenario.gracefulRampDown, '450s');
});

test('finite smoke run retains real studio journey and timeout', () => {
  const c = settings({ LOAD_SCENARIO: 'studio', LOAD_USERS: '1', LOAD_ITERATIONS: '1', LOAD_SECONDS: '180' });
  assert.deepEqual(execution(c), { executor: 'per-vu-iterations', vus: 1, iterations: 1, maxDuration: '180s', gracefulStop: '450s', exec: 'studio' });
});

test('rejects invalid targets, unsupported artwork, aggressive polls and invalid image keys', () => {
  for (const env of [{ LOAD_TARGET: 'https://user:pass@example.com' }, { LOAD_ARTWORKS: 'unknown' }, { LOAD_POLL_SECONDS: '0' }, { LOAD_IMAGE_KEYS: '../private' }, { LOAD_USERS: 'NaN' }]) {
    assert.throws(() => settings(env));
  }
});

test('workload selection is reproducible and bounded', () => {
  const sequence = Array.from({ length: 20 }, (_, i) => choice(12, 3, i, 4, 3));
  assert.deepEqual(sequence, Array.from({ length: 20 }, (_, i) => choice(12, 3, i, 4, 3)));
  assert.ok(sequence.every(x => x >= 0 && x < 3));
  assert.ok(new Set(sequence).size > 1);
});

test('larger VU counts reach the executor while retaining a finite upper bound', () => {
  for (const users of [201, 500, 2000]) assert.equal(execution(settings({ LOAD_USERS: String(users) })).vus, users);
  for (const users of ['0', '2001', '1.5']) assert.throws(() => settings({ LOAD_USERS: users }));
});

test('summary distinguishes zero accepted work, rejection and observation failures', () => {
  const text = summaryText({ metrics: { responses_throttled: { values: { count: 10 } }, renders_observation_error: { values: { count: 2 } } } }, settings({}));
  assert.match(text, /Actions accepted: 0 \| throttled: 10/);
  assert.match(text, /observation_error=2/);
});

test('retry jitter honors Retry-After and the total deadline', () => {
  assert.equal(retryDelay('10', 0, 5, 0.5), null);
  assert.ok(retryDelay('10', 0, 20, 0) >= 10);
  assert.notEqual(retryDelay('10', 0, 20, 0.1), retryDelay('10', 0, 20, 0.9));
  assert.ok(retryDelay(undefined, 3, 30, 0.5) < 10);
  assert.ok(retryDelay('10', 0, 10.1, 1) <= 10.1);
});

test('operation breakdown preserves endpoint status and exact limit counts', () => {
  const result = breakdown({ metrics: {
    'response_outcomes{operation:preview_image,outcome:throttled}': { values: { count: 6 } },
    'response_statuses{operation:preview_image,status_code:429}': { values: { count: 6 } },
    'response_limits{operation:preview_image,limit:read-ip}': { values: { count: 6 } },
  } });
  assert.equal(result.preview_image.statuses['429'], 6);
  assert.equal(result.preview_image.limits['read-ip'], 6);
  assert.equal(Object.keys(result).length, 1);
});

test('image/status observation recovers from 429 and 503 without another render POST', () => {
  let clock = 0, retries = 0, exhausted = 0;
  const responses = [{ status: 429 }, { status: 503 }, { status: 200, loadOutcome: 'ok' }];
  const result = retryRead({ now: () => clock, deadline: 10000,
    fetch: () => responses.shift(), transient: r => [429, 503].includes(r.status),
    backoff: () => { clock += 1000; return true; }, retry: () => retries++, exhausted: () => exhausted++,
  });
  assert.equal(result.status, 200); assert.equal(retries, 2); assert.equal(exhausted, 0);
});

test('retry stops at deadline and never retries permanent missing images', () => {
  let clock = 0, calls = 0, exhausted = 0;
  retryRead({ now: () => clock, deadline: 1500, fetch: () => { calls++; return { status: 503 }; }, transient: () => true,
    backoff: () => { clock += 1000; return true; }, retry: () => {}, exhausted: () => exhausted++,
  });
  assert.equal(calls, 2); assert.equal(exhausted, 1);
  const result = retryRead({ now: () => 0, deadline: 1000, fetch: () => ({ status: 410 }), transient: () => false,
    backoff: () => { throw Error('must not retry 410'); }, retry: () => {}, exhausted: () => {},
  });
  assert.equal(result.status, 410);
});
