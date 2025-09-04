
-- Users table
CREATE TABLE users (
    id SERIAL UNIQUE NOT NULL PRIMARY KEY,
    login TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


-- Order statuses
CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

-- Orders table
CREATE TABLE orders (      
    order_number TEXT UNIQUE NOT NULL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,  
    status order_status NOT NULL DEFAULT 'NEW',
    accrual DECIMAL(10, 2) CHECK (accrual >= 0),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes for orders table
CREATE INDEX idx_orders_user_id ON orders (user_id);
CREATE INDEX idx_orders_status ON orders (status);;

-- Withdrawals table
CREATE TABLE withdrawals (
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number TEXT NOT NULL,
    summ DECIMAL(10, 2) NOT NULL CHECK (summ > 0),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, order_number)
);

-- Indexes for withdrawals table
CREATE INDEX idx_withdrawals_user_processed_at ON withdrawals (user_id, processed_at DESC);
