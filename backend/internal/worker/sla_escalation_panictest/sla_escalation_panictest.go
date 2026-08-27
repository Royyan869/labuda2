// Package slaescalationpanictest contains panic-hardening regression tests
// for the SLA escalation worker. It lives in a sibling directory rather than
// in the worker package itself because the worker package's existing
// test scaffolding (subscription_reconciliation_worker_test.go,
// reconciliation_worker_v2_test.go) has pre-existing compile errors that
// prevent the worker test binary from building. Isolating the panic-proof
// tests here lets them run without depending on that broken scaffolding.
//
// These tests pin the typed repo→worker contract that replaced the
// formerly-panic-prone map[string]interface{} contract. See the package
// comment in sla_escalation_panictest_test.go for the precise panic scenarios
// reproduced.
package slaescalationpanictest


