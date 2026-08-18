ALTER TABLE personas
    DROP CONSTRAINT IF EXISTS personas_bonus_likes_range,
    DROP CONSTRAINT IF EXISTS personas_match_reward_count_range;

ALTER TABLE personas
    DROP COLUMN IF EXISTS bonus_likes,
    DROP COLUMN IF EXISTS profile_reward_claimed,
    DROP COLUMN IF EXISTS match_reward_count;
