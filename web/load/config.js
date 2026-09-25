// Pure configuration helpers shared by the three scenarios. No remote JS imports.
export function integer(value, fallback, min, max, name) {
  const n = value === undefined || value === '' ? fallback : Number(value);
  if (!Number.isInteger(n) || n < min || n > max) throw new Error(`${name} must be ${min}..${max}`);
  return n;
}

export function settings(env) {
  const scenario = env.LOAD_SCENARIO || 'browse';
  if (!['browse', 'studio', 'burst'].includes(scenario)) throw new Error('Unknown load scenario');
  const seconds = integer(env.LOAD_SECONDS, 120, 5, 86400, 'duration');
  const users = integer(env.LOAD_USERS, scenario === 'browse' ? 5 : 3, 1, 2000, 'users');
  const timeout = integer(env.LOAD_COMPLETION_TIMEOUT, 120, 5, 600, 'completion timeout');
  const iterations = integer(env.LOAD_ITERATIONS, 0, 0, 10000, 'iterations');
  const artworks = (env.LOAD_ARTWORKS || 'pools,foam,iris').split(',');
  if (!artworks.length || artworks.some(x => !['pools', 'foam', 'iris'].includes(x))) throw new Error('Invalid artworks');
  const imageKeys = (env.LOAD_IMAGE_KEYS || '').split(',').filter(Boolean);
  if (imageKeys.some(x => !/^[a-f0-9]{64}$/.test(x))) throw new Error('Image keys must be 64 lowercase hexadecimal characters');
  const origin = (env.LOAD_TARGET || 'http://127.0.0.1:8280').replace(/\/$/, '');
  if (!/^https?:\/\/[a-zA-Z0-9.:[\]-]+$/.test(origin)) throw new Error('Target must be an HTTP(S) origin without credentials/path/query');
  return {
    scenario, seconds, users, timeout, iterations, artworks, imageKeys, origin,
    poll: integer(env.LOAD_POLL_SECONDS, 3, 2, 30, 'poll seconds'),
    think: integer(env.LOAD_THINK_SECONDS, 2, 0, 60, 'think seconds'),
    seed: integer(env.LOAD_SEED, 1, 0, 2147483647, 'seed'),
    downloadEvery: integer(env.LOAD_DOWNLOAD_EVERY, 5, 0, 10000, 'download every'),
    similarEvery: integer(env.LOAD_SIMILAR_EVERY, 2, 0, 10000, 'similar every'),
    favouriteEvery: integer(env.LOAD_FAVOURITE_EVERY, 3, 0, 10000, 'favourite every'),
  };
}

export function execution(config) {
  const exec = config.scenario === 'browse' ? 'browse' : 'studio';
  // An iteration can include initial preview, download and similarity. Allow it
  // to finish rather than silently abandon accepted jobs when duration elapses.
  const grace = config.scenario === 'browse' ? '30s' : `${3 * config.timeout + 90}s`;
  if (config.iterations) return {
    executor: 'per-vu-iterations', vus: config.users, iterations: config.iterations,
    maxDuration: `${config.seconds}s`, gracefulStop: grace, exec,
  };
  if (config.scenario !== 'burst') return {
    executor: 'constant-vus', vus: config.users, duration: `${config.seconds}s`, gracefulStop: grace, exec,
  };
  const baseline = Math.max(1, Math.floor(config.seconds * 0.15));
  const rise = Math.max(1, Math.floor(config.seconds * 0.05));
  const peak = Math.max(1, Math.floor(config.seconds * 0.45));
  const drain = config.seconds - baseline - rise - peak - 1;
  return {
    executor: 'ramping-vus', startVUs: 1, exec, gracefulRampDown: grace, gracefulStop: grace,
    stages: [
      { duration: `${baseline}s`, target: 1 },
      { duration: `${rise}s`, target: config.users },
      { duration: `${peak}s`, target: config.users },
      { duration: '1s', target: 0 },
      { duration: `${Math.max(1, drain)}s`, target: 0 },
    ],
  };
}

// Deterministic selection of workload shape; the application still chooses fresh render seeds.
export function choice(seed, vu, iteration, salt, length) {
  let n = (seed ^ Math.imul(vu, 2654435761) ^ Math.imul(iteration + 1, 2246822519) ^ salt) >>> 0;
  n = Math.imul(n ^ (n >>> 16), 3266489917) >>> 0;
  return n % length;
}

// Retry-After is a lower bound; extra jitter prevents a synchronized retry wave.
// Return null when there is insufficient deadline remaining to honor that bound.
export function retryDelay(header, attempt, remaining, random) {
  let minimum = Number(header);
  if (!header || !Number.isFinite(minimum) || minimum < 0) minimum = Math.min(5, 0.5 * 2 ** Math.min(attempt, 4));
  if (minimum >= remaining) return null;
  return Math.min(remaining, minimum + 0.25 + random * Math.min(3, Math.max(0.5, minimum * 0.5)));
}

export const operations = ['gallery', 'art_page', 'catalogue_asset', 'existing_bucket_image', 'enter', 'exploration_page', 'sample_page', 'batch_status', 'download_status', 'preview_image', 'download_image', 'favourite', 'unfavourite', 'prepare_download', 'similar'];
export const outcomeNames = ['ok', 'throttled', 'busy', 'conflict', 'unexpected'];
export const statusNames = ['200', '303', '400', '403', '404', '409', '410', '429', '500', '502', '503', '504', '0', 'other'];
export const limitNames = ['none', 'read-ip', 'asset-ip', 'start', 'generate-ip', 'generate-workspace', 'image-readers', 'image-bytes', 'other'];

export function breakdown(data) {
  const result = {};
  for (const operation of operations) {
    const outcomes = {}, statuses = {}, limits = {};
    for (const outcome of outcomeNames) outcomes[outcome] = data.metrics[`response_outcomes{operation:${operation},outcome:${outcome}}`]?.values.count || 0;
    for (const status of statusNames) statuses[status] = data.metrics[`response_statuses{operation:${operation},status_code:${status}}`]?.values.count || 0;
    for (const limit of limitNames) limits[limit] = data.metrics[`response_limits{operation:${operation},limit:${limit}}`]?.values.count || 0;
    if (Object.values(outcomes).some(n => n)) result[operation] = { outcomes, statuses, limits };
  }
  return result;
}

export function summaryText(data, config) {
  const value = (name, field, fallback = 0) => data.metrics[name]?.values[field] ?? fallback;
  const count = name => value(name, 'count');
  const trend = name => `p50=${value(name, 'med').toFixed(0)}ms p95=${value(name, 'p(95)').toFixed(0)}ms max=${value(name, 'max').toFixed(0)}ms`;
  return `\nLOAD RESULT: ${config.scenario} / ${config.users} users / ${config.seconds}s planned\n` +
    `Target: ${config.origin}\n` +
    `HTTP requests: ${count('http_reqs')} | duration ${trend('http_req_duration')}\n` +
    `Actions accepted: ${count('actions_accepted')} | throttled: ${count('responses_throttled')} | queue busy: ${count('responses_busy')} | conflicts: ${count('responses_conflict')}\n` +
    `Unexpected responses: ${(100 * value('unexpected_responses', 'rate')).toFixed(2)}% | protocol errors: ${count('protocol_errors')} | script errors: ${count('script_errors')}\n` +
    `Preview batches ready: ${count('batches_completed')} (${(60 * value('batches_completed', 'rate')).toFixed(1)}/min) | downloads ready: ${count('downloads_completed')} (${(60 * value('downloads_completed', 'rate')).toFixed(1)}/min)\n` +
    `Accepted render outcomes: ready=${count('renders_ready')} failed=${count('renders_failed')} timeout=${count('renders_timeout')} observation_error=${count('renders_observation_error')}\n` +
    `Accepted-to-image completion: ${trend('render_completion_ms')}\n` +
    `  previews: ${trend('render_completion_ms{kind:preview}')}\n  downloads: ${trend('render_completion_ms{kind:download}')}\n` +
    `Interrupted iterations: see k6 execution summary in console.log\n` +
    `Observation retries: image=${count('image_retries')} status=${count('status_retries')} exhausted=${count('observation_retry_exhausted')}\n` +
    `Per-operation/status/limit totals: see operations.json\n` +
    `Deployment limit profile stays active (see config.json). 503 is queue-busy only for the explicit admission-busy message.\n` +
    `This is HTTP load with bounded status polling, not browser/SSE execution.\n`;
}
