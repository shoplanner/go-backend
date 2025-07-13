-- name: InitUsers :exec
CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login varchar(36) NOT NULL UNIQUE,
    hash text NOT NULL,
    UNIQUE KEY unique_username (login)
);
