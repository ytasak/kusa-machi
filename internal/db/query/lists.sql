-- name: ListReceivedLikes :many
-- 自分に Like を送った Persona を新しい順に返す。時刻も送信者の Like 残数も
-- 一切公開しない。
SELECT p.*
FROM likes l
JOIN personas p ON p.id = l.from_persona_id
WHERE l.to_persona_id = sqlc.arg('persona_id')
ORDER BY l.created_at DESC, l.id DESC;

-- name: ListSentLikes :many
-- その日の Like 配分の履歴。Match した相手も一覧に残し、画面が MATCH バッジを
-- 出せるようフラグを立てる。
SELECT p.*, EXISTS (
    SELECT 1 FROM matches m
    WHERE (m.persona_low_id = sqlc.arg('persona_id') AND m.persona_high_id = p.id)
       OR (m.persona_low_id = p.id AND m.persona_high_id = sqlc.arg('persona_id'))
) AS matched
FROM likes l
JOIN personas p ON p.id = l.to_persona_id
WHERE l.from_persona_id = sqlc.arg('persona_id')
ORDER BY l.created_at DESC, l.id DESC;

-- name: ListMatches :many
-- 各 Match の相手 Persona を新しい順に返す。
--
-- match_id を添えるのは、一覧のカードから Match 詳細を開けるようにするため。
-- child_generated は子ガチャを引いたかどうかで、一覧では「👶 子あり」の
-- 目印と、詳細を開いたときに演出を出すかどうかの判断に使う。子の中身そのものは
-- 一覧には出さない。
SELECT
    m.id AS match_id,
    sqlc.embed(p),
    EXISTS (
        SELECT 1 FROM match_children c WHERE c.match_id = m.id
    ) AS child_generated
FROM matches m
JOIN personas p ON p.id = CASE
    WHEN m.persona_low_id = sqlc.arg('persona_id') THEN m.persona_high_id
    ELSE m.persona_low_id
END
WHERE m.persona_low_id = sqlc.arg('persona_id')
   OR m.persona_high_id = sqlc.arg('persona_id')
ORDER BY m.created_at DESC, m.id DESC;
