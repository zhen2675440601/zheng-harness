package verify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"zheng-harness/internal/domain"
)

const VerifierPluginContractVersion = "1.0.0"

var (
	ErrUnknownVerificationPolicy      = errors.New("unknown verification policy")
	ErrUnauthorizedVerificationPolicy = errors.New("unauthorized verification policy binding")
	ErrDuplicateVerificationPolicy    = errors.New("duplicate verification policy")
	ErrMalformedVerificationResult    = errors.New("malformed verification result")
)

// VerifierFactory constructs a verifier strategy for one host-predeclared policy.
type VerifierFactory func(mode string, executor domain.ToolExecutor) domain.Verifier

// VerifierDescriptor stores one verifier policy entry inside the host-owned registry.
type VerifierDescriptor struct {
	ID      string
	Factory VerifierFactory
}

// VerifierPluginContract binds a plugin implementation to one host-predeclared policy ID.
// The host remains responsible for validating plugin metadata and verification result schema.
type VerifierPluginContract interface {
	Metadata() domain.PluginMetadata
	PolicyID() string
	NewVerifier(mode string, executor domain.ToolExecutor) domain.Verifier
}

// VerifierRegistry is the host-owned namespace for verifier policy selection.
// Only host-predeclared policy IDs can be registered.
type VerifierRegistry struct {
	mu              sync.RWMutex
	predeclared     map[string]struct{}
	builtins        map[string]VerifierDescriptor
	plugins         map[string]VerifierDescriptor
	defaultPolicyID string
}

// NewVerifierRegistry constructs a verifier registry with built-in host policies registered.
func NewVerifierRegistry() *VerifierRegistry {
	r := NewVerifierRegistryWithPolicies([]string{PolicyCommandBacked, PolicyEvidenceBased, PolicyStateOutput}, PolicyCommandBacked)
	r.mustRegisterBuiltin(PolicyCommandBacked, func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return NewCommandVerifier(executor)
	})
	r.mustRegisterBuiltin(PolicyEvidenceBased, func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return ResearchVerifier{}
	})
	r.mustRegisterBuiltin(PolicyStateOutput, func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return FileWorkflowVerifier{}
	})
	return r
}

// NewVerifierRegistryWithPolicies constructs a verifier registry constrained to host-predeclared policies.
func NewVerifierRegistryWithPolicies(predeclared []string, defaultPolicyID string) *VerifierRegistry {
	allowed := make(map[string]struct{}, len(predeclared))
	for _, policy := range predeclared {
		normalized, ok := canonicalVerificationPolicyID(policy)
		if !ok {
			panic(fmt.Sprintf("verify: unsupported predeclared policy %q", policy))
		}
		allowed[normalized] = struct{}{}
	}
	if len(allowed) == 0 {
		panic("verify: verifier registry requires at least one predeclared policy")
	}
	normalizedDefault, ok := canonicalVerificationPolicyID(defaultPolicyID)
	if !ok {
		panic(fmt.Sprintf("verify: unsupported default verifier policy %q", defaultPolicyID))
	}
	if _, exists := allowed[normalizedDefault]; !exists {
		panic(fmt.Sprintf("verify: default verifier policy %q is not predeclared", defaultPolicyID))
	}
	return &VerifierRegistry{
		predeclared:     allowed,
		builtins:        make(map[string]VerifierDescriptor),
		plugins:         make(map[string]VerifierDescriptor),
		defaultPolicyID: normalizedDefault,
	}
}

// Register adds a verifier policy entry into the shared host namespace.
func (r *VerifierRegistry) Register(id string, factory VerifierFactory) error {
	normalizedID, err := r.normalizeRegisteredPolicy(id)
	if err != nil {
		return err
	}
	if factory == nil {
		return errors.New("verifier factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.builtins[normalizedID]; ok {
		return fmt.Errorf("%w: verifier policy %q already registered", ErrDuplicateVerificationPolicy, existing.ID)
	}
	r.builtins[normalizedID] = VerifierDescriptor{ID: normalizedID, Factory: factory}
	return nil
}

// RegisterPlugin adds one plugin-backed verifier bound to a host-predeclared policy ID.
func (r *VerifierRegistry) RegisterPlugin(plugin VerifierPluginContract) error {
	if plugin == nil {
		return errors.New("verifier plugin contract must not be nil")
	}
	metadata := plugin.Metadata().Normalize()
	if err := metadata.Validate(); err != nil {
		return err
	}
	if metadata.Family != domain.PluginFamilyVerifier {
		return fmt.Errorf("verifier plugin metadata family must be %q, got %q", domain.PluginFamilyVerifier, metadata.Family)
	}
	policyID, err := r.normalizeRegisteredPolicy(plugin.PolicyID())
	if err != nil {
		return err
	}
	wrapped := func(mode string, executor domain.ToolExecutor) domain.Verifier {
		return &pluginVerifier{
			delegate: plugin.NewVerifier(mode, executor),
			metadata: metadata,
			policyID: policyID,
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.plugins[policyID]; ok {
		return fmt.Errorf("%w: verifier policy %q already registered", ErrDuplicateVerificationPolicy, existing.ID)
	}
	r.plugins[policyID] = VerifierDescriptor{ID: policyID, Factory: wrapped}
	return nil
}

// Get returns a verifier registration by its logical policy ID.
func (r *VerifierRegistry) Get(id string) (VerifierDescriptor, bool) {
	normalizedID, ok := canonicalVerificationPolicyID(id)
	if !ok || r == nil {
		return VerifierDescriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, exists := r.plugins[normalizedID]
	if exists {
		return descriptor, true
	}
	descriptor, exists = r.builtins[normalizedID]
	return descriptor, exists
}

// Resolve applies host selection precedence and constructs the selected verifier.
func (r *VerifierRegistry) Resolve(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error) {
	descriptor, err := r.resolveDescriptor(id)
	if err != nil {
		return nil, err
	}
	return descriptor.Factory(mode, executor), nil
}

// DefaultID returns the host-owned default verifier policy selection.
func (r *VerifierRegistry) DefaultID() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultPolicyID
}

func (r *VerifierRegistry) resolveDescriptor(id string) (VerifierDescriptor, error) {
	if r == nil {
		return VerifierDescriptor{}, fmt.Errorf("%w: verifier registry is nil", ErrUnknownVerificationPolicy)
	}
	selectedID := strings.TrimSpace(id)
	if selectedID == "" {
		selectedID = r.DefaultID()
	}
	normalizedID, err := r.normalizeRegisteredPolicy(selectedID)
	if err != nil {
		return VerifierDescriptor{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.plugins[normalizedID]
	if ok {
		return descriptor, nil
	}
	descriptor, ok = r.builtins[normalizedID]
	if !ok {
		return VerifierDescriptor{}, fmt.Errorf("%w: verifier policy %q not configured", ErrUnknownVerificationPolicy, normalizedID)
	}
	return descriptor, nil
}

type verifierProvenanceCarrier interface {
	VerifierProvenance() *domain.PluginMetadata
}

type pluginVerifier struct {
	delegate domain.Verifier
	metadata domain.PluginMetadata
	policyID string
}

func (v *pluginVerifier) Verify(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) (result domain.VerificationResult, err error) {
	if v == nil || v.delegate == nil {
		return v.failClosed("plugin verifier is not available"), nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = v.failClosed(fmt.Sprintf("plugin verifier crashed: %v", recovered))
			err = nil
		}
	}()
	result, err = v.delegate.Verify(ctx, task, session, plan, steps, observation)
	if err != nil {
		return v.failClosed(fmt.Sprintf("plugin verifier execution failed: %v", err)), nil
	}
	result = result.Normalize()
	if validationErr := result.Validate(); validationErr != nil {
		return v.failClosed(fmt.Sprintf("%v: %v", ErrMalformedVerificationResult, validationErr)), nil
	}
	return result, nil
}

func (v *pluginVerifier) VerifierProvenance() *domain.PluginMetadata {
	metadata := v.metadata.Normalize()
	return &metadata
}

func (v *pluginVerifier) failClosed(message string) domain.VerificationResult {
	return domain.VerificationResult{
		Passed: false,
		Status: domain.VerificationStatusFailed,
		Reason: fmt.Sprintf("verification policy %q fail-closed via plugin %s (%s): %s", v.policyID, v.metadata.LogicalID, v.metadata.SourcePath, strings.TrimSpace(message)),
	}
}

func (r *VerifierRegistry) mustRegisterBuiltin(id string, factory VerifierFactory) {
	if err := r.Register(id, factory); err != nil {
		panic(err)
	}
}

func (r *VerifierRegistry) normalizeRegisteredPolicy(id string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("%w: verifier registry is nil", ErrUnknownVerificationPolicy)
	}
	normalizedID, ok := canonicalVerificationPolicyID(id)
	if !ok {
		return "", fmt.Errorf("%w: verifier policy %q", ErrUnauthorizedVerificationPolicy, strings.TrimSpace(id))
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.predeclared[normalizedID]; !exists {
		return "", fmt.Errorf("%w: verifier policy %q", ErrUnauthorizedVerificationPolicy, normalizedID)
	}
	return normalizedID, nil
}

func canonicalVerificationPolicyID(raw string) (string, bool) {
	normalized := normalizeVerificationPolicy(raw)
	if normalized == "" {
		return "", false
	}
	return normalized, true
}
