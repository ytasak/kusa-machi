-- name: UpsertPass :one
-- 1回目は INSERT、2回目以降は加算。DO UPDATE に条件を付けているので4回目は
-- 何も更新されず行も返らない。呼び出し側はそれを PassLimitReached として扱い、
-- 黙って上限で止めることはしない。
INSERT INTO passes (id, from_persona_id, to_persona_id)
VALUES (sqlc.arg('id'), sqlc.arg('from_persona_id'), sqlc.arg('to_persona_id'))
ON CONFLICT (from_persona_id, to_persona_id) DO UPDATE
SET pass_count = passes.pass_count + 1,
    last_passed_at = NOW()
WHERE passes.pass_count < 3
RETURNING pass_count;

-- name: GetPassCount :one
SELECT pass_count FROM passes
WHERE from_persona_id = $1 AND to_persona_id = $2;
