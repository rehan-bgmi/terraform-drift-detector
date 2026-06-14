package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/driftctl/driftctl/internal/model"
)

func TestOpenSQLite(t *testing.T) {
	// Create temporary database file
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestSaveAndGetWorkspace(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	ws := &model.Workspace{
		Name:     "test-workspace",
		Provider: "aws",
		State: model.StateConfig{
			Backend: "local",
			Path:    "/path/to/state",
		},
		Regions: []string{"us-east-1", "us-west-2"},
	}

	// Save workspace
	err = store.SaveWorkspace(ctx, ws)
	if err != nil {
		t.Fatalf("failed to save workspace: %v", err)
	}

	// Get workspace by name
	retrieved, err := store.GetWorkspaceByName(ctx, "test-workspace")
	if err != nil {
		t.Fatalf("failed to get workspace: %v", err)
	}

	if retrieved.Name != "test-workspace" {
		t.Errorf("expected name 'test-workspace', got %q", retrieved.Name)
	}
	if retrieved.Provider != "aws" {
		t.Errorf("expected provider 'aws', got %q", retrieved.Provider)
	}
}

func TestSaveAndGetScan(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	report := &model.DriftReport{
		ScanID:      "scan-123",
		WorkspaceID: "ws-123",
		Workspace:   "test",
		StartedAt:   time.Now().UTC(),
		Status:      model.ScanStatusCompleted,
		Summary: model.DriftSummary{
			TotalResources: 10,
			TotalFindings:  2,
			MissingInCloud: 1,
			ExtraInCloud:   1,
		},
		Findings: []model.DriftFinding{
			{
				Kind:         model.DriftMissingInCloud,
				ResourceID:   "aws/instance/i-123",
				ResourceType: "aws_instance",
				Severity:     model.SeverityCritical,
			},
		},
	}

	// Save scan
	err = store.SaveScan(ctx, report)
	if err != nil {
		t.Fatalf("failed to save scan: %v", err)
	}

	// Get scan
	retrieved, err := store.GetScan(ctx, "scan-123")
	if err != nil {
		t.Fatalf("failed to get scan: %v", err)
	}

	if retrieved.ScanID != "scan-123" {
		t.Errorf("expected scan ID 'scan-123', got %q", retrieved.ScanID)
	}
	if retrieved.Summary.TotalFindings != 2 {
		t.Errorf("expected 2 findings, got %d", retrieved.Summary.TotalFindings)
	}
}

func TestListScans(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Save multiple scans
	for i := 0; i < 5; i++ {
		report := &model.DriftReport{
			ScanID:      string(rune(i)),
			WorkspaceID: "ws-123",
			Workspace:   "test",
			StartedAt:   time.Now().UTC(),
			Status:      model.ScanStatusCompleted,
		}
		err = store.SaveScan(ctx, report)
		if err != nil {
			t.Fatalf("failed to save scan: %v", err)
		}
	}

	// List scans
	scans, err := store.ListScans(ctx, "ws-123", 10)
	if err != nil {
		t.Fatalf("failed to list scans: %v", err)
	}

	if len(scans) != 5 {
		t.Errorf("expected 5 scans, got %d", len(scans))
	}
}

func BenchmarkSaveScan(b *testing.B) {
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		b.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	report := &model.DriftReport{
		ScanID:      "scan-123",
		WorkspaceID: "ws-123",
		Workspace:   "test",
		StartedAt:   time.Now().UTC(),
		Status:      model.ScanStatusCompleted,
		Summary: model.DriftSummary{
			TotalResources: 100,
			TotalFindings:  10,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.SaveScan(ctx, report)
	}
}

func BenchmarkListScans(b *testing.B) {
	tempFile, err := os.CreateTemp("", "test_driftctl_*.db")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	store, err := OpenSQLite(tempFile.Name())
	if err != nil {
		b.Fatalf("failed to open SQLite: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Pre-populate with scans
	for i := 0; i < 100; i++ {
		report := &model.DriftReport{
			ScanID:      string(rune(i)),
			WorkspaceID: "ws-123",
			Workspace:   "test",
			StartedAt:   time.Now().UTC(),
			Status:      model.ScanStatusCompleted,
		}
		_ = store.SaveScan(ctx, report)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.ListScans(ctx, "ws-123", 50)
	}
}
