CREATE TABLE withdrawals (
    id serial primary key,
    user_id INT NOT NULL,
	order_num VARCHAR(255) NOT NULL,
    accrual BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id)
);
