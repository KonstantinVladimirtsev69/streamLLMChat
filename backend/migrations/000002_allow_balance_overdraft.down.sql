-- 000002_allow_balance_overdraft.down.sql
ALTER TABLE balances ADD CONSTRAINT balances_amount_kopecks_check CHECK (amount_kopecks >= 0);
