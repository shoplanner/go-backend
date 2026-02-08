-- name: UpsertCategory :exec
INSERT INTO
    product_categories(id, name)
VALUES
    (?, ?)
ON DUPLICATE KEY UPDATE
    name = VALUES(name);

-- name: UpsertProduct :exec
INSERT INTO
    products(id, created_at, updated_at, name, category_id)
VALUES
    (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    created_at = VALUES(created_at),
    updated_at = VALUES(updated_at),
    name = VALUES(name),
    category_id = VALUES(category_id);

-- name: DeleteFormsByProductID :exec
DELETE FROM
    product_forms
WHERE
    product_id = ?;

-- name: InsertProductForm :exec
INSERT INTO
    product_forms(id, product_id, name)
VALUES
    (?, ?, ?);

-- name: DeleteProductByID :exec
DELETE FROM
    products
WHERE
    id = ?;

-- name: GetProductByID :one
SELECT
    p.id,
    p.created_at,
    p.updated_at,
    p.name,
    p.category_id,
    pc.name AS category_name
FROM
    products p
    LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
WHERE
    p.id = ?
LIMIT
    1;

-- name: GetFormsByProductID :many
SELECT
    id,
    product_id,
    name
FROM
    product_forms
WHERE
    product_id = ?;

-- name: GetProductsByListID :many
SELECT
    p.id,
    p.created_at,
    p.updated_at,
    p.name,
    p.category_id,
    pc.name AS category_name
FROM
    products p
    LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
WHERE
    p.id IN (sqlc.slice('product_ids'));

-- name: GetFormsByProductListID :many
SELECT
    id,
    product_id,
    name
FROM
    product_forms
WHERE
    product_id IN (sqlc.slice('product_ids'));
