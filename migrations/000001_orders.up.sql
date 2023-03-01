CREATE TABLE metrics (
    user_id INT,
	order_num VARCHAR(256) PRIMARY KEY,
    status VARCHAR(16),
    e_ball BIGINT,
    date_time TIMESTAMP
	);