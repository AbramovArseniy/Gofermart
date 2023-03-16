CREATE TABLE orders (
    order_num VARCHAR(255) PRIMARY KEY,
    user_id INT NOT NULL,
    order_status VARCHAR(16) NOT NULL,
    accrual BIGINT,
    date_time TIMESTAMP NOT NULL
);
