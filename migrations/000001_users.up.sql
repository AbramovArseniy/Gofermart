CREATE TABLE  users (
	id SERIAL PRIMARY KEY,
	login VARCHAR UNIQUE NOT NULL,
	password_hash VARCHAR NOT NULL,
)
