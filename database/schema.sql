CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    address_line3 TEXT,
    town TEXT NOT NULL,
    county TEXT NOT NULL,
    postcode TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_timestamp TEXT NOT NULL,
    updated_timestamp TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS accounts (
    account_number TEXT PRIMARY KEY
        CHECK (
            account_number GLOB
            '01[0-9][0-9][0-9][0-9][0-9][0-9]'
        ),
    user_id TEXT NOT NULL,
    sort_code TEXT NOT NULL DEFAULT '10-10-10'
        CHECK (sort_code = '10-10-10'),
    name TEXT NOT NULL,
    account_type TEXT NOT NULL
        CHECK (account_type = 'personal'),
    balance_pence INTEGER NOT NULL DEFAULT 0
        CHECK (
            balance_pence >= 0
            AND balance_pence <= 1000000
        ),
    currency TEXT NOT NULL DEFAULT 'GBP'
        CHECK (currency = 'GBP'),
    created_timestamp TEXT NOT NULL,
    updated_timestamp TEXT NOT NULL,

    FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_accounts_user_id
    ON accounts(user_id);