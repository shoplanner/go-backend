-- name: InitProductCategories :exec
CREATE TABLE IF NOT EXISTS product_categories (
    id varchar(255) PRIMARY KEY,
    name varchar(255) NOT NULL UNIQUE
);

-- name: InitProducts :exec
CREATE TABLE IF NOT EXISTS products (
    id varchar(36) PRIMARY KEY,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    name varchar(256) NOT NULL,
    category_id varchar(255)
);

-- name: InitProductForms :exec
CREATE TABLE IF NOT EXISTS product_forms (
    id varchar(36) PRIMARY KEY,
    product_id varchar(36) NOT NULL,
    name varchar(255) NOT NULL
);
