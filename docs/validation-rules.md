# Validation Rules

Each rule is enforced either by the database (CHECK, UNIQUE, NOT NULL, foreign keys, triggers) or by the application. The database enforcement is in the migrations under `backend/migrations/`, starting with `0001_init.sql`. The data model is described in [data-model.md](data-model.md).

## Ledger

| ID | Rule | Enforced by |
|---|---|---|
| L1 | Exactly one of `user_id` and `household_id` is set. | DB |
| L2 | `payday_day` is an integer from 1 to 31. | DB |
| L3 | `currency` must be an ISO 4217 code from the allowed list. Only `EUR` is active in the MVP. | App (DB only checks length) |
| L4 | Each ledger has exactly one default pot, created together with the ledger. | DB (partial unique index) + App |
| L5 | A household has exactly one ledger, created together with the household. | DB (unique index) + App |
| L6 | Every account gets a personal ledger named "Personal" at registration, in the same transaction. It is the user's primary ledger (`is_primary`); `GET /me` returns it as `personal_ledger_id`. Further personal ledgers are allowed but never primary. | DB (partial unique index) + App |
| L7 | A ledger's balance is given as of one day (`date`, default: today on the server; the app sends its local today). `available_cents` is the default pot, `total_cents` all pots including archived ones. Transactions after that day and deleted ones do not count. | App |

## Categories

| ID | Rule | Enforced by |
|---|---|---|
| K1 | Name is not empty after trimming and has at most 50 characters. Names are unique per ledger, case-insensitive. | DB |
| K2 | Deleting a category moves all its transactions, soft-deleted ones included, to a target category, and adds its planned amounts to the target's planned amounts in the same pots (templates and monthly plans, month by month; P3 may reject the move with 409). The owner may choose the target (`move_to`): another active category of the same kind, otherwise the answer is 400. Without a choice, the target is the fallback category of the same kind (K6): expenses to "Other", income to "Other income". Its recurring payments switch to the target too (R8). The answer names the target (`moved_to`) and the number of moved transactions. | App |
| K3 | Expense categories have `is_income = false`, income categories `is_income = true`. The transaction type must match. | App |
| K4 | Every new ledger (personal or household) starts with 14 default categories (Salary, Other income, Groceries, Rent, Utilities, Transport, Insurance, Health, Eating out, Leisure, Clothing, Subscriptions, Household, Other). Expense categories come with a purpose (K5): need for Groceries, Rent, Utilities, Transport, Insurance, Health and Household; want for Eating out, Leisure, Clothing and Subscriptions; none for Other. Users rename, archive or delete them like any other category. Migration 0012 added the names, without purposes, to existing ledgers that had no category at all. | App |
| K5 | An expense category may have a `purpose`: `need`, `want` or `saving` (UI: Needs, Wants, Savings). It is optional. Income categories never have one. | DB + App |
| K6 | Each ledger has at most one fallback category per kind (`is_fallback`): "Other" for expenses, "Other income" for income. They are recognised by the flag, not by their name, so they may be renamed, but never archived or deleted. A ledger without a fallback gets one when it first deletes a category of that kind: an existing category with the fallback name becomes the fallback (and is restored if it was archived); otherwise a new one is created, even beyond the limit of K7. If the name is used by a category of the other kind, the deletion is refused until that category is renamed. Migration 0013 marked the existing active categories named like the fallbacks. | DB (partial unique index, CHECK) + App |
| K7 | A ledger has at most 50 categories, archived ones included. | App |
| K8 | An archived category accepts no new transactions, and no transaction can move into it. Existing transactions keep it and can still be corrected. | App |
| K9 | Every member may create categories; only the owner of the ledger may rename, archive or delete them or change their purpose (see Permissions). Others get 403. | App |

## Pots

| ID | Rule | Enforced by |
|---|---|---|
| T1 | Name is not empty and unique among the active pots of a ledger. | DB |
| T2 | The default pot is monthly, never archived, and cannot be deleted. | DB (CHECK + trigger) |
| T3 | `target_cents` is empty or greater than 0. | DB |
| T4 | `target_date` may only be set if `target_cents` is set. | DB |
| T5 | An archived pot accepts no new transactions. Existing transactions stay visible. | App |

## Planned amounts (`pot_line`, `period_plan`)

| ID | Rule | Enforced by |
|---|---|---|
| P1 | `amount_cents` is greater than or equal to 0. | DB |
| P2 | The category and the pot belong to the same ledger. | DB (composite foreign keys) |
| P3 | A category is planned in at most one monthly pot of a ledger, per month and in the templates. One-time pots are not limited. Otherwise the answer is 409. | DB (`period_plan`), App (templates) |
| P4 | The plan of a month is copied from the templates of all active monthly pots once, on the first access in that month. Future months show the template without storing it; past months are never filled in afterwards. | App |
| P5 | Only the plan of the current month can be changed, with `scope` `this_month` or `from_now_on` (also changes the template). One-time pots take no scope. Otherwise the answer is 400. | App |
| P6 | Reports of a monthly pot cover one month (`month`, default the current one); `from`/`to` are rejected with 400. | App |

## Recurring payments (`recurring_payment`, `recurring_skip`)

A recurring payment is a reminder. It is never booked automatically.

| ID | Rule | Enforced by |
|---|---|---|
| R1 | `amount_cents` is greater than 0 and at most 10,000,000,000. The name has 1 to 50 characters. | DB + App |
| R2 | Only expenses: the category is an active expense category of the same ledger, the pot an active pot of the same ledger (empty: the default pot). Otherwise 400 (income category) or 409 (archived). | DB (composite foreign keys) + App |
| R3 | Schedule: every `interval_months` months (1, 3, 6 or 12) on `due_day` (1–31), counted from `first_month`. A day that a month lacks falls back to the month's last day. | DB + App |
| R4 | `last_month` is optional (no end) and not before `first_month`. | DB + App |
| R5 | Occurrences due before `tracked_from` are never open. It is the app's date when the payment was created, and moves to the app's date when the due dates change (`interval_months`, `due_day`, `first_month`) or the payment leaves the archive. Changing the amount, name, category, pot or end keeps it. | App |
| R6 | An expense may pay one occurrence (`recurring_payment_id` and `recurring_due_on`, set together and only on creation). The date must be a due date of an active payment of the same ledger. At most one booking per occurrence that is not deleted (otherwise 409); deleting it opens the occurrence again, restoring it fails with 409 while another booking pays it. The booking cannot become income or a transfer. | DB (CHECK, unique index) + App |
| R7 | Every member may skip an occurrence that is not booked and undo the skip. A skipped occurrence cannot be booked (409). | App |
| R8 | Every member may create recurring payments; only the owner of the ledger may change or archive them (otherwise 403). They are not deleted, only archived. Deleting a category moves its recurring payments along with its bookings (K2). | App |
| R9 | Changing the category of a booking that pays a recurring payment may take `category_scope`: `this` (default), `this_and_earlier`, `this_and_later` or `all`. The other bookings of the same payment in that range (by the due date they pay, the edited one included) get the new category; deleted bookings keep theirs. The owner changes every booking, a member only their own. `this_and_later` and `all` also change the payment's category, so only the owner may use them (403). A scope other than `this` needs `category_id` and a booking of a recurring payment, otherwise 400. | App |

Open occurrences (`GET /ledgers/{ledger}/recurring-payments/due?date=`) are those of active payments from `tracked_from` to the end of the current payday period that are neither booked nor skipped, each with `status` `overdue`, `due` (today) or `upcoming`.

## Transactions

| ID | Rule | Enforced by |
|---|---|---|
| X1 | `amount_cents` is not 0. | DB |
| X2 | Sign by type: `expense` and `transfer_out` negative, `income`, `transfer` and `transfer_in` positive. Input is always a positive amount, and the app sets the sign. | DB |
| X3 | `expense` and `income`: `category_id` is required, `counterpart_pot_id` and `linked_transaction_id` are empty. | DB |
| X4 | `transfer`: `category_id` and `linked_transaction_id` are empty, `counterpart_pot_id` is required and differs from `pot_id`. Both pots belong to the same ledger. | DB |
| X5 | `pot_id` is required. If none is given, the ledger's default pot is used. | DB (NOT NULL) + App |
| X6 | The category belongs to the same ledger as the transaction. | DB (composite foreign key) |
| X7 | Personal ledger: `user_id` is the owner. Household ledger: `user_id` is a member of the household. | App |
| X8 | `booked_on` is required and must be an ISO date. Future dates are allowed (planned transactions). | DB (format) |
| X9 | `note` has at most 500 characters. | DB |
| X10 | Deletion is a soft delete via `deleted_at`. A background job removes transactions permanently 30 days after deletion. Deleting an account removes all of its data immediately. | App (job) |
| X11 | The absolute value of `amount_cents` is at most 10,000,000,000 (100 million EUR). | DB |
| X12 | When a member leaves a household or deletes the account, the member's bookings and deletions in the household are handed over to the placeholder user `former-member`. They stay visible to the other members. | App |
| X13 | `deleted_by` records who deleted a transaction. It is only set while the transaction is soft-deleted. | DB (CHECK) + App |
| X14 | A transfer between two ledgers is booked as two legs: `transfer_out` (negative) in the source ledger and `transfer_in` (positive) in the target ledger. The legs have no category and no counterpart pot and point at each other with `linked_transaction_id`. They are created, deleted and restored together and cannot be edited. | DB (CHECK) + App |
| X15 | Accounts have no email address; the app collects none. A forgotten password is reset with the recovery code (A4) or by the server operator (A9). | App |
| X16 | When a ledger is deleted, the other leg of a transfer into or out of it stays in its own ledger and loses its link. From then on it is deleted and restored on its own. | DB (`ON DELETE SET NULL`) |
| X17 | Transaction lists name the other side of a transfer between ledgers as `counterpart`: `kind` (`personal` or `household`) always, `ledger_id` and `name` only if the viewer may access that ledger. A member therefore never learns the ID or name of another member's personal ledger. Legs that lost their link (X16) have no counterpart. | App |
| X18 | Transaction lists name the author of each booking by `username`, the same name the member list shows (never the display name). It is `null` for the `former-member` placeholder, so departed members stay anonymous. The list of deleted transactions names the deleting user the same way as `deleted_by_username`. | App |
| X19 | Transaction lists tell the viewer, per booking, whether they may change it: `can_edit` and `can_delete` follow the rules for editing and deleting below. A transfer between ledgers (X14) is never editable, and deleting one of its legs needs the right on both legs. The flags only let the app hide actions; the server still checks every change. | App |
| X20 | Only expenses can be marked as spontaneous (`is_spontaneous`, UI "⚡ Spontaneous"). Changing an expense into income or a transfer removes the mark. | DB + App |

## Permissions

Every member of a household may read its ledger and book in it. Changing an existing transaction is restricted. In a household the owner is the household owner; a personal ledger is always owned by its user.

| Action | Allowed for |
|---|---|
| Edit a transaction | The author and the owner of the ledger. |
| Delete or restore a transaction | The author. The owner may also delete or restore bookings of a departed member (X12), because nobody else could. For a leg of a transfer between ledgers (X14), the user must be allowed to change both legs, so a household owner cannot change a leg in someone else's personal ledger. |
| Mark a transaction as spontaneous or remove the mark (X20) | Only the author, not even the owner, because the mark describes the author's own decision. |
| Create a category | Every member of the ledger. |
| Rename, archive, delete a category or change its purpose | The owner of the ledger (in a household the household owner). |
| Create a recurring payment, book or skip an occurrence, undo a skip | Every member of the ledger (R7, R8). |
| Change or archive a recurring payment | The owner of the ledger (R8). |
| Give earlier bookings of a recurring payment the new category (R9) | The owner for all bookings; a member for their own. |
| Give later or all bookings the new category, which also changes the payment (R9) | The owner of the ledger. |
| See the members of a household | Every member. Only the user ID, username and role are shown; the display name and other personal data stay hidden. |
| Add a household member | The household owner. |
| Remove a household member | The household owner. Every member may leave by removing themselves. |
| Hand over the owner role | The household owner, to another member. The previous owner becomes a regular member, so a household always has exactly one owner. The handover takes effect immediately. |

## Accounts and logins

| ID | Rule | Enforcement |
|---|---|---|
| A1 | Username: 3 to 32 characters, lowercase letters, digits and underscore. Unique, ignoring case. | DB (unique index) + App |
| A2 | Password: 8 to 256 characters. Only the Argon2id hash is stored. | App |
| A3 | Session tokens: only the SHA-256 hash is stored. Valid for 30 days. | App |
| A4 | Recovery code: six words from the EFF Large Wordlist (7776 words, about 77 bits), created at registration and shown only in that answer. Only its Argon2id hash is stored. With username, code and a new password (`POST /auth/recover`) the password is reset; the code is used up and the answer carries a new one. All sessions of the account end. Case and separators (spaces, commas) do not matter when typing the code. A new code is only issued after the code was used (or by A9), not on request. | App |
| A5 | Recovery answers the same way (401, "wrong username or recovery code") for an unknown username and a wrong code; for unknown names a dummy hash is checked, so the response time matches. | App |
| A6 | Rate limits: 10 failed logins per username within 15 minutes, 5 failed recoveries per username within an hour. Both count unknown names too. A wrong current password when changing the password or deleting the account counts as a failed login. A successful recovery clears both counters of the name. Above the limit the API answers 429. The counters are kept in memory and start again after a restart. | App |
| A7 | The login token reaches the API as `Authorization: Bearer` (native apps) or as the `session` cookie (web app, login with `"cookie": true`; the token is then not in the response body). The cookie is HttpOnly, Secure and SameSite=Strict and lives as long as the session. `COOKIE_SECURE=false` drops only the Secure flag, for a home network without HTTPS; the server then logs a warning at startup. Any value other than a boolean stops the server. A bearer token takes precedence. Logout and account deletion clear the cookie. | App |
| A8 | Requests authenticated by cookie must send the `X-Requested-With` header, otherwise 403 (CSRF protection). CORS is off unless `CORS_ORIGINS` lists exact origins; only those may send credentials and that header. | App |
| A9 | Password change (`PUT /auth/password`) needs the current password. All other sessions of the account end; the one making the request stays. The recovery code stays the same. The server operator can run `server reset-password <username>` on the server: it sets a temporary password (four words) and a new recovery code, ends all sessions and prints both. It is not reachable over the network. | App |

## Notes

- One-time pots have no period. Transactions are assigned only through `pot_id`, so no date-range check exists for them.
- Soft-deleted transactions (`deleted_at` set) are excluded from all calculations and lists.
- A category or active pot name that is already taken answers 409.
- The transaction list and the list of deleted transactions return at most 100 entries per request by default (`limit` up to 500, `offset` to page).
