# Data Model

This document records the decisions behind the data model. The schema lives in `backend/migrations/0001_init.sql`, and the validation rules are in [validation-rules.md](validation-rules.md).

## Principles

- **Transactions are private by default.** Shared expenses will later be tracked in a household ledger.
- **Payday is a personal setting** on each ledger. The default is `1` (calendar month). Many users will later use `30`. Flexible payday calculation comes after testing with real users, but the column is added now.
- **Currency:** each ledger has one currency. The MVP only supports `EUR`. More currencies can be added later without conversion.
- **Amounts** are stored as integers in cents, never as floating-point numbers.
- **Deletion is a soft delete.** Deleted transactions stay for 30 days so household members can discuss them or restore them. After that they are removed permanently. Deleting an account removes all of its data immediately.

## Entities

### Users and households
- `user`: id, name, created_at
- `household`: id, name, created_at
- `household_member`: household_id, user_id (primary key: both fields)

### Ledger
A container for categories, pots and transactions. It belongs to exactly one user (personal) or one household (shared).

- `ledger`: id, name, user_id or household_id (exactly one is set), payday_day (default 1), currency (default `EUR`), created_at

### Categories
Categories belong to a ledger. The personal ledger has its own categories, and the household ledger has its own.

- `category`: id, ledger_id, name, is_income, archived. Names are unique per ledger.

### Pots
A pot groups planned amounts and collects transactions.

- `pot`: id, ledger_id, name, kind (`monthly` or `one_time`), is_default, target_cents (optional savings goal), target_date (optional), archived
- Each ledger has **exactly one default pot** (`is_default = true`). It is a monthly pot and can be renamed but never deleted.
- **One-time pots have no period.** They stay open until archived. Transactions are assigned to them explicitly through `pot_id`, regardless of the booking date. Example: a holiday pot can contain a flight booked in November 2027 and a balance payment from June 2028.
- The optional target date lets the frontend calculate the required monthly rate and show progress.
- Other pots could be sinking funds or a monthly savings rate.

- `pot_line`: pot_id, ledger_id, category_id, amount_cents. Planned amount for a category within a pot. Primary key: pot_id and category_id. (`ledger_id` is included so the database can check that the pot and the category belong to the same ledger.)

### Transactions
- `transactions`: id, ledger_id, user_id (who booked it), type, category_id, pot_id, counterpart_pot_id, amount_cents, booked_on, note, created_at, updated_at, deleted_at

**Types:**
- `expense`: negative amount, category required
- `income`: positive amount, category required
- `transfer`: positive amount moved between two pots. `pot_id` is the source pot and `counterpart_pot_id` the target pot. No category.

**Assignment:**
- `pot_id` is **always set**. If none is given, the ledger's default pot is used.
- Assignment to a pot is **explicit**, not derived from the date.
- A transaction belongs to a monthly pot's period based on `booked_on`. The period is derived from `payday_day`.

**Deletion:**
- Deleted transactions have `deleted_at` set and are excluded from all calculations and lists.
- A background job permanently removes transactions once `deleted_at` is more than 30 days old.

## Calculations

- **Period of a monthly pot:** starts on `payday_day` of a month and ends the day before `payday_day` of the next month. If the day does not exist in a month (for example the 30th in February), the last day of the month is used. This is implemented with the flexible payday feature.
- **Balance of a pot:** incoming transfers and income, minus expenses and outgoing transfers. Deleted transactions are excluded.
- **Progress toward a savings goal:** the pot balance relative to `target_cents`. Balances are calculated on demand and not stored.
- **Required monthly rate:** the remaining amount up to `target_cents`, divided by the number of months until `target_date`. Only available when a target date is set.

## Deferred

- Tracking savings rates for stocks or ETFs. Price histories will not be tracked, and the approach is still open.
- Sinking funds with recurring contributions (probably transfers on a schedule).
- Flexible payday calculation (after testing with real users).
- Shared expenses in household ledgers: permissions and visibility.
- Gamification and rewards.
