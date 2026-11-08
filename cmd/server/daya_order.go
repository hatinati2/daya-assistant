package main

import "strings"

// Live Daya API dispatch is enabled only in the isolated Preview environment.
// Production remains fail-closed even if this code is deployed there.
func dayaOrderAPICapable() bool {
	return strings.EqualFold(env("DAYA_PREVIEW_MODE", "false"), "true")
}
