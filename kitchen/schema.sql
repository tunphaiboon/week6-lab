CREATE TABLE menus (
    id    SERIAL PRIMARY KEY,
    name  TEXT NOT NULL,
    price NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    type  TEXT NOT NULL,
    stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0)
);

CREATE TABLE orders (
    id         SERIAL PRIMARY KEY,
    table_no   INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    order_id INT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_id  INT NOT NULL REFERENCES menus(id),
    qty      INT NOT NULL CHECK (qty > 0),
    PRIMARY KEY (order_id, menu_id)
);

INSERT INTO menus (name, price, type, stock) VALUES
 ('ข้าวผัดกุ้ง', 80.00, 'จานเดียว', 20),
 ('ต้มยำกุ้ง', 150.00, 'ต้ม', 12),
 ('ขาหมูตุ๋น', 220.00, 'ตุ๋น', 1),
 ('ไข่เจียว', 45.00, 'จานเดียว', 50),
 ('ชาเย็น', 35.00, 'เครื่องดื่ม', 99);