CREATE OR REPLACE FUNCTION forbid_submission_update() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'submissions are immutable: create a new version instead';
END;
$$ LANGUAGE plpgsql;
