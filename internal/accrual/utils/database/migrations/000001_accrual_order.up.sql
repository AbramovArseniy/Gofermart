CREATE TABLE accrual (
    order_number varchar(60) primary key unique,
    status varchar(15),
    accrual int
);

CREATE TABLE items (
    id serial primary key,
    order_number varchar(60) references accrual(order_number),
    description varchar(255),
    price int
);

