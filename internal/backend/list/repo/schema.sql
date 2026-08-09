-- Reproduces GORM's AutoMigrate output column for column so that CREATE TABLE IF NOT EXISTS is
-- a no-op against deployed files; see internal/backend/functest/testdata/schema.sql.
--
-- Two oddities are deliberate. `title` is nullable because the model tag reads
-- `gorm:"notNull,size:255"` with a comma instead of a semicolon, so AutoMigrate parsed neither
-- half of it (TestNullableColumnsAreNullable pins that). And the ordering column is called
-- "index", a reserved word, which is why it is quoted everywhere it appears.
-- name: InitProductLists :exec
CREATE TABLE IF NOT EXISTS product_lists (
    id text NOT NULL,
    status integer NOT NULL,
    updated_at datetime NOT NULL,
    created_at datetime NOT NULL,
    title text,
    PRIMARY KEY (id)
);

-- name: InitProductListMembers :exec
CREATE TABLE IF NOT EXISTS product_list_members (
    id text NOT NULL,
    user_id text NOT NULL,
    list_id text NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    member_type integer NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT fk_product_list_members_user FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_product_lists_members FOREIGN KEY (list_id) REFERENCES product_lists(id) ON DELETE CASCADE
);

-- name: InitProductListStates :exec
CREATE TABLE IF NOT EXISTS product_list_states (
    id text NOT NULL,
    product_id text NOT NULL,
    list_id text NOT NULL,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    "index" integer NOT NULL,
    count integer,
    form_idx integer,
    status integer NOT NULL,
    replacement_count integer,
    replacement_form_idx integer,
    replacement_product_id text,
    PRIMARY KEY (id),
    CONSTRAINT fk_product_list_states_product FOREIGN KEY (product_id) REFERENCES products(id),
    CONSTRAINT fk_product_list_states_replacement_product FOREIGN KEY (replacement_product_id) REFERENCES products(id),
    CONSTRAINT fk_product_lists_states FOREIGN KEY (list_id) REFERENCES product_lists(id)
);

-- Declared here so this file stays the whole description of the schema, but sqlc generates
-- nothing for CREATE INDEX, so the statement is also spelled out as createIndexQuery in
-- sqlite.go and executed from there. Keep the two in step.
CREATE UNIQUE INDEX IF NOT EXISTS idx_list_user ON product_list_members(user_id, list_id);
