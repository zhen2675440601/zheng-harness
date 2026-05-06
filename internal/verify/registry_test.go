package verify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"zheng-harness/internal/domain"
)

func TestVerifierRegistryBuiltinsAvailableWithoutPlugins(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	if got := r.DefaultID(); got != PolicyCommandBacked {
		t.Fatalf("DefaultID() = %q, want %q", got, PolicyCommandBacked)
	}

	commandVerifier, err := r.Resolve("", "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve(default) error = %v", err)
	}
	if _, ok := commandVerifier.(*CommandVerifier); !ok {
		t.Fatalf("Resolve(default) = %T, want *CommandVerifier", commandVerifier)
	}

	evidenceVerifier, err := r.Resolve(PolicyEvidenceBased, "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve(evidence) error = %v", err)
	}
	if _, ok := evidenceVerifier.(ResearchVerifier); !ok {
		t.Fatalf("Resolve(evidence) = %T, want ResearchVerifier", evidenceVerifier)
	}

	stateVerifier, err := r.Resolve(PolicyStateOutput, "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve(state_output) error = %v", err)
	}
	if _, ok := stateVerifier.(FileWorkflowVerifier); !ok {
		t.Fatalf("Resolve(state_output) = %T, want FileWorkflowVerifier", stateVerifier)
	}
}

func TestVerifierRegistryRejectsUnknownPolicy(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	if _, err := r.Resolve("missing-policy", "standard", &stubRegistryToolExecutor{}); !errors.Is(err, ErrUnauthorizedVerificationPolicy) {
		t.Fatalf("Resolve(missing-policy) error = %v, want %v", err, ErrUnauthorizedVerificationPolicy)
	}
}

func TestVerifierRegistryRejectsPluginBindingOutsidePredeclaredPolicies(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	err := r.RegisterPlugin(stubVerifierPluginContract{policyID: "plugin-defined-policy"})
	if !errors.Is(err, ErrUnauthorizedVerificationPolicy) {
		t.Fatalf("RegisterPlugin() error = %v, want %v", err, ErrUnauthorizedVerificationPolicy)
	}
}

func TestVerifierPluginRejectsMalformedResult(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	err := r.RegisterPlugin(stubVerifierPluginContract{
		policyID: PolicyEvidenceBased,
		logicalID: "malformed-plugin",
		newVerifier: func(_ string, _ domain.ToolExecutor) domain.Verifier {
			return verifierFunc(func(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, domain.Observation) (domain.VerificationResult, error) {
				return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusFailed, Reason: "contradictory"}, nil
			})
		},
	})
	if err != nil {
		t.Fatalf("RegisterPlugin() error = %v", err)
	}

	v, err := r.Resolve(PolicyEvidenceBased, "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	result, verifyErr := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if verifyErr != nil {
		t.Fatalf("Verify() error = %v", verifyErr)
	}
	if result.Passed || result.Status != domain.VerificationStatusFailed {
		t.Fatalf("Verify() = %+v, want fail-closed failed result", result)
	}
	if !strings.Contains(result.Reason, ErrMalformedVerificationResult.Error()) {
		t.Fatalf("reason = %q, want malformed-result detail", result.Reason)
	}
	carrier, ok := v.(interface{ VerifierProvenance() *domain.PluginMetadata })
	if !ok || carrier.VerifierProvenance() == nil || carrier.VerifierProvenance().LogicalID != "malformed-plugin" {
		t.Fatalf("verifier provenance = %#v, want malformed-plugin metadata", carrier.VerifierProvenance())
	}
}

func TestVerifierPluginTimeoutFailsClosed(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	err := r.RegisterPlugin(stubVerifierPluginContract{
		policyID: PolicyEvidenceBased,
		logicalID: "timeout-plugin",
		newVerifier: func(_ string, _ domain.ToolExecutor) domain.Verifier {
			return verifierFunc(func(ctx context.Context, _ domain.Task, _ domain.Session, _ domain.Plan, _ []domain.Step, _ domain.Observation) (domain.VerificationResult, error) {
				<-ctx.Done()
				return domain.VerificationResult{}, ctx.Err()
			})
		},
	})
	if err != nil {
		t.Fatalf("RegisterPlugin() error = %v", err)
	}

	v, err := r.Resolve(PolicyEvidenceBased, "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, verifyErr := v.Verify(ctx, domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if verifyErr != nil {
		t.Fatalf("Verify() error = %v", verifyErr)
	}
	if result.Passed || result.Status != domain.VerificationStatusFailed {
		t.Fatalf("Verify() = %+v, want fail-closed failed result", result)
	}
	if !strings.Contains(strings.ToLower(result.Reason), "fail-closed") || !strings.Contains(strings.ToLower(result.Reason), "timeout-plugin") {
		t.Fatalf("reason = %q, want fail-closed timeout provenance", result.Reason)
	}
}

func TestVerifierPluginCrashFailsClosed(t *testing.T) {
	t.Parallel()

	r := NewVerifierRegistry()
	err := r.RegisterPlugin(stubVerifierPluginContract{
		policyID: PolicyEvidenceBased,
		logicalID: "panic-plugin",
		newVerifier: func(_ string, _ domain.ToolExecutor) domain.Verifier {
			return verifierFunc(func(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, domain.Observation) (domain.VerificationResult, error) {
				panic("boom")
			})
		},
	})
	if err != nil {
		t.Fatalf("RegisterPlugin() error = %v", err)
	}

	v, err := r.Resolve(PolicyEvidenceBased, "standard", &stubRegistryToolExecutor{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	result, verifyErr := v.Verify(context.Background(), domain.Task{}, domain.Session{}, domain.Plan{}, nil, domain.Observation{})
	if verifyErr != nil {
		t.Fatalf("Verify() error = %v", verifyErr)
	}
	if result.Passed || result.Status != domain.VerificationStatusFailed {
		t.Fatalf("Verify() = %+v, want fail-closed failed result", result)
	}
	if !strings.Contains(result.Reason, "crashed") || !strings.Contains(result.Reason, "panic-plugin") {
		t.Fatalf("reason = %q, want crash provenance detail", result.Reason)
	}
}

type stubRegistryToolExecutor struct{}

func (stubRegistryToolExecutor) Execute(_ context.Context, _ domain.ToolCall) (domain.ToolResult, error) {
	return domain.ToolResult{}, nil
}

type stubVerifierPluginContract struct {
	policyID    string
	logicalID   string
	newVerifier func(string, domain.ToolExecutor) domain.Verifier
}

func (s stubVerifierPluginContract) Metadata() domain.PluginMetadata {
	return domain.PluginMetadata{
		Family:          domain.PluginFamilyVerifier,
		LogicalID:       firstNonEmpty(s.logicalID, "stub-verifier"),
		DisplayName:     "Stub Verifier",
		ContractVersion: VerifierPluginContractVersion,
		ExecutionMode:   domain.PluginExecutionModeExternal,
		SourcePath:      "/plugins/" + firstNonEmpty(s.logicalID, "stub-verifier"),
	}
}

func (s stubVerifierPluginContract) PolicyID() string {
	return s.policyID
}

func (s stubVerifierPluginContract) NewVerifier(mode string, executor domain.ToolExecutor) domain.Verifier {
	if s.newVerifier != nil {
		return s.newVerifier(mode, executor)
	}
	return verifierFunc(func(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, domain.Observation) (domain.VerificationResult, error) {
		return domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}, nil
	})
}

type verifierFunc func(context.Context, domain.Task, domain.Session, domain.Plan, []domain.Step, domain.Observation) (domain.VerificationResult, error)

func (f verifierFunc) Verify(ctx context.Context, task domain.Task, session domain.Session, plan domain.Plan, steps []domain.Step, observation domain.Observation) (domain.VerificationResult, error) {
	return f(ctx, task, session, plan, steps, observation)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
