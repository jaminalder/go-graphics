CREATE TABLE IF NOT EXISTS art_schema (version integer PRIMARY KEY, checksum text NOT NULL);
CREATE TABLE art_control (
  singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
  enabled boolean NOT NULL DEFAULT false,
  build text NOT NULL DEFAULT '', epoch bigint NOT NULL DEFAULT 0,
  bucket_bound boolean NOT NULL DEFAULT false,
  bucket_location text NOT NULL DEFAULT '',
  object_cursor text NOT NULL DEFAULT ''
);
INSERT INTO art_control(singleton) VALUES(true);
CREATE TABLE art_workspaces (
  id text PRIMARY KEY CHECK(length(id)=64), csrf text NOT NULL,
  touched timestamptz NOT NULL DEFAULT now(), expires timestamptz NOT NULL,
  data bytea NOT NULL DEFAULT ''::bytea,
  CHECK(octet_length(data)<=2097152)
);
CREATE INDEX ON art_workspaces(expires);
CREATE TABLE art_requests (
  id text PRIMARY KEY CHECK(length(id)=64), recipe bytea NOT NULL,
  tier text NOT NULL CHECK(tier IN ('preview','download')), build text NOT NULL,
  job_id bigint, generation text NOT NULL, epoch bigint NOT NULL,
  outcome text NOT NULL DEFAULT 'queued', created timestamptz NOT NULL DEFAULT now(),
  first_started timestamptz,
  CHECK(octet_length(recipe)<=16384)
);
CREATE TABLE art_interests (
  workspace text REFERENCES art_workspaces(id) ON DELETE CASCADE,
  request text REFERENCES art_requests(id) ON DELETE CASCADE,
  PRIMARY KEY(workspace,request)
);
CREATE INDEX ON art_interests(request);
CREATE TABLE art_artifacts (
  request text PRIMARY KEY REFERENCES art_requests(id), object_key text UNIQUE NOT NULL,
  digest text NOT NULL, size bigint NOT NULL CHECK(size BETWEEN 1 AND 16777216),
  created timestamptz NOT NULL DEFAULT now(), expires timestamptz NOT NULL
);
CREATE INDEX ON art_artifacts(expires);
CREATE TABLE art_pins (
  workspace text REFERENCES art_workspaces(id) ON DELETE CASCADE,
  sample text NOT NULL, request text NOT NULL REFERENCES art_requests(id),
  PRIMARY KEY(workspace,sample)
);
CREATE INDEX ON art_pins(request);
CREATE TABLE art_uploads (
  object_key text PRIMARY KEY, request text NOT NULL, generation text NOT NULL,
  size bigint NOT NULL CHECK(size BETWEEN 1 AND 16777216),
  state text NOT NULL CHECK(state IN ('pending','published','deleting')),
  created timestamptz NOT NULL DEFAULT now(), delete_after timestamptz,
  deleted_at timestamptz
);
CREATE INDEX ON art_uploads(state,delete_after);
CREATE TABLE art_rate_buckets (id text PRIMARY KEY, tokens double precision NOT NULL, touched timestamptz NOT NULL);
CREATE TABLE art_renderers (id text PRIMARY KEY, build text NOT NULL, touched timestamptz NOT NULL, storage_ok boolean NOT NULL);
