-- name: InitUsers :exec
CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login text NOT NULL UNIQUE,
    hash text NOT NULL
);
