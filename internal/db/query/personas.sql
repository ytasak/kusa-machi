-- name: InsertPersona :one
-- Idempotent persona generation: a participant may only ever have one persona,
-- so a losing concurrent request gets the already-stored row back.
INSERT INTO personas (
    id, participant_id, age, gender, height_cm, education, occupation, annual_income
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (participant_id)
DO UPDATE SET participant_id = personas.participant_id
RETURNING *;

-- name: GetPersonaByParticipant :one
SELECT * FROM personas WHERE participant_id = $1;

-- name: GetPersonaByID :one
SELECT * FROM personas WHERE id = $1;

-- name: GetActivePersona :one
-- A persona is only a valid interaction target on its own game day.
SELECT p.* FROM personas p
JOIN participants pa ON pa.id = p.participant_id
WHERE p.id = sqlc.arg('persona_id') AND pa.game_date = sqlc.arg('game_date');

-- name: LockPersona :one
-- Serialises like/pass transactions. Callers always lock the personas of a pair
-- in normalised (low, high) order, which both keeps the like budget correct and
-- makes mutual-like detection deterministic without any deadlock risk.
SELECT id FROM personas WHERE id = $1 FOR UPDATE;

-- name: IncrementExposure :exec
-- Exposure counts profiles the user actually evaluated, so this runs only after
-- a successful like or pass, never when a discover batch is returned.
UPDATE personas SET exposure_count = exposure_count + 1 WHERE id = $1;

-- name: UpdatePersonaProfile :one
UPDATE personas
SET name = $2, hobby = $3, bio = $4
WHERE id = $1
RETURNING *;

-- name: SetPersonaPhoto :one
UPDATE personas SET photo_updated_at = NOW() WHERE id = $1 RETURNING photo_updated_at;

-- name: ClearPersonaPhoto :exec
UPDATE personas SET photo_updated_at = NULL WHERE id = $1;
