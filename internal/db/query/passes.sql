-- name: UpsertPass :one
-- 1回目は INSERT、2回目以降は加算。回数に上限は無い。Pass を何度受けても
-- 相手が候補から消えることはなく、再表示までの間隔はフロントエンドの
-- クールダウンだけが持つ。
INSERT INTO passes (id, from_persona_id, to_persona_id)
VALUES (sqlc.arg('id'), sqlc.arg('from_persona_id'), sqlc.arg('to_persona_id'))
ON CONFLICT (from_persona_id, to_persona_id) DO UPDATE
SET pass_count = passes.pass_count + 1,
    last_passed_at = NOW()
RETURNING pass_count;

-- name: GetPassCount :one
SELECT pass_count FROM passes
WHERE from_persona_id = $1 AND to_persona_id = $2;
