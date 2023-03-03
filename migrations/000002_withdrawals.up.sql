CREATE TABLE withdrawals (
    user_id INT PRIMARY KEY,
	order_num VARCHAR(256),
    e_ball_sum BIGINT,
    processed_at TIMESTAMP
);
