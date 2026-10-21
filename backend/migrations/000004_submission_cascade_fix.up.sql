-- Submissions stay immutable, with one exception: when a milestone is deleted,
-- the foreign key may set submissions.milestone_id to NULL. Nothing else may change.
CREATE OR REPLACE FUNCTION forbid_submission_update() RETURNS trigger AS $$
BEGIN
  IF OLD.milestone_id IS NOT NULL AND NEW.milestone_id IS NULL
     AND (NEW.id, NEW.project_id, NEW.version, NEW.files, NEW.total_bytes,
          NEW.content_sha256, NEW.learner_note, NEW.created_at)
         IS NOT DISTINCT FROM
         (OLD.id, OLD.project_id, OLD.version, OLD.files, OLD.total_bytes,
          OLD.content_sha256, OLD.learner_note, OLD.created_at)
  THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'submissions are immutable: create a new version instead';
END;
$$ LANGUAGE plpgsql;
