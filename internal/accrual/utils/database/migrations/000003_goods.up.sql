CREATE TABLE goods (
    id serial primary key,
    match varchar(60) not null unique,
    reward float,
    reward_type varchar(2)
);