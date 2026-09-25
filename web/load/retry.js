// No k6 imports: deterministic orchestration tested with a fake clock and reads.
export function retryRead({ now, deadline, fetch, transient, backoff, retry, exhausted }) {
  let response;
  for (let attempt = 0; now() < deadline; attempt++) {
    response = fetch(deadline);
    if (response.loadOutcome === 'ok' || !transient(response)) return response;
    if (!backoff(response, (deadline - now()) / 1000, attempt)) break;
    if (now() >= deadline) break;
    retry();
  }
  exhausted();
  return response;
}
