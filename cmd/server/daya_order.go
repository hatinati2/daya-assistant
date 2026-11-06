package main

// Daya's order-submission endpoint, payload, enums, and response schema have
// not been verified from authenticated broker traffic. Keep live API dispatch
// disabled until the contract is captured and tested.
func dayaOrderAPICapable() bool {
	return false
}
