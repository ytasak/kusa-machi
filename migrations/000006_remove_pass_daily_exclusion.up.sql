-- Pass 3回で当日除外にするのをやめる。
--
-- ユーザー数が少ないうちは、探す画面に出せる相手がそもそも多くない。そこへ
-- 「3回 Pass した相手はその日もう出さない」を掛けると、遊んでいるうちに候補が
-- 尽きて画面が空になる。抑制はフロントエンドの5枚クールダウン（§9）だけで足り、
-- 上限は候補の枯渇という形でしか効いていなかった。
--
-- pass_count は残す。除外には使わなくなるが、Pass の回数そのものは
-- 後から候補の並び順などに使える材料なので、記録は続ける。上限が無くなった
-- ぶん、範囲としては下限だけを DB に書いておく。
ALTER TABLE passes
    DROP CONSTRAINT IF EXISTS passes_pass_count_check;

ALTER TABLE passes
    ADD CONSTRAINT passes_pass_count_positive CHECK (pass_count >= 1);
