CREATE TABLE accrual (
    id serial primary key,
    order_number varchar(60) unique,
    status varchar(15),
    accrual int
);
