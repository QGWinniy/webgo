CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name TEXT,
    price INT,
    description TEXT,
    availability BOOL
);

CREATE TABLE product_images (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL,
    url TEXT NOT NULL,

    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);

CREATE TABLE product_video (
    id SERIAL PRIMARY KEY,
    product_id INT NOT NULL,
    url TEXT NOT NULL,

    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);
