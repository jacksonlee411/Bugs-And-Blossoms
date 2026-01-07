DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    CREATE ROLE app_runtime
      LOGIN
      PASSWORD 'app'
      NOSUPERUSER
      NOCREATEDB
      NOCREATEROLE
      NOREPLICATION
      NOBYPASSRLS;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_nobypassrls') THEN
    CREATE ROLE app_nobypassrls
      NOLOGIN
      NOSUPERUSER
      NOCREATEDB
      NOCREATEROLE
      NOREPLICATION
      NOBYPASSRLS;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'superadmin_runtime') THEN
    CREATE ROLE superadmin_runtime
      LOGIN
      PASSWORD 'app'
      NOSUPERUSER
      NOCREATEDB
      NOCREATEROLE
      NOREPLICATION
      NOBYPASSRLS;
  END IF;
END
$$;

GRANT app_nobypassrls TO app_runtime;

ALTER DATABASE bugs_and_blossoms OWNER TO app_runtime;
GRANT ALL PRIVILEGES ON DATABASE bugs_and_blossoms TO app_runtime;
GRANT ALL PRIVILEGES ON DATABASE bugs_and_blossoms TO superadmin_runtime;
