package ledger

// FormerMemberID is the placeholder user that replaces a departed member in
// household transactions (rule X12). It is created by migration 0003 and has
// no username or password, so nobody can log in as it.
const FormerMemberID = "former-member"
