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
| X15 | New accounts need an email address. It is unique, case-insensitive, and used for password resets. | App + DB (unique index) |
| X16 | When a ledger is deleted, the other leg of a transfer into or out of it stays in its own ledger and loses its link. From then on it is deleted and restored on its own. | DB (`ON DELETE SET NULL`) |

## Permissions

Every member of a household may read its ledger and book in it. Changing an existing transaction is restricted. In a household the owner is the household owner; a personal ledger is always owned by its user.

| Action | Allowed for |
|---|---|
| Edit a transaction | The author and the owner of the ledger. |
| Delete or restore a transaction | The author. The owner may also delete or restore bookings of a departed member (X12), because nobody else could. For a leg of a transfer between ledgers (X14), the user must be allowed to change both legs, so a household owner cannot change a leg in someone else's personal ledger. |
| See the members of a household | Every member. Only the user ID, username and role are shown; email addresses and other personal data stay hidden. |
| Add a household member | The household owner. |
| Remove a household member | The household owner. Every member may leave by removing themselves. |
| Hand over the owner role | The household owner, to another member. The previous owner becomes a regular member, so a household always has exactly one owner. The handover takes effect immediately. |

## Accounts and logins

| ID | Rule | Enforcement |
|---|---|---|
| A1 | Username: 3 to 32 characters, lowercase letters, digits and underscore. Unique, ignoring case. | DB (unique index) + App |
| A2 | Password: 8 to 256 characters. Only the Argon2id hash is stored. | App |
| A3 | Session tokens: only the SHA-256 hash is stored. Valid for 30 days. | App |
| A4 | Password reset: the code is sent by email, valid for 30 minutes, usable once and stored only as a hash. A new code replaces older ones. After a reset, all sessions of the account end. If the mail cannot be sent, the code is removed. | App |
| A5 | The reset request answers the same way whether or not the address is registered. The mail is sent in the background, so neither the response time nor a mail error reveals it. A taken username or email gives one shared message. | App |
| A6 | Rate limits: 10 failed logins per username within 15 minutes, 3 reset requests per address within an hour. Both count unknown names too. Above the limit the API answers 429. The counters are kept in memory and start again after a restart. | App |
| A7 | The login token reaches the API as `Authorization: Bearer` (native apps) or as the `session` cookie (web app, login with `"cookie": true`; the token is then not in the response body). The cookie is HttpOnly, Secure and SameSite=Strict and lives as long as the session. A bearer token takes precedence. Logout and account deletion clear the cookie. | App |
| A8 | Requests authenticated by cookie must send the `X-Requested-With` header, otherwise 403 (CSRF protection). CORS is off unless `CORS_ORIGINS` lists exact origins; only those may send credentials and that header. | App |

## Notes

- One-time pots have no period. Transactions are assigned only through `pot_id`, so no date-range check exists for them.
- Soft-deleted transactions (`deleted_at` set) are excluded from all calculations and lists.
- A category or active pot name that is already taken answers 409.
- The transaction list returns at most 100 entries per request by default (`limit` up to 500, `offset` to page).
