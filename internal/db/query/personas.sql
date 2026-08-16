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

-- name: UpdatePersonaProfile :one
UPDATE personas
SET name = $2, hobby = $3, bio = $4
WHERE id = $1
RETURNING *;
