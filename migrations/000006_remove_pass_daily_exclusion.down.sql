-- 3回上限があった頃の制約を戻す。すでに4回以上 Pass された行は制約を満たさない
-- ので、戻す前に3へ丸める。前日以前の行は日次クリーンアップで消えるため、
-- 丸めの影響を受けるのは当日ぶんだけ。
UPDATE passes SET pass_count = 3 WHERE pass_count > 3;

ALTER TABLE passes
    DROP CONSTRAINT IF EXISTS passes_pass_count_positive;

ALTER TABLE passes
    ADD CONSTRAINT passes_pass_count_check CHECK (pass_count BETWEEN 1 AND 3);
