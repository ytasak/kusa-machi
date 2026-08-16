-- Separate database used by the Go integration tests so they can truncate
-- freely without touching the development data.
CREATE DATABASE kusamachi_test OWNER kusa;
