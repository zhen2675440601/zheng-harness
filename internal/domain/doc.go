// Package domain contains the core business concepts and contracts for the
// general agent harness.
//
// Ownership: this package tree defines the stable inner model and must stay
// independent from infrastructure concerns.
//
// Guardrail: packages under internal/domain must not import infrastructure or
// interface-layer packages such as internal/store, internal/tools,
// internal/runtime, cmd/agent, or future transport/adapters. Dependencies must
// point inward only so domain logic remains portable and testable.
package domain
