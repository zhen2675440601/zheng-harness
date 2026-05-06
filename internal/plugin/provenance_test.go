package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/store"
)

func TestPluginProvenanceRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "plugin-provenance.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() { _ = sessionStore.Close() })

	now := time.Date(2026, time.April, 30, 9, 0, 0, 0, time.UTC)
	provenance := &domain.Provenance{Plugins: []domain.PluginMetadata{
		{
			Family:                domain.PluginFamilyProvider,
			LogicalID:             "dashscope",
			DisplayName:           "DashScope Provider",
			ContractVersion:       "1.0.0",
			ImplementationVersion: "2.3.4",
			ExecutionMode:         domain.PluginExecutionModeExternal,
			SourcePath:            "plugins/providers/dashscope",
		},
		{
			Family:                domain.PluginFamilyTool,
			LogicalID:             "echo",
			DisplayName:           "Echo Tool",
			ContractVersion:       ContractVersion,
			ImplementationVersion: "1.2.3",
			ExecutionMode:         domain.PluginExecutionModeNative,
			SourcePath:            "plugins/echo.so",
		},
	}}
	if err := provenance.Validate(); err != nil {
		t.Fatalf("provenance.Validate() error = %v", err)
	}

	session := domain.Session{
		ID:         "session-plugin-provenance",
		TaskID:     "task-plugin-provenance",
		Status:     domain.SessionStatusRunning,
		Provenance: provenance,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	plan := domain.Plan{ID: "plan-plugin-provenance", TaskID: session.TaskID, Summary: "persist plugin provenance", CreatedAt: now}
	step := domain.Step{
		Index: 1,
		Action: domain.Action{
			Type:    domain.ActionTypeToolCall,
			Summary: "invoke persisted plugin-backed tool",
			ToolCall: &domain.ToolCall{
				Name:    "echo",
				Input:   `{"message":"hello"}`,
				Timeout: time.Second,
			},
		},
		Observation: domain.Observation{
			Summary:       "tool provenance captured",
			FinalResponse: "ok",
			ToolResult: &domain.ToolResult{
				ToolName: "echo",
				Output:   "hello",
				Duration: 10 * time.Millisecond,
			},
		},
		Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "captured"},
		Provenance:   provenance,
	}

	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := sessionStore.AppendStep(ctx, session.ID, step); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	resumedSession, _, resumedSteps, err := sessionStore.ResumeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if !reflect.DeepEqual(resumedSession.Provenance, provenance) {
		t.Fatalf("resumed session provenance mismatch: got %#v want %#v", resumedSession.Provenance, provenance)
	}
	if len(resumedSteps) != 1 {
		t.Fatalf("len(resumedSteps) = %d, want 1", len(resumedSteps))
	}
	if !reflect.DeepEqual(resumedSteps[0].Provenance, provenance) {
		t.Fatalf("resumed step provenance mismatch: got %#v want %#v", resumedSteps[0].Provenance, provenance)
	}
	if !reflect.DeepEqual(resumedSteps[0], step) {
		t.Fatalf("resumed step mismatch: got %#v want %#v", resumedSteps[0], step)
	}

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })

	var rawSessionMetadata string
	if err := rawDB.QueryRowContext(ctx, `SELECT config_json FROM sessions WHERE id = ?`, session.ID).Scan(&rawSessionMetadata); err != nil {
		t.Fatalf("query session config_json: %v", err)
	}
	if !strings.Contains(rawSessionMetadata, `"provenance"`) {
		t.Fatalf("config_json missing provenance payload: %s", rawSessionMetadata)
	}

	var rawStepProvenance string
	if err := rawDB.QueryRowContext(ctx, `SELECT provenance_json FROM steps WHERE session_id = ? AND step_index = ?`, session.ID, step.Index).Scan(&rawStepProvenance); err != nil {
		t.Fatalf("query step provenance_json: %v", err)
	}
	if !strings.Contains(rawStepProvenance, `"family":"tool"`) {
		t.Fatalf("step provenance_json missing tool family payload: %s", rawStepProvenance)
	}
}

func TestPluginProvenanceRejectsInvalid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "unknown family",
			payload: `{"family":"mystery","logical_id":"x","display_name":"X","contract_version":"1.0.0","execution_mode":"external"}`,
			wantErr: "unknown plugin family",
		},
		{
			name:    "malformed contract version",
			payload: `{"family":"tool","logical_id":"x","display_name":"X","contract_version":"v1","execution_mode":"external"}`,
			wantErr: "semantic version format",
		},
		{
			name:    "malformed implementation version",
			payload: `{"family":"provider","logical_id":"x","display_name":"X","contract_version":"1.0.0","implementation_version":"beta","execution_mode":"native"}`,
			wantErr: "semantic version format",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var metadata domain.PluginMetadata
			err := json.Unmarshal([]byte(tc.payload), &metadata)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("json.Unmarshal() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestPluginProvenanceRejectsInvalidProvenanceList(t *testing.T) {
	t.Parallel()

	var provenance domain.Provenance
	err := json.Unmarshal([]byte(`{"plugins":[{"family":"unknown","logical_id":"x","display_name":"X","contract_version":"1.0.0","execution_mode":"external"}]}`), &provenance)
	if err == nil || !strings.Contains(err.Error(), "unknown plugin family") {
		t.Fatalf("json.Unmarshal() error = %v, want unknown plugin family", err)
	}
}

func TestPluginProvenanceLegacySessionsRemainDecodable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy-plugin-provenance.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() { _ = sessionStore.Close() })

	now := time.Date(2026, time.April, 30, 10, 0, 0, 0, time.UTC)
	session := domain.Session{ID: "legacy-session", TaskID: "legacy-task", Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}
	plan := domain.Plan{ID: "legacy-plan", TaskID: session.TaskID, Summary: "legacy summary", CreatedAt: now}
	step := domain.Step{
		Index: 1,
		Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "legacy"},
		Observation: domain.Observation{Summary: "legacy observation", FinalResponse: "done"},
		Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "legacy ok"},
	}

	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := sessionStore.AppendStep(ctx, session.ID, step); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	resumedSession, _, resumedSteps, err := sessionStore.ResumeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if resumedSession.Provenance != nil {
		t.Fatalf("legacy resumed session provenance = %#v, want nil", resumedSession.Provenance)
	}
	if len(resumedSteps) != 1 {
		t.Fatalf("len(resumedSteps) = %d, want 1", len(resumedSteps))
	}
	if resumedSteps[0].Provenance != nil {
		t.Fatalf("legacy resumed step provenance = %#v, want nil", resumedSteps[0].Provenance)
	}
	if !reflect.DeepEqual(resumedSteps[0], step) {
		t.Fatalf("legacy resumed step mismatch: got %#v want %#v", resumedSteps[0], step)
	}
}

func TestPluginProvenanceSessionSaveMergesAdditively(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "merged-plugin-provenance.db")

	sessionStore, err := store.NewSQLiteSessionStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteSessionStore() error = %v", err)
	}
	t.Cleanup(func() { _ = sessionStore.Close() })

	now := time.Date(2026, time.April, 30, 11, 0, 0, 0, time.UTC)
	session := domain.Session{ID: "merge-session", TaskID: "merge-task", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}
	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession(initial) error = %v", err)
	}
	if err := sessionStore.SavePlan(ctx, domain.Plan{ID: "merge-plan", TaskID: session.TaskID, Summary: "merge provenance", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}

	provider := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyProvider,
		LogicalID:             "acme/provider",
		DisplayName:           "Acme Provider",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "2.0.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/provider-acme",
	}}}
	verifier := &domain.Provenance{Plugins: []domain.PluginMetadata{{
		Family:                domain.PluginFamilyVerifier,
		LogicalID:             "evidence-plugin",
		DisplayName:           "Evidence Plugin",
		ContractVersion:       "1.0.0",
		ImplementationVersion: "3.1.0",
		ExecutionMode:         domain.PluginExecutionModeExternal,
		SourcePath:            "plugins/verifier-evidence",
	}}}

	session.Provenance = provider
	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession(provider) error = %v", err)
	}
	session.Provenance = verifier
	if err := sessionStore.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession(verifier) error = %v", err)
	}

	resumedSession, _, _, err := sessionStore.ResumeSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("ResumeSession() error = %v", err)
	}
	if resumedSession.Provenance == nil || len(resumedSession.Provenance.Plugins) != 2 {
		t.Fatalf("merged provenance = %#v, want 2 plugins", resumedSession.Provenance)
	}
}
