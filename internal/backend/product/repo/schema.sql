-- name: InitProducts :exec
CREATE TABLE products (
    id varchar(36) PRIMARY KEY,
    name varchar(255) NOT NULL,
    category_id varchar(36),
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);

-- name: InitProductForms :exec
CREATE TABLE product_forms (
    product_id varchar(36) PRIMARY KEY,
    name varchar(255) NOT NULL,
    FOREIGN KEY (productId) REFERENCES products(id)
);

-- name: InitProductCategories :exec
CREATE TABLE product_categories (
    id varchar(36) PRIMARY KEY,
    name varchar(255) NOT NULL
)
