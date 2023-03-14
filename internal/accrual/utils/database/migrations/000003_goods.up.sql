CREATE TABLE goods (
    id serial primary key,
    match varchar(60) not null unique,
    reward int,
    reward_type varchar(2)
);