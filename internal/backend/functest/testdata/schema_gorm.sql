-- index idx_list_user
CREATE UNIQUE INDEX `idx_list_user` ON `product_list_members`(`user_id`,`list_id`);

-- index idx_unique_product
CREATE UNIQUE INDEX `idx_unique_product` ON `favorite_products`(`product_id`,`favorite_list_id`);

-- index idx_users_login
CREATE UNIQUE INDEX `idx_users_login` ON `users`(`login`);

-- table favorite_lists
CREATE TABLE `favorite_lists` (`id` text NOT NULL,`list_type` integer NOT NULL,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,PRIMARY KEY (`id`));

-- table favorite_members
CREATE TABLE `favorite_members` (`id` text NOT NULL,`user_id` text NOT NULL,`favorite_list_id` text NOT NULL,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,`member_type` integer NOT NULL,PRIMARY KEY (`id`),CONSTRAINT `fk_favorite_lists_members` FOREIGN KEY (`favorite_list_id`) REFERENCES `favorite_lists`(`id`),CONSTRAINT `fk_favorite_members_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`));

-- table favorite_products
CREATE TABLE `favorite_products` (`id` text NOT NULL,`product_id` text NOT NULL,`favorite_list_id` text NOT NULL,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,PRIMARY KEY (`id`),CONSTRAINT `fk_favorite_lists_products` FOREIGN KEY (`favorite_list_id`) REFERENCES `favorite_lists`(`id`),CONSTRAINT `fk_favorite_products_product` FOREIGN KEY (`product_id`) REFERENCES `products`(`id`));

-- table product_categories
CREATE TABLE `product_categories` (`id` text,`name` text,PRIMARY KEY (`id`));

-- table product_forms
CREATE TABLE `product_forms` (`product_id` text,`id` text,`name` text,PRIMARY KEY (`id`),CONSTRAINT `fk_products_forms` FOREIGN KEY (`product_id`) REFERENCES `products`(`id`));

-- table product_list_members
CREATE TABLE `product_list_members` (`id` text NOT NULL,`user_id` text NOT NULL,`list_id` text NOT NULL,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,`member_type` integer NOT NULL,PRIMARY KEY (`id`),CONSTRAINT `fk_product_list_members_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`),CONSTRAINT `fk_product_lists_members` FOREIGN KEY (`list_id`) REFERENCES `product_lists`(`id`) ON DELETE CASCADE);

-- table product_list_states
CREATE TABLE `product_list_states` (`id` text NOT NULL,`product_id` text NOT NULL,`list_id` text NOT NULL,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,`index` integer NOT NULL,`count` integer,`form_idx` integer,`status` integer NOT NULL,`replacement_count` integer,`replacement_form_idx` integer,`replacement_product_id` text,PRIMARY KEY (`id`),CONSTRAINT `fk_product_list_states_product` FOREIGN KEY (`product_id`) REFERENCES `products`(`id`),CONSTRAINT `fk_product_list_states_replacement_product` FOREIGN KEY (`replacement_product_id`) REFERENCES `products`(`id`),CONSTRAINT `fk_product_lists_states` FOREIGN KEY (`list_id`) REFERENCES `product_lists`(`id`));

-- table product_lists
CREATE TABLE `product_lists` (`id` text NOT NULL,`status` integer NOT NULL,`updated_at` datetime NOT NULL,`created_at` datetime NOT NULL,`title` text,PRIMARY KEY (`id`));

-- table products
CREATE TABLE `products` (`id` text,`created_at` datetime NOT NULL,`updated_at` datetime NOT NULL,`name` text NOT NULL,`category_id` text,PRIMARY KEY (`id`),CONSTRAINT `fk_products_category` FOREIGN KEY (`category_id`) REFERENCES `product_categories`(`id`));

-- table shop_map_categories
CREATE TABLE shop_map_categories ( map_id varchar(36) NOT NULL, number integer NOT NULL, category varchar(255) NOT NULL, PRIMARY KEY (map_id, number), FOREIGN KEY(map_id) REFERENCES shop_maps(id) );

-- table shop_map_viewers
CREATE TABLE shop_map_viewers ( map_id varchar(36) NOT NULL, user_id varchar(36) NOT NULL, FOREIGN KEY (map_id) REFERENCES shop_maps(id), FOREIGN KEY (user_id) REFERENCES users(id) );

-- table shop_maps
CREATE TABLE shop_maps ( id varchar(36) PRIMARY KEY, owner_id varchar(36) NOT NULL, title varchar(255) NOT NULL, created_at timestamp NOT NULL, updated_at timestamp NOT NULL );

-- table users
CREATE TABLE "users" (id varchar(36) PRIMARY KEY,`role` integer,`login` text,`hash` text);

