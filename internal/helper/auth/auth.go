// Package auth holds the small surface that both the helper server and the
// CLI installer need to share — the SID allow-list path and read/write
// helpers.
package auth

import "errors"

// AllowedSIDFile is the on-disk allow-list of SIDs that may connect to the
// helper pipe. Lines beginning with "#" are comments.
const AllowedSIDFile = `C:\ProgramData\ITG Ray\Helper\allowed_sid.txt`

// ErrNoAllowList means the file does not exist (Helper not installed yet).
var ErrNoAllowList = errors.New("auth: no allow-list (helper not installed?)")
