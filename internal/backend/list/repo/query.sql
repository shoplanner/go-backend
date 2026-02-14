-- name: UpsertProductList :exec
INSERT INTO
    product_lists(id, status, updated_at, created_at, title)
VALUES
    (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    status = VALUES(status),
    updated_at = VALUES(updated_at),
    created_at = VALUES(created_at),
    title = VALUES(title);

-- name: GetProductListByID :one
SELECT
    id,
    status,
    updated_at,
    created_at,
    title
FROM
    product_lists
WHERE
    id = ?
LIMIT
    1;

-- name: DeleteProductListByID :exec
DELETE FROM
    product_lists
WHERE
    id = ?;

-- name: InsertProductListMember :exec
INSERT INTO
    product_list_members(id, user_id, list_id, created_at, updated_at, member_type)
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: DeleteProductListMembersByListID :exec
DELETE FROM
    product_list_members
WHERE
    list_id = ?;

-- name: GetProductListIDsByUserID :many
SELECT
    list_id
FROM
    product_list_members
WHERE
    user_id = ?;

-- name: GetMembersByListID :many
SELECT
    m.user_id,
    m.member_type,
    m.created_at,
    m.updated_at,
    COALESCE(u.login, '') AS login
FROM
    product_list_members m
    LEFT JOIN users u ON u.id = m.user_id
WHERE
    m.list_id = ?;

-- name: InsertProductListState :exec
INSERT INTO
    product_list_states(
        id,
        product_id,
        list_id,
        created_at,
        updated_at,
        `index`,
        count,
        form_idx,
        status,
        replacement_count,
        replacement_form_idx,
        replacement_product_id
    )
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteProductListStatesByListID :exec
DELETE FROM
    product_list_states
WHERE
    list_id = ?;

-- name: GetStatesByListID :many
SELECT
    product_id,
    created_at,
    updated_at,
    `index`,
    count,
    form_idx,
    status,
    replacement_count,
    replacement_form_idx,
    replacement_product_id
FROM
    product_list_states
WHERE
    list_id = ?;

-- name: UpdateStateIndexByProductID :exec
UPDATE product_list_states
SET
    `index` = ?
WHERE
    list_id = ?
    AND product_id = ?;

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
    p.id IN (sqlc.slice('product_ids'));
