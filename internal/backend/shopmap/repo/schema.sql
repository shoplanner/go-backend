-- name: InitShopMaps :exec
CREATE TABLE IF NOT EXISTS shop_maps (
    id varchar(36) PRIMARY KEY,
    owner_id varchar(36) NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);

-- name: InitShopMapViewers :exec
CREATE TABLE IF NOT EXISTS shop_map_viewers (
    map_id varchar(36) NOT NULL,
    user_id varchar(36) NOT NULL,
    FOREIGN KEY (map_id) REFERENCES shop_maps(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- name: InitShopMapCategories :exec
CREATE TABLE IF NOT EXISTS shop_map_categories (
    map_id varchar(36) NOT NULL PRIMARY KEY,
    number int UNSIGNED NOT NULL PRIMARY KEY,
    category varchar(255) NOT NULL,
    FOREIGN KEY(map_id) REFERENCES shop_map(id)
)
