-- name:  create :one
CREATE TABLE users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login text NOT NULL UNIQUE,
    hash text NOT NULL
);
