CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  oidc_subject     text NOT NULL UNIQUE,
  email            text,
  display_name     text,
  experience_level text NOT NULL DEFAULT 'beginner'
                   CHECK (experience_level IN ('beginner','intermediate','advanced')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz
);

CREATE TABLE projects (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  brief       text NOT NULL CHECK (char_length(brief) <= 4000),
  language    text NOT NULL CHECK (language IN ('javascript','typescript','go')),
  status      text NOT NULL DEFAULT 'active'
              CHECK (status IN ('active','archived','pending_deletion')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE INDEX projects_user_idx ON projects (user_id, created_at DESC)
  WHERE deleted_at IS NULL;
CREATE TRIGGER projects_updated BEFORE UPDATE ON projects
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE milestones (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id        uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  position          integer NOT NULL CHECK (position >= 1),
  title             text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 160),
  goal              text NOT NULL DEFAULT '',
  evidence_required text NOT NULL DEFAULT '',
  status            text NOT NULL DEFAULT 'not_started'
                    CHECK (status IN ('not_started','in_progress','under_review','completed')),
  version           integer NOT NULL DEFAULT 1,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, position) DEFERRABLE INITIALLY DEFERRED
);
CREATE TRIGGER milestones_updated BEFORE UPDATE ON milestones
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE submissions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id     uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  milestone_id   uuid REFERENCES milestones(id) ON DELETE SET NULL,
  version        integer NOT NULL CHECK (version >= 1),
  files          jsonb NOT NULL
                 CHECK (jsonb_typeof(files) = 'array'
                        AND jsonb_array_length(files) BETWEEN 1 AND 10),
  total_bytes    integer NOT NULL CHECK (total_bytes BETWEEN 1 AND 102400),
  content_sha256 text NOT NULL,
  learner_note   text NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, version)
);
CREATE INDEX submissions_project_idx ON submissions (project_id, version DESC);