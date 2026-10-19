CREATE FUNCTION forbid_submission_update() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'submissions are immutable: create a new version instead';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER submissions_no_update
  BEFORE UPDATE ON submissions
  FOR EACH ROW EXECUTE FUNCTION forbid_submission_update();