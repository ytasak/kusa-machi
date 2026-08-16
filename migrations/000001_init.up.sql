CREATE TABLE participants (
    id UUID PRIMARY KEY,
    cookie_token UUID NOT NULL,
    game_date DATE NOT NULL,
    csrf_token TEXT NOT NULL,
    likes_last_seen_at TIMESTAMPTZ NULL,
    matches_last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (cookie_token, game_date)
);

-- Used by the daily physical deletion job.
CREATE INDEX participants_game_date_idx ON participants (game_date);

CREATE TABLE personas (
    id UUID PRIMARY KEY,
    participant_id UUID NOT NULL UNIQUE
        REFERENCES participants(id)
        ON DELETE CASCADE,

    age SMALLINT NOT NULL,
    gender TEXT NOT NULL,
    height_cm SMALLINT NOT NULL,
    education TEXT NOT NULL,
    occupation TEXT NOT NULL,
    annual_income INTEGER NOT NULL,

    name VARCHAR(20) NULL,
    hobby VARCHAR(30) NULL,
    bio VARCHAR(60) NULL,

    exposure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (age BETWEEN 20 AND 50),
    CHECK (height_cm BETWEEN 140 AND 200),
    CHECK (annual_income >= 0),
    CHECK (exposure_count >= 0)
);

CREATE TABLE likes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id)
);

CREATE INDEX likes_from_persona_created_at_idx
    ON likes (from_persona_id, created_at DESC);
CREATE INDEX likes_to_persona_created_at_idx
    ON likes (to_persona_id, created_at DESC);

CREATE TABLE passes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    pass_count SMALLINT NOT NULL DEFAULT 1,
    last_passed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id),
    CHECK (pass_count BETWEEN 1 AND 3)
);

CREATE TABLE matches (
    id UUID PRIMARY KEY,

    persona_low_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    persona_high_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (persona_low_id, persona_high_id),

    CHECK (persona_low_id <> persona_high_id)
);

CREATE INDEX matches_persona_low_created_at_idx
    ON matches (persona_low_id, created_at DESC);
CREATE INDEX matches_persona_high_created_at_idx
    ON matches (persona_high_id, created_at DESC);
