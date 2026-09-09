-- 000002_allow_balance_overdraft.up.sql
-- Drop non-negative check constraint to allow overdraft for token usage debits (D-05)
ALTER TABLE balances DROP CONSTRAINT IF EXISTS balances_amount_kopecks_check;
