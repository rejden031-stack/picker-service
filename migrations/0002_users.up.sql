CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role         TEXT NOT NULL CHECK (role IN ('worker', 'senior'))
);