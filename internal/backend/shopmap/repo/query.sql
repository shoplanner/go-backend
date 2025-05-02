-- name: GetByID :one
SELECT
    *
FROM
    shop_maps AS m
WHERE
    m.id = ?
LIMIT
    1;

-- name: GetByOwnerID :many
SELECT
    *
FROM
    shop_maps AS m
WHERE
    m.owner_id = ?;

-- name: GetByListID :many
SELECT
    *
FROM
    shop_maps
WHERE
    id IN (sqlc.slice('map_ids'));

-- name: GetCategoriesByID :many
SELECT
    *
FROM
    shop_map_categories
WHERE
    map_id = ?;

-- name: GetViewersByMapID :many
SELECT
    *
FROM
    shop_map_viewers
WHERE
    map_id = ?;

-- name: GetMapsWithViewer :many
SELECT
    map_id
FROM
    shop_map_viewers
WHERE
    user_id = ?;

-- name: CreateShopMap :exec
INSERT INTO
    shop_maps(
        id,
        title,
        owner_id,
        created_at,
        updated_at
    )
VALUES
    (?, ?, ?, ?, ?);

-- name: InsertViewers :copyfrom
INSERT INTO
    shop_map_viewers(map_id, user_id)
VALUES
    (?, ?);

-- name: InsertCategories :copyfrom
INSERT INTO
    shop_map_categories(map_id, number, category)
VALUES
    (?, ?, ?);

-- name: UpdateShopMap :exec
UPDATE
    shop_maps
SET
    title = ?,
    owner_id = ?,
    updated_at = ?
WHERE
    id = ?;

-- name: DeleteShopMap :exec
DELETE FROM
    shop_maps
WHERE
    id = ?;

-- name: DeleteViewers :exec
DELETE FROM
    shop_map_viewers
WHERE
    map_id = ?;

-- name: DeleteCategoriesByMapID :exec
DELETE FROM
    shop_map_categories
WHERE
    map_id = ?;

-- name: GetCategoriesByListID :many
SELECT
    *
FROM
    shop_map_categories
WHERE
    map_id IN (sqlc.slice('map_ids'));

-- name: GetViewersByListID :many
SELECT
    *
FROM
    shop_map_viewers
WHERE
    map_id IN (sqlc.slice('map_ids'));

-- name: DeleteViewersByListID :exec
DELETE FROM
    shop_map_viewers
WHERE
    user_id IN (sqlc.slice('user_ids'));
