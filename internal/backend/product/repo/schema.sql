-- The DDL below reproduces, column for column, what GORM's AutoMigrate used to create for
-- product.Product, so CREATE TABLE IF NOT EXISTS is a no-op against every deployed file and
-- no migration is owed. That includes the parts nobody would design on purpose: the surrogate
-- `id` on product_forms, the nullable primary keys, and the missing NOT NULL on category name.
-- internal/backend/functest/testdata/schema.sql is the authority; do not "tidy" this up.
-- name: InitProductCategories :exec
CREATE TABLE IF NOT EXISTS product_categories (
    id text,
    name text,
    PRIMARY KEY (id)
);

-- name: InitProducts :exec
CREATE TABLE IF NOT EXISTS products (
    id text,
    created_at datetime NOT NULL,
    updated_at datetime NOT NULL,
    name text NOT NULL,
    category_id text,
    PRIMARY KEY (id),
    CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES product_categories(id)
);

-- name: InitProductForms :exec
CREATE TABLE IF NOT EXISTS product_forms (
    product_id text,
    id text,
    name text,
    PRIMARY KEY (id),
    CONSTRAINT fk_products_forms FOREIGN KEY (product_id) REFERENCES products(id)
);
