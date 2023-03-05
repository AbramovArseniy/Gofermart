CREATE TABLE orders (
    order_num VARCHAR(255) PRIMARY KEY,
    user_id INT NOT NULL,
    status VARCHAR(16) NOT NULL
    accrual BIGINT,
    date_time TIMESTAMP NOT NULL
);