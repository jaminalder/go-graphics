CREATE TABLE art_instances (
  id text PRIMARY KEY,
  role text NOT NULL CHECK(role IN ('web','renderer')),
  hostname text NOT NULL,
  name text NOT NULL,
  build text NOT NULL,
  started timestamptz NOT NULL DEFAULT now(),
  touched timestamptz NOT NULL DEFAULT now(),
  stopped timestamptz,
  storage_ok boolean
);
CREATE INDEX ON art_instances(touched);
CREATE INDEX art_render_jobs_recent ON river_job(created_at) WHERE kind='render';
