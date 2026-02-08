-- name: CreateFavoriteList :exec
INSERT INTO
    favorite_lists(id, list_type, created_at, updated_at)
VALUES
    (?, ?, ?, ?);

-- name: UpdateFavoriteList :exec
UPDATE favorite_lists
SET
    list_type = ?,
    created_at = ?,
    updated_at = ?
WHERE
    id = ?;

-- name: DeleteFavoriteList :exec
DELETE FROM
    favorite_lists
WHERE
    id = ?;

-- name: CreateFavoriteMember :exec
INSERT INTO
    favorite_members(id, user_id, favorite_list_id, created_at, updated_at, member_type)
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: DeleteFavoriteMembersByListID :exec
DELETE FROM
    favorite_members
WHERE
    favorite_list_id = ?;

-- name: CreateFavoriteProduct :exec
INSERT INTO
    favorite_products(id, product_id, favorite_list_id, created_at, updated_at)
VALUES
    (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    updated_at = VALUES(updated_at);

-- name: DeleteFavoriteProductsByListID :exec
DELETE FROM
    favorite_products
WHERE
    favorite_list_id = ?;

-- name: GetFavoriteListByID :one
SELECT
    id,
    list_type,
    created_at,
    updated_at
FROM
    favorite_lists
WHERE
    id = ?
LIMIT
    1;

-- name: GetFavoriteMembersByListID :many
SELECT
    user_id,
    member_type,
    created_at,
    updated_at
FROM
    favorite_members
WHERE
    favorite_list_id = ?;

-- name: GetFavoriteProductsByListID :many
SELECT
    product_id,
    created_at,
    updated_at
FROM
    favorite_products
WHERE
    favorite_list_id = ?;

-- name: GetFavoriteListIDsByUserID :many
SELECT DISTINCT
    fl.id
FROM
    favorite_lists fl
    JOIN favorite_members fm ON fm.favorite_list_id = fl.id
WHERE
    fm.user_id = ?;

-- name: LoadProductsByIDs :many
SELECT
    p.id,
    p.created_at,
    p.updated_at,
    p.name,
    p.category_id,
    pc.name AS category_name,
    pf.id AS form_id,
    pf.name AS form_name
FROM
    products p
    LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
    LEFT JOIN product_forms pf ON pf.product_id = p.id
WHERE
    p.id IN (sqlc.slice('product_ids'))
ORDER BY
    p.id;
