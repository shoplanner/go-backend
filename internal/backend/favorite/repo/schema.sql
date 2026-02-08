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

-- name: InitFavoriteLists :exec
CREATE TABLE IF NOT EXISTS favorite_lists (
    id varchar(36) PRIMARY KEY,
    list_type int NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL
);

-- name: InitFavoriteMembers :exec
CREATE TABLE IF NOT EXISTS favorite_members (
    id varchar(36) PRIMARY KEY,
    user_id varchar(36) NOT NULL,
    favorite_list_id varchar(36) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    member_type int NOT NULL
);

-- name: InitFavoriteProducts :exec
CREATE TABLE IF NOT EXISTS favorite_products (
    id varchar(36) PRIMARY KEY,
    product_id varchar(36) NOT NULL,
    favorite_list_id varchar(36) NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    UNIQUE KEY idx_unique_product (product_id, favorite_list_id)
);
