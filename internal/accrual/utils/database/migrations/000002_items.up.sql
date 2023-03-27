CREATE TABLE items (
    id serial primary key,
    order_number varchar(60),
    description varchar(255),
    price float
);