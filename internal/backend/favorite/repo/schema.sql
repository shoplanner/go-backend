-- Reproduces GORM's AutoMigrate output column for column so that CREATE TABLE IF NOT EXISTS is
-- a no-op against deployed files; see internal/backend/functest/testdata/schema.sql.
-- name: InitFavoriteLists :exec
CREATE TABLE IF NOT EXISTS favorite_lists (
    id text NOT NULL,
    list_type integer NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    PRIMARY KEY (id)
);

-- name: InitFavoriteMembers :exec
CREATE TABLE IF NOT EXISTS favorite_members (
    id text NOT NULL,
    user_id text NOT NULL,
    favorite_list_id text NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    member_type integer NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT fk_favorite_lists_members FOREIGN KEY (favorite_list_id) REFERENCES favorite_lists(id),
    CONSTRAINT fk_favorite_members_user FOREIGN KEY (user_id) REFERENCES users(id)
);

-- name: InitFavoriteProducts :exec
CREATE TABLE IF NOT EXISTS favorite_products (
    id text NOT NULL,
    product_id text NOT NULL,
    favorite_list_id text NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT fk_favorite_lists_products FOREIGN KEY (favorite_list_id) REFERENCES favorite_lists(id),
    CONSTRAINT fk_favorite_products_product FOREIGN KEY (product_id) REFERENCES products(id)
);

-- The unique index is load-bearing, not decoration: the write path relies on it for its
-- ON CONFLICT clause, which is what keeps a product from being favourited twice.
--
-- It is declared here so this file stays the whole description of the schema, but sqlc silently
-- generates nothing for CREATE INDEX, so the statement is also spelled out as createIndexQuery
-- in sqlite.go and executed from there. Keep the two in step.
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_product ON favorite_products(product_id, favorite_list_id);
