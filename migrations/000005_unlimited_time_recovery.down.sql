-- 回数の上限があった頃の列を戻す。当日すでに受け取った回数は復元できないので
-- 0 から数え直す。前日以前の persona 行は日次クリーンアップで消えるため、
-- 巻き戻しの影響を受けるのは当日ぶんだけ。
ALTER TABLE personas
    ADD COLUMN time_recovery_count SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE personas
    ADD CONSTRAINT personas_time_recovery_count_range CHECK (time_recovery_count BETWEEN 0 AND 2);
