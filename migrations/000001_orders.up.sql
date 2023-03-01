CREATE TABLE metrics (
    user_id INT PRIMARY KEY,
	order_num VARCHAR(256),
    status VARCHAR(16)
    e_ball BIGINT,
    date_time TIMESTAMP
	);
CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_id_type ON metrics (id, type);