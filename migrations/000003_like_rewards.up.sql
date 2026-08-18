-- Like の回復報酬。当日の状態なので persona に持たせる。
--
-- persona 行はゲーム日ごとに作り直され、前日のものは日次クリーンアップが
-- participant ごと消す。よって 0:00 のリセットはデフォルト値がそのまま担い、
-- リセット処理を書く必要がない。
--
-- Like と Pass のトランザクションは lockPair で persona 行を FOR UPDATE する。
-- 報酬もこの行に置くことで、新しいロックを足さずに同じ直列化に乗る。

-- 予算 10 に上乗せされた回復ぶんの累計。残数は 10 + bonus_likes - 送信済み Like。
-- 所持上限を超える回復は付与の時点で切り捨てるため、失われた分はここに現れない。
ALTER TABLE personas
    ADD COLUMN bonus_likes SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN profile_reward_claimed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN match_reward_count SMALLINT NOT NULL DEFAULT 0;

-- 上限そのものはアプリケーションが強制するが、取りうる値の範囲は DB に書いておく。
-- 1日に得られるのはプロフィール完成の1と Match 2回ぶんの4で、合計5が上限。
ALTER TABLE personas
    ADD CONSTRAINT personas_bonus_likes_range CHECK (bonus_likes BETWEEN 0 AND 5),
    ADD CONSTRAINT personas_match_reward_count_range CHECK (match_reward_count BETWEEN 0 AND 2);
