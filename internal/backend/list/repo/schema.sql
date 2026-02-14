CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login varchar(36) NOT NULL UNIQUE,
    hash text NOT NULL
);

CREATE TABLE IF NOT EXISTS product_categories (
    id varchar(255) PRIMARY KEY,
    name varchar(255) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS products (
    id varchar(36) PRIMARY KEY,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    name varchar(256) NOT NULL,
    category_id varchar(255)
);

CREATE TABLE IF NOT EXISTS product_forms (
    id varchar(36) PRIMARY KEY,
    product_id varchar(36) NOT NULL,
    name varchar(255) NOT NULL
);

-- name: InitProductLists :exec
CREATE TABLE IF NOT EXISTS product_lists (
    id varchar(36) PRIMARY KEY,
    status int NOT NULL,
    updated_at datetime NOT NULL,
    created_at datetime NOT NULL,
    title varchar(255) NOT NULL
);

-- name: InitProductListMembers :exec
CREATE TABLE IF NOT EXISTS product_list_members (
    id varchar(36) PRIMARY KEY,
    user_id varchar(36) NOT NULL,
    list_id varchar(36) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    member_type int NOT NULL,
    UNIQUE KEY idx_list_user (user_id, list_id)
);

-- name: InitProductListStates :exec
CREATE TABLE IF NOT EXISTS product_list_states (
    id varchar(36) PRIMARY KEY,
    product_id varchar(36) NOT NULL,
    list_id varchar(36) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    `index` int NOT NULL,
    count int,
    form_idx int,
    status int NOT NULL,
    replacement_count int,
    replacement_form_idx int,
    replacement_product_id varchar(36)
);
