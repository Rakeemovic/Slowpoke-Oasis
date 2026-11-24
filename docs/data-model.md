# Data Model

This document records the decisions behind the data model. The schema lives in the migrations under `backend/migrations/`, and the validation rules are in [validation-rules.md](validation-rules.md).

## Principles

- **Transactions are private by default.** Shared expenses are tracked in a household ledger that all members can read.
- **Payday is a personal setting** on each ledger. The default is `1` (calendar month). Reports for a month cover the payday period that starts in that month.
- **Currency:** each ledger has one currency. The MVP only supports `EUR`. More currencies can be added later without conversion.
- **Amounts** are stored as integers in cents, never as floating-point numbers.
- **Deletion is a soft delete.** Deleted transactions stay for 30 days so household members can discuss them or restore them. After that they are removed permanently. Deleting an account removes all of its data immediately.

## Entities

### Users and households
- `user`: id, name, username, password_hash, recovery_hash, created_at. The username is unique, ignoring case. `recovery_hash` is the Argon2id hash of the recovery code (rule A4); it is empty for accounts created before migration `0011`. The placeholder user `former-member` has no username and no password and cannot log in. No email address is stored.
- `session`: token_hash, user_id, created_at, expires_at. A login is valid for 30 days. Only the SHA-256 hash of the token is stored.
- `household`: id, name, created_at
- `household_member`: household_id, user_id (primary key: both fields), role (`owner` or `member`), joined_at. The creator is the owner. A household has exactly one owner: the owner can hand the role to another member and becomes a regular member. When the owner leaves, the longest-standing member (earliest `joined_at`) becomes owner. When the last member leaves, the household and its ledger are deleted.

### Ledger
A container for categories, pots and transactions. It belongs to exactly one user (personal) or one household (shared). A household has exactly one ledger, so the household list can return it directly as `ledger_id`. Every account gets a personal ledger at registration; it is marked as primary and returned by `GET /me` (rule L6). Users may create more personal ledgers, which are not primary.

- `ledger`: id, name, user_id or household_id (exactly one is set), payday_day (default 1), currency (default `EUR`), is_primary (only for the personal ledger created at registration, at most one per user), created_at

### Categories
Categories belong to a ledger. The personal ledger has its own categories, and the household ledger has its own.

- `category`: id, ledger_id, name, is_income, archived, purpose (`need`, `want`, `saving` or NULL; expense categories only), is_fallback. Names are unique per ledger. The category list also contains archived categories, because older transactions still refer to them, and gives the number of transactions per category. New ledgers start with a set of default categories (rule K4). Deleting a category moves its transactions to the fallback category of the same kind (rules K2, K6).

### Pots
A pot groups planned amounts and collects transactions.

- `pot`: id, ledger_id, name, kind (`monthly` or `one_time`), is_default, target_cents (optional savings goal), target_date (optional), archived, reserve_for (the recurring payment this one-time pot is the reserve of, rule R10)
- Each ledger has **exactly one default pot** (`is_default = true`), called "Main" ("Monthly" before migration 0017). It holds the money of the account, is a monthly pot and can be renamed but never deleted.
- **One-time pots have no period.** They stay open until archived. Transactions are assigned to them explicitly through `pot_id`, regardless of the booking date. Example: a holiday pot can contain a flight booked in November 2027 and a balance payment from June 2028.
- The optional target date lets the frontend calculate the required monthly rate and show progress.
- A **reserve** is a one-time pot that saves for a recurring payment due every 3, 6 or 12 months (rules R10–R14). The payment books from it; the app suggests a rate each payday period and offers to top it up, because a reserve never goes below zero.

- `pot_line`: pot_id, ledger_id, category_id, amount_cents. Planned amount for a category within a pot. Primary key: pot_id and category_id. (`ledger_id` is included so the database can check that the pot and the category belong to the same ledger.) For the main pot it is the **template** that each new month's plan is copied from. Only the main pot has a budget (rule P7); lines of other pots are old data that is kept but no longer changed.
- `period_plan`: pot_id, ledger_id, category_id, month, amount_cents. The plan of the main pot for one month (older rows may belong to other monthly pots). `month` (`YYYY-MM`) is the month in which the payday period starts. Primary key: pot_id, category_id and month. Only monthly pots have rows here (rules P3, P4, P7).
- `period_plan_month`: ledger_id, month. Marks the months of a ledger whose plans were created from the templates, so that a month whose lines were removed is not filled again.

**Monthly plan:**
- The plan of the current month is copied from the templates of all active monthly pots of the ledger on first access (rule P4). Later changes to the template do not change it.
- The current month is the payday period that contains the app's local date (`?date=`), because the server may run in another time zone.
- Future months are not stored: they show the template as a preview.
- Past months are never filled in afterwards. A past month that was never opened has no plan.
- Only the current month can be changed (rule P5): "this month only" (`scope=this_month`) or "this month and the months to come" (`scope=from_now_on`, also changes the template).

### Recurring payments
- `recurring_payment`: id, ledger_id, name, amount_cents, category_id, pot_id, interval_months (1, 3, 6, 12), due_day (1–31), first_month, last_month (optional, `YYYY-MM`), tracked_from (date from which occurrences can be open), archived, created_at. Only expenses (rules R1–R5, R8). A payment due every few months books from its reserve (`pot_id` = the pot whose `reserve_for` is the payment, rule R10).
- `recurring_skip`: recurring_payment_id, due_on. Occurrences a member chose not to book (rule R7).

**Occurrences** are not stored. They are calculated from the schedule: one due date per `interval_months` months from `first_month` on. An occurrence is *open* if it is due on or after `tracked_from`, is neither booked nor skipped, and the payment is not archived. The app lists open occurrences up to the end of the current payday period and books one with a tap as a normal expense that remembers the occurrence (rule R6). Nothing is booked automatically.

### Transactions
- `transactions`: id, ledger_id, user_id (who booked it), type, category_id, pot_id, counterpart_pot_id, amount_cents, booked_on, note, created_at, updated_at, deleted_at, deleted_by (who deleted it), linked_transaction_id (the other leg of a transfer between ledgers), is_spontaneous (expenses only, set by the author, rule X20), recurring_payment_id and recurring_due_on (the occurrence of a recurring payment that the expense pays, rule R6)

**Types:**
- `expense`: negative amount, category required
- `income`: positive amount, category required
- `transfer`: positive amount moved between two pots of the same ledger. `pot_id` is the source pot and `counterpart_pot_id` the target pot. No category.
- `transfer_out` and `transfer_in`: the two legs of a transfer between two ledgers, for example from a personal ledger into a household ledger. The outgoing leg is negative and sits in the source ledger, the incoming leg is positive and sits in the target ledger. Each leg points at the other with `linked_transaction_id`. No category.

**Assignment:**
- `pot_id` is **always set**. If none is given, the ledger's default pot is used.
- Assignment to a pot is **explicit**, not derived from the date.
- A transaction belongs to a monthly pot's period based on `booked_on`. The period is derived from `payday_day`.

**Deletion:**
- Deleted transactions have `deleted_at` set and are excluded from all calculations and lists.
- A background job permanently removes transactions once `deleted_at` is more than 30 days old. Until then they can be restored.
- The two legs of a transfer between ledgers are deleted and restored together.
- If a whole ledger is deleted, the leg in the other ledger stays, because the money did arrive there or leave from there. It loses its link.

## Calculations

- **Period of a monthly pot:** starts on `payday_day` of a month and ends the day before `payday_day` of the next month. If the day does not exist in a month (for example the 30th in February), the last day of the month is used.
- **Plan versus actual of a pot** (`GET /ledgers/{ledger}/pots/{pot}/summary`): planned and booked amounts per category, plus transfers in and out. A monthly pot is always reported for one month (`?month=`, default the current month); `plan_source` says whether the planned amounts are the stored plan of the month (`month`), a preview of the template (`preview`) or missing (`none`). A one-time pot uses its own plan (`pot`) and any date range.
- **Balance of a pot:** incoming transfers and income, minus expenses and outgoing transfers. Transfers between ledgers count like transfers between pots. Deleted transactions are excluded.
- **Balance of a ledger** (`GET /ledgers/{ledger}/balance`, rule L7): *available* is the balance of the default pot, *total* the sum of all pots. Only transactions booked on or before the given day count, so planned transactions do not lower it early.
- **Progress toward a savings goal:** the pot balance relative to `target_cents`. Balances are calculated on demand and not stored.
- **Required monthly rate:** the remaining amount up to `target_cents`, divided by the number of months until `target_date`. Only available when a target date is set.
- **Rate of a reserve** (rule R14): what is missing up to the payment's amount without this period's savings, divided by the payday periods until the next open occurrence, rounded up to the cent. It corrects itself: too little or too much this period changes the next rates.

## Deferred

- Tracking savings rates for stocks or ETFs. Price histories will not be tracked, and the approach is still open.
- Gamification and rewards.
