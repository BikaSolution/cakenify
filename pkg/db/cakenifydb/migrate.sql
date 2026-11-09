CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    is_admin boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions (user_id);

CREATE TABLE IF NOT EXISTS birthdays (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    person_name text NOT NULL,
    birth_month smallint NOT NULL CHECK (birth_month BETWEEN 1 AND 12),
    birth_day smallint NOT NULL CHECK (birth_day BETWEEN 1 AND 31),
    birth_year smallint,
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_birthdays_user_id ON birthdays (user_id);
CREATE INDEX IF NOT EXISTS idx_birthdays_month_day ON birthdays (birth_month, birth_day);

CREATE TABLE IF NOT EXISTS notification_settings (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    strategy text NOT NULL,
    config jsonb NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    UNIQUE (user_id, strategy)
);

CREATE TABLE IF NOT EXISTS scheduler_state (
    job_name text PRIMARY KEY,
    last_run_date date NOT NULL
);
