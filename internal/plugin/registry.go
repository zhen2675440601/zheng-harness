package plugin

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/llm"
	"zheng-harness/internal/verify"
)

// RegistrationSource identifies whether a registry entry is host-owned or plugin-backed.
type RegistrationSource string

const (
	RegistrationSourceBuiltin RegistrationSource = "builtin"
	RegistrationSourcePlugin  RegistrationSource = "plugin"
)

const BuiltinAgentStrategyHostDefault = "host_default"

var (
	ErrDuplicateRegistryID   = errors.New("duplicate registry id")
	ErrRegistryEntryNotFound = errors.New("registry entry not found")
	ErrVerifierContractVersionMismatch = errors.New("verifier plugin contract version mismatch")
)

// RegistrationMetadata captures host-owned provenance needed for collision checks and selection.
type RegistrationMetadata struct {
	DisplayName string
	Source      RegistrationSource
	Path        string
}

// AgentStrategy is the dedicated contract family for future host-controlled runtime strategies.
type AgentStrategy interface {
	StrategyID() string
}

// AgentStrategyFactory constructs a host-controlled agent strategy selection.
type AgentStrategyFactory func() (AgentStrategy, error)

// ProviderDescriptor stores one provider entry inside the provider registry.
type ProviderDescriptor struct {
	ID       string
	Metadata RegistrationMetadata
	Factory  llm.ProviderFactory
}

type VerifierRegistration struct {
	ID       string
	Metadata RegistrationMetadata
}

// AgentStrategyDescriptor stores one agent strategy entry inside the agent strategy registry.
type AgentStrategyDescriptor struct {
	ID       string
	Metadata RegistrationMetadata
	Factory  AgentStrategyFactory
}

// ProviderRegistry is the host-owned namespace for provider implementations.
// Selection precedence is deterministic: an explicit ID wins; otherwise the built-in default is used.
type ProviderRegistry struct {
	mu        sync.RWMutex
	entries   map[string]ProviderDescriptor
	defaultID string
}

// VerifierRegistry is the host-owned namespace for verifier policies.
// Selection precedence is deterministic: an explicit ID wins; otherwise the built-in default is used.
type VerifierRegistry struct {
	mu       sync.RWMutex
	entries  map[string]verifierRegistration
	registry *verify.VerifierRegistry
}

type verifierRegistration struct {
	Metadata RegistrationMetadata
}

// AgentStrategyRegistry is the host-owned namespace for runtime strategy selection.
// Selection precedence is deterministic: an explicit ID wins; otherwise the built-in default is used.
type AgentStrategyRegistry struct {
	mu        sync.RWMutex
	entries   map[string]AgentStrategyDescriptor
	defaultID string
}

// NewProviderRegistry constructs a provider registry with built-ins always registered.
func NewProviderRegistry() *ProviderRegistry {
	r := &ProviderRegistry{entries: make(map[string]ProviderDescriptor), defaultID: config.ProviderOpenAI}
	r.mustRegisterBuiltin(config.ProviderOpenAI, RegistrationMetadata{DisplayName: "OpenAI", Source: RegistrationSourceBuiltin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		return llm.AdaptProviderPlugin(config.ProviderOpenAI, llm.NewOpenAIProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel())), nil
	})
	r.mustRegisterBuiltin(config.ProviderAnthropic, RegistrationMetadata{DisplayName: "Anthropic", Source: RegistrationSourceBuiltin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return llm.AdaptProviderPlugin(config.ProviderAnthropic, llm.NewAnthropicProvider(cfg.GetAPIKey(), baseURL, cfg.GetModel())), nil
	})
	r.mustRegisterBuiltin(config.ProviderDashScope, RegistrationMetadata{DisplayName: "DashScope", Source: RegistrationSourceBuiltin}, func(cfg llm.ProviderConfig) (llm.ProviderPluginContract, error) {
		baseURL := cfg.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://coding.dashscope.aliyuncs.com/apps/anthropic/v1"
		}
		provider := llm.NewDashScopeProvider(cfg.GetModel(), baseURL, cfg.GetAPIKey())
		return llm.AdaptProviderPlugin(config.ProviderDashScope, provider), nil
	})
	return r
}

// NewVerifierRegistry constructs a verifier registry with built-ins always registered.
func NewVerifierRegistry() *VerifierRegistry {
	r := &VerifierRegistry{entries: make(map[string]verifierRegistration), registry: verify.NewVerifierRegistry()}
	r.mustRegisterBuiltin(verify.PolicyCommandBacked, RegistrationMetadata{DisplayName: "Command Verifier", Source: RegistrationSourceBuiltin})
	r.mustRegisterBuiltin(verify.PolicyEvidenceBased, RegistrationMetadata{DisplayName: "Research Verifier", Source: RegistrationSourceBuiltin})
	r.mustRegisterBuiltin(verify.PolicyStateOutput, RegistrationMetadata{DisplayName: "File Workflow Verifier", Source: RegistrationSourceBuiltin})
	return r
}

// NewAgentStrategyRegistry constructs an agent strategy registry with built-ins always registered.
func NewAgentStrategyRegistry() *AgentStrategyRegistry {
	r := &AgentStrategyRegistry{entries: make(map[string]AgentStrategyDescriptor), defaultID: BuiltinAgentStrategyHostDefault}
	r.mustRegisterBuiltin(BuiltinAgentStrategyHostDefault, RegistrationMetadata{DisplayName: "Host Default Strategy", Source: RegistrationSourceBuiltin}, func() (AgentStrategy, error) {
		return staticAgentStrategy{id: BuiltinAgentStrategyHostDefault}, nil
	})
	return r
}

// Register adds a provider entry into the shared provider namespace.
func (r *ProviderRegistry) Register(id string, metadata RegistrationMetadata, factory llm.ProviderFactory) error {
	normalizedID, normalizedMetadata, err := normalizeRegistration(id, metadata)
	if err != nil {
		return err
	}
	if factory == nil {
		return errors.New("provider factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ensureNoProviderCollision(r.entries, normalizedID, normalizedMetadata); err != nil {
		return err
	}
	r.entries[normalizedID] = ProviderDescriptor{ID: normalizedID, Metadata: normalizedMetadata, Factory: factory}
	return nil
}

// Get returns a provider registration by its logical ID.
func (r *ProviderRegistry) Get(id string) (ProviderDescriptor, bool) {
	normalizedID := normalizeID(id)
	if normalizedID == "" || r == nil {
		return ProviderDescriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.entries[normalizedID]
	return descriptor, ok
}

// Resolve applies host selection precedence and constructs the selected provider.
func (r *ProviderRegistry) Resolve(id string, cfg llm.ProviderConfig) (llm.Provider, error) {
	descriptor, err := r.resolveDescriptor(id)
	if err != nil {
		return nil, err
	}
	return descriptor.Factory(cfg)
}

// DefaultID returns the host-owned default provider selection.
func (r *ProviderRegistry) DefaultID() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultID
}

// Register adds a verifier entry into the shared verifier namespace.
func (r *VerifierRegistry) Register(id string, metadata RegistrationMetadata, factory verify.VerifierFactory) error {
	normalizedID, normalizedMetadata, err := normalizeRegistration(id, metadata)
	if err != nil {
		return err
	}
	if factory == nil {
		return errors.New("verifier factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ensureNoVerifierCollision(r.entries, normalizedID, normalizedMetadata); err != nil {
		return err
	}
	if err := r.registry.Register(normalizedID, factory); err != nil {
		return err
	}
	r.entries[normalizedID] = verifierRegistration{Metadata: normalizedMetadata}
	return nil
}

// RegisterPluginContract validates metadata and contract version before a verifier plugin is allowed to bind a predeclared policy.
func (r *VerifierRegistry) RegisterPluginContract(plugin verify.VerifierPluginContract) error {
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
	if metadata.ContractVersion != verify.VerifierPluginContractVersion {
		return fmt.Errorf("%w: plugin=%q expected=%q", ErrVerifierContractVersionMismatch, metadata.ContractVersion, verify.VerifierPluginContractVersion)
	}
	return r.registry.RegisterPlugin(plugin)
}

// Get returns a verifier registration by its logical ID.
func (r *VerifierRegistry) Get(id string) (verify.VerifierDescriptor, bool) {
	normalizedID := normalizeID(id)
	if normalizedID == "" || r == nil {
		return verify.VerifierDescriptor{}, false
	}
	return r.registry.Get(normalizedID)
}

func (r *VerifierRegistry) GetRegistration(id string) (VerifierRegistration, bool) {
	normalizedID := normalizeID(id)
	if normalizedID == "" || r == nil {
		return VerifierRegistration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[normalizedID]
	if !ok {
		return VerifierRegistration{}, false
	}
	return VerifierRegistration{ID: normalizedID, Metadata: entry.Metadata}, true
}

// Resolve applies host selection precedence and constructs the selected verifier.
func (r *VerifierRegistry) Resolve(id, mode string, executor domain.ToolExecutor) (domain.Verifier, error) {
	if r == nil || r.registry == nil {
		return nil, fmt.Errorf("%w: verifier registry is nil", ErrRegistryEntryNotFound)
	}
	verifierStrategy, err := r.registry.Resolve(id, mode, executor)
	if err != nil {
		if errors.Is(err, verify.ErrUnknownVerificationPolicy) || errors.Is(err, verify.ErrUnauthorizedVerificationPolicy) {
			return nil, fmt.Errorf("%w: verifier %q", ErrRegistryEntryNotFound, normalizeID(id))
		}
		return nil, err
	}
	return verifierStrategy, nil
}

// DefaultID returns the host-owned default verifier selection.
func (r *VerifierRegistry) DefaultID() string {
	if r == nil || r.registry == nil {
		return ""
	}
	return r.registry.DefaultID()
}

// Register adds an agent strategy entry into the shared agent strategy namespace.
func (r *AgentStrategyRegistry) Register(id string, metadata RegistrationMetadata, factory AgentStrategyFactory) error {
	normalizedID, normalizedMetadata, err := normalizeRegistration(id, metadata)
	if err != nil {
		return err
	}
	if factory == nil {
		return errors.New("agent strategy factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ensureNoAgentStrategyCollision(r.entries, normalizedID, normalizedMetadata); err != nil {
		return err
	}
	r.entries[normalizedID] = AgentStrategyDescriptor{ID: normalizedID, Metadata: normalizedMetadata, Factory: factory}
	return nil
}

// Get returns an agent strategy registration by its logical ID.
func (r *AgentStrategyRegistry) Get(id string) (AgentStrategyDescriptor, bool) {
	normalizedID := normalizeID(id)
	if normalizedID == "" || r == nil {
		return AgentStrategyDescriptor{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	descriptor, ok := r.entries[normalizedID]
	return descriptor, ok
}

// Resolve applies host selection precedence and constructs the selected strategy.
func (r *AgentStrategyRegistry) Resolve(id string) (AgentStrategy, error) {
	descriptor, err := r.resolveDescriptor(id)
	if err != nil {
		return nil, err
	}
	return descriptor.Factory()
}

// DefaultID returns the host-owned default agent strategy selection.
func (r *AgentStrategyRegistry) DefaultID() string {
	if r == nil {
		return ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultID
}

func (r *ProviderRegistry) resolveDescriptor(id string) (ProviderDescriptor, error) {
	if r == nil {
		return ProviderDescriptor{}, fmt.Errorf("%w: provider registry is nil", ErrRegistryEntryNotFound)
	}
	selectedID := normalizeID(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if selectedID == "" {
		selectedID = r.defaultID
	}
	descriptor, ok := r.entries[selectedID]
	if !ok {
		return ProviderDescriptor{}, fmt.Errorf("%w: provider %q", ErrRegistryEntryNotFound, selectedID)
	}
	return descriptor, nil
}

func (r *AgentStrategyRegistry) resolveDescriptor(id string) (AgentStrategyDescriptor, error) {
	if r == nil {
		return AgentStrategyDescriptor{}, fmt.Errorf("%w: agent strategy registry is nil", ErrRegistryEntryNotFound)
	}
	selectedID := normalizeID(id)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if selectedID == "" {
		selectedID = r.defaultID
	}
	descriptor, ok := r.entries[selectedID]
	if !ok {
		return AgentStrategyDescriptor{}, fmt.Errorf("%w: agent strategy %q", ErrRegistryEntryNotFound, selectedID)
	}
	return descriptor, nil
}

func (r *ProviderRegistry) mustRegisterBuiltin(id string, metadata RegistrationMetadata, factory llm.ProviderFactory) {
	if err := r.Register(id, metadata, factory); err != nil {
		panic(err)
	}
}

func (r *VerifierRegistry) mustRegisterBuiltin(id string, metadata RegistrationMetadata) {
	normalizedID, normalizedMetadata, err := normalizeRegistration(id, metadata)
	if err != nil {
		panic(err)
	}
	r.entries[normalizedID] = verifierRegistration{Metadata: normalizedMetadata}
	if _, ok := r.registry.Get(normalizedID); !ok {
		panic(fmt.Errorf("builtin verifier policy %q missing from verify registry", normalizedID))
	}
}

func (r *AgentStrategyRegistry) mustRegisterBuiltin(id string, metadata RegistrationMetadata, factory AgentStrategyFactory) {
	if err := r.Register(id, metadata, factory); err != nil {
		panic(err)
	}
}

func normalizeRegistration(id string, metadata RegistrationMetadata) (string, RegistrationMetadata, error) {
	normalizedID := normalizeID(id)
	if normalizedID == "" {
		return "", RegistrationMetadata{}, errors.New("registry id must not be empty")
	}
	metadata.DisplayName = strings.TrimSpace(metadata.DisplayName)
	metadata.Path = strings.TrimSpace(metadata.Path)
	if metadata.Source == "" {
		metadata.Source = RegistrationSourcePlugin
	}
	switch metadata.Source {
	case RegistrationSourceBuiltin, RegistrationSourcePlugin:
		return normalizedID, metadata, nil
	default:
		return "", RegistrationMetadata{}, fmt.Errorf("unsupported registration source %q", metadata.Source)
	}
}

func normalizeID(id string) string {
	return strings.TrimSpace(id)
}

func ensureNoProviderCollision(entries map[string]ProviderDescriptor, id string, metadata RegistrationMetadata) error {
	if existing, ok := entries[id]; ok {
		return collisionError("provider", id, existing.Metadata, metadata)
	}
	return nil
}

func ensureNoVerifierCollision(entries map[string]verifierRegistration, id string, metadata RegistrationMetadata) error {
	if existing, ok := entries[id]; ok {
		return collisionError("verifier", id, existing.Metadata, metadata)
	}
	return nil
}

func ensureNoAgentStrategyCollision(entries map[string]AgentStrategyDescriptor, id string, metadata RegistrationMetadata) error {
	if existing, ok := entries[id]; ok {
		return collisionError("agent strategy", id, existing.Metadata, metadata)
	}
	return nil
}

func collisionError(family, id string, existing, incoming RegistrationMetadata) error {
	return fmt.Errorf("%w: %s %q already registered by %s (%s), cannot replace with %s (%s)", ErrDuplicateRegistryID, family, id, metadataDisplayName(existing), existing.Source, metadataDisplayName(incoming), incoming.Source)
}

func metadataDisplayName(metadata RegistrationMetadata) string {
	if metadata.DisplayName != "" {
		return metadata.DisplayName
	}
	if metadata.Path != "" {
		return metadata.Path
	}
	return string(metadata.Source)
}

type staticAgentStrategy struct {
	id string
}

func (s staticAgentStrategy) StrategyID() string {
	return s.id
}

func sortedProviderIDs(entries map[string]ProviderDescriptor) []string {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedVerifierIDs(entries map[string]verifierRegistration) []string {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedAgentStrategyIDs(entries map[string]AgentStrategyDescriptor) []string {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
