-- name: GetListByID :one
SELECT
    *
FROM
    product_lists
WHERE
    id = ?
LIMIT
    1;

-- name: GetListsByIDList :many
SELECT
    *
FROM
    product_lists
WHERE
    id IN (sqlc.slice('ids'));

-- name: GetListIDListByUserID :many
SELECT
    list_id
FROM
    product_list_members
WHERE
    user_id = ?;

-- name: GetMembersByListIDList :many
SELECT
    *
FROM
    product_list_members
WHERE
    list_id IN (sqlc.slice('list_ids'))
ORDER BY
    rowid;

-- name: GetStatesByListIDList :many
SELECT
    *
FROM
    product_list_states
WHERE
    list_id IN (sqlc.slice('list_ids'))
ORDER BY
    rowid;

-- name: InsertList :exec
INSERT INTO
    product_lists(id, status, updated_at, created_at, title)
VALUES
    (?, ?, ?, ?, ?);

-- name: UpdateList :exec
UPDATE
    product_lists
SET
    status = ?,
    updated_at = ?,
    created_at = ?,
    title = ?
WHERE
    id = ?;

-- name: DeleteList :exec
DELETE FROM
    product_lists
WHERE
    id = ?;

-- name: InsertMember :exec
INSERT INTO
    product_list_members(
        id,
        user_id,
        list_id,
        created_at,
        updated_at,
        member_type
    )
VALUES
    (?, ?, ?, ?, ?, ?);

-- name: DeleteMembersByListID :exec
DELETE FROM
    product_list_members
WHERE
    list_id = ?;

-- name: InsertState :exec
INSERT INTO
    product_list_states(
        id,
        product_id,
        list_id,
        created_at,
        updated_at,
        "index",
        count,
        form_idx,
        status,
        replacement_count,
        replacement_form_idx,
        replacement_product_id
    )
VALUES
    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteStatesByListID :exec
DELETE FROM
    product_list_states
WHERE
    list_id = ?;
