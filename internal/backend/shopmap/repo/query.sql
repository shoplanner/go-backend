-- name: GetByID :many
SELECT
    m.id,
    m.owner_id,
    m.created_at,
    m.updated_at,
    v.user_id,
    c.category
FROM
    shop_maps AS m
    JOIN shop_map_categories AS c ON m.id = c.map_id
    JOIN shop_map_viewers AS v ON m.id = v.map_id
WHERE
    m.id = ?;

-- name: GetByUserID :many
SELECT
    m.id,
    m.owner_id,
    m.created_at,
    m.updated_at,
    v.user_id,
    c.category
FROM
    shop_maps AS m
    JOIN shop_map_categories AS c ON m.id = c.map_id
    JOIN shop_map_viewers AS v ON m.id = v.map_id
WHERE
    v.user_id = ?;

-- name: CreateShopMap :exec
INSERT INTO
    shop_maps(
        id,
        owner_id,
        created_at,
        updated_at
    )
VALUES
    (?, ?, ?, ?);

-- name: InsertViewers :copyfrom
INSERT INTO
    shop_map_viewers(map_id, user_id)
VALUES
    (?, ?);

-- name: InsertCategories :copyfrom
INSERT INTO
    shop_map_categories(map_id, category)
VALUES
    (?, ?);

-- name: UpdateShopMap :exec
UPDATE
    shop_maps
SET
    owner_id = ?,
    updated_at = ?,
    created_at = ?
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

-- name: DeleteCategories :exec
DELETE FROM
    shop_map_categories
WHERE
    map_id = ?;
