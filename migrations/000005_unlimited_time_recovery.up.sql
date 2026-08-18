-- 時間回復の1日上限を撤廃する。
--
-- もともと時間回復は1日2回で打ち止めにしており、その回数を
-- personas.time_recovery_count で数えていた。天井を回数で作るのをやめ、
-- 「3時間ごと」という間隔と「所持上限 12 を超える分は失われる」の2つだけで
-- 抑える形に変えたため、この列を読む処理がどこにも無くなった。
--
-- 回数を持たないことは、上限に達している間の経過時間を取り置かないことと
-- 対になっている。起点 like_recovery_anchor_at は経過した3時間単位ぶんだけ
-- 必ず進むので、「あと何回もらえるか」という状態はどこにも要らない。
ALTER TABLE personas
    DROP CONSTRAINT IF EXISTS personas_time_recovery_count_range,
    DROP COLUMN IF EXISTS time_recovery_count;
