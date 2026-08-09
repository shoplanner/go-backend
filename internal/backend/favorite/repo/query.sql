-- name: GetListByID :one
SELECT
    *
FROM
    favorite_lists
WHERE
    id = ?
LIMIT
    1;

-- name: GetListsByIDList :many
SELECT
    *
FROM
    favorite_lists
WHERE
    id IN (sqlc.slice('ids'));

-- name: GetListIDListByUserID :many
SELECT
    favorite_list_id
FROM
    favorite_members
WHERE
    user_id = ?;

-- name: GetMembersByListIDList :many
SELECT
    *
FROM
    favorite_members
WHERE
    favorite_list_id IN (sqlc.slice('list_ids'))
ORDER BY
    rowid;

-- name: GetProductsByListIDList :many
SELECT
    *
FROM
    favorite_products
WHERE
    favorite_list_id IN (sqlc.slice('list_ids'))
ORDER BY
    rowid;

-- name: InsertList :exec
INSERT INTO
    favorite_lists(id, list_type, created_at, updated_at)
VALUES
    (?, ?, ?, ?);

-- name: UpdateList :exec
UPDATE
    favorite_lists
SET
    list_type = ?,
    created_at = ?,
    updated_at = ?
WHERE
    id = ?;

-- name: InsertMember :exec
INSERT INTO
    favorite_members(
        id,
        user_id,
        favorite_list_id,
        created_at,
        updated_at,
        member_type
    )
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: DeleteMembersByListID :exec
DELETE FROM
    favorite_members
WHERE
    favorite_list_id = ?;

-- Adding the same product twice must not duplicate the row; the collection is rewritten
-- wholesale on every update, so the conflict can only come from a duplicate inside one batch.
-- name: InsertProduct :exec
INSERT INTO
    favorite_products(
        id,
        product_id,
        favorite_list_id,
        created_at,
        updated_at
    )
VALUES
    (?, ?, ?, ?, ?) ON CONFLICT (product_id, favorite_list_id) DO
UPDATE
SET
    updated_at = excluded.updated_at;

-- name: DeleteProductsByListID :exec
DELETE FROM
    favorite_products
WHERE
    favorite_list_id = ?;
