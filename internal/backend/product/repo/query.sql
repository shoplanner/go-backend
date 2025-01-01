-- name: InsertProduct :exec
INSERT INTO
    products (id, name, category_id, created_at, updated_at)
VALUES
    (?, ?, ?, ?, ?);

-- name: InsertCategory :exec
INSERT INTO
    product_categories (id, name)
VALUES
    (?, ?);

-- name: InsertProductForms :copyfrom
INSERT INTO
    product_forms (product_id, name)
VALUES
    (?, ?);

-- name: UpdateProduct :exec
UPDATE
    products
SET
    name = ?,
    updated_at = ?,
    category_id = ?
WHERE
    id = ?;

-- name: UpdateCategory :exec
UPDATE
    product_categories
SET
    name = ?
WHERE
    id = ?;

-- name: UpdateProductForm :exec
UPDATE
    product_forms
SET
    product_id = ?,
    name = ?;

-- name: DeleteProductForm :exec
DELETE FROM
    product_forms
WHERE
    product_id = ?
    AND name = ?;
