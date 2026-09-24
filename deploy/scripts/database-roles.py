#!/usr/bin/env python3
"""Apply least-privilege runtime roles through stdin; credentials never appear in argv."""
import argparse
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument("directory", type=Path)
p.add_argument("compose", nargs=argparse.REMAINDER)
a = p.parse_args()
sql = ["REVOKE CREATE ON SCHEMA public FROM PUBLIC;"]
for role in ("web", "renderer"):
    password = (a.directory / f"{role}-password").read_text().strip()
    if not password or any(c not in "0123456789abcdef" for c in password):
        raise SystemExit("invalid generated password")
    name = f"art_{role}"
    sql.append(f"DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='{name}') THEN CREATE ROLE {name} LOGIN; END IF; END $$;")
    sql.append(f"ALTER ROLE {name} PASSWORD '{password}'; GRANT CONNECT ON DATABASE art TO {name}; GRANT USAGE ON SCHEMA public TO {name}; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO {name}; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO {name};")
    sql.append(f"REVOKE ALL ON art_schema,river_migration FROM {name}; GRANT SELECT ON art_schema,river_migration TO {name};")
subprocess.run(a.compose + ["exec", "-T", "postgres", "psql", "-U", "art_owner", "-d", "art", "-v", "ON_ERROR_STOP=1"], input="\n".join(sql), text=True, check=True, stdout=subprocess.DEVNULL)
