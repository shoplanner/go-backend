-- name: GetProductByID :one
SELECT
    *
FROM
    products
WHERE
    id = ?
LIMIT
    1;

-- name: GetProductsByIDList :many
SELECT
    *
FROM
    products
WHERE
    id IN (sqlc.slice('ids'));

-- GetAllProducts backs the empty-filter case of GetByListID. GORM's Find(&entities, uuids)
-- treated an empty id slice as "no condition" and returned the whole table; that behaviour is
-- pinned by TestProductIDListWithNoIDs_CurrentBehaviour.
-- name: GetAllProducts :many
SELECT
    *
FROM
    products;

-- name: GetFormsByProductIDList :many
SELECT
    *
FROM
    product_forms
WHERE
    product_id IN (sqlc.slice('product_ids'))
ORDER BY
    rowid;

-- name: GetCategoriesByIDList :many
SELECT
    *
FROM
    product_categories
WHERE
    id IN (sqlc.slice('ids'));

-- GORM's First() ordered by primary key, so a duplicated category name always resolved to the
-- lowest id. The legacy dump contains two rows named '', which is where that matters.
-- name: GetCategoryByName :one
SELECT
    *
FROM
    product_categories
WHERE
    name = ?
ORDER BY
    id
LIMIT
    1;

-- name: InsertCategory :exec
INSERT INTO
    product_categories(id, name)
VALUES
    (?, ?);

-- name: InsertProduct :exec
INSERT INTO
    products(id, created_at, updated_at, name, category_id)
VALUES
    (?, ?, ?, ?, ?);

-- name: UpdateProduct :exec
UPDATE
    products
SET
    created_at = ?,
    updated_at = ?,
    name = ?,
    category_id = ?
WHERE
    id = ?;

-- name: InsertForm :exec
INSERT INTO
    product_forms(product_id, id, name)
VALUES
    (?, ?, ?);

-- name: DeleteFormsByProductID :exec
DELETE FROM
    product_forms
WHERE
    product_id = ?;
