CREATE TABLE accrual (
    order_number varchar(60) primary key unique,
    status varchar(15),
    accrual int
);
