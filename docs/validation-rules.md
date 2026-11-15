# Validation Rules

Each rule is enforced either by the database (CHECK, UNIQUE, NOT NULL, foreign keys, triggers) or by the application. The database enforcement is in `backend/migrations/0001_init.sql`. The data model is described in [data-model.md](data-model.md).

## Ledger

| ID | Rule | Enforced by |
|---|---|---|
| L1 | Exactly one of `user_id` and `household_id` is set. | DB |
| L2 | `payday_day` is an integer from 1 to 31. | DB |
| L3 | `currency` must be an ISO 4217 code from the allowed list. Only `EUR` is active in the MVP. | App (DB only checks length) |
| L4 | Each ledger has exactly one default pot, created together with the ledger. | DB (partial unique index) + App |

## Categories

| ID | Rule | Enforced by |
|---|---|---|
| K1 | Name is not empty after trimming and has at most 50 characters. Names are unique per ledger, case-insensitive. | DB |
| K2 | A category that has transactions is archived, never deleted. | App |
| K3 | Expense categories have `is_income = false`, income categories `is_income = true`. The transaction type must match. | App |

## Pots

| ID | Rule | Enforced by |
|---|---|---|
| T1 | Name is not empty and unique among the active pots of a ledger. | DB |
| T2 | The default pot is monthly, never archived, and cannot be deleted. | DB (CHECK + trigger) |
| T3 | `target_cents` is empty or greater than 0. | DB |
| T4 | `target_date` may only be set if `target_cents` is set. | DB |
| T5 | An archived pot accepts no new transactions. Existing transactions stay visible. | App |

## Planned amounts (`pot_line`)

| ID | Rule | Enforced by |
|---|---|---|
| P1 | `amount_cents` is greater than or equal to 0. | DB |
| P2 | The category and the pot belong to the same ledger. | DB (composite foreign keys) |

## Transactions

| ID | Rule | Enforced by |
|---|---|---|
| X1 | `amount_cents` is not 0. | DB |
| X2 | Sign by type: `expense` negative, `income` positive, `transfer` positive. Input is always a positive amount, and the app sets the sign. | DB |
| X3 | `expense` and `income`: `category_id` is required, `counterpart_pot_id` is empty. | DB |
| X4 | `transfer`: `category_id` is empty, `counterpart_pot_id` is required and differs from `pot_id`. Both pots belong to the same ledger. | DB |
| X5 | `pot_id` is required. If none is given, the ledger's default pot is used. | DB (NOT NULL) + App |
| X6 | The category belongs to the same ledger as the transaction. | DB (composite foreign key) |
| X7 | Personal ledger: `user_id` is the owner. Household ledger: `user_id` is a member of the household. | App |
| X8 | `booked_on` is required and must be an ISO date. Future dates are allowed (planned transactions). | DB (format) |
| X9 | `note` has at most 500 characters. | DB |
| X10 | Deletion is a soft delete via `deleted_at`. A background job removes transactions permanently 30 days after deletion. Deleting an account removes all of its data immediately. | App (job) |
| X11 | The absolute value of `amount_cents` is at most 10,000,000,000 (100 million EUR). | DB |

## Notes

- One-time pots have no period. Transactions are assigned only through `pot_id`, so no date-range check exists for them.
- Soft-deleted transactions (`deleted_at` set) are excluded from all calculations and lists.
