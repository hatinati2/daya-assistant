package main

// A POST /orders request and ENTRY / SENT_TO_TRADING_ENGINE acknowledgement
// are now observed. Required authentication details and the final BOOKED
// confirmation/reconciliation source remain unverified, so live dispatch
// stays disabled until those parts are captured and reviewed.
func dayaOrderAPICapable() bool {
	return false
}
