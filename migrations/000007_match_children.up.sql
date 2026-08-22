-- 子ガチャの結果。1 Match につき 0 個か1個。
--
-- 子は通常 Persona とは別概念であり、participant も持たない。市場に参加せず、
-- Discover / Like / Pass / Match / ランキングのいずれにも現れない。Match に
-- ぶら下がる「ガチャ結果」でしかないので、personas に混ぜずに独立した表にする。
--
-- match_id を UNIQUE にすることで「引き直せない」を DB のレベルで保証する。
-- 多重クリックや再送があっても、2件目の INSERT はここで必ず弾かれる。
--
-- 0:00 JST の失効は matches と同じ経路に乗る。participants の日次削除から
-- personas -> matches -> match_children と CASCADE するので、この表のために
-- 削除処理を書き足す必要はない。
--
-- レア度の列は持たない。§7.7 のとおりレア度は表示専用の後付けの解釈であり、
-- 判定を1か所（web/src/lib/gacha.js）にしか置かないと決めているため。
-- 属性はここに固定されるので、何度開き直しても同じレア度になる。
CREATE TABLE match_children (
    id UUID PRIMARY KEY,

    match_id UUID NOT NULL UNIQUE
        REFERENCES matches(id)
        ON DELETE CASCADE,

    gender TEXT NOT NULL,
    height_cm SMALLINT NOT NULL,
    education TEXT NOT NULL,
    occupation TEXT NOT NULL,
    annual_income INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 年齢は持たない。子は「2人から生まれた架空の将来ステータス」であって、
    -- 現在のプロフィールではない。
    CHECK (height_cm BETWEEN 140 AND 200),
    CHECK (annual_income >= 0)
);
