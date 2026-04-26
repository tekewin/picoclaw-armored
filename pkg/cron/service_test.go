package cron

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveStore_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not enforced on Windows")
	}

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "cron", "jobs.json")

	cs := NewCronService(storePath, nil)

	_, err := cs.AddJob("test", CronSchedule{Kind: "every", EveryMS: int64Ptr(60000)}, "hello", "", false, "cli", "direct")
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("cron store has permission %04o, want 0600", perm)
	}
}

func TestAddJob_DeduplicateEvery(t *testing.T) {
	tmpDir := t.TempDir()
	cs := NewCronService(filepath.Join(tmpDir, "jobs.json"), nil)

	sched := CronSchedule{Kind: "every", EveryMS: int64Ptr(3600000)}
	_, err := cs.AddJob("hourly", sched, "ping", "", false, "cli", "direct")
	if err != nil {
		t.Fatalf("first AddJob failed: %v", err)
	}
	_, err = cs.AddJob("hourly", sched, "ping", "", false, "cli", "direct")
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func TestAddJob_DeduplicateCron(t *testing.T) {
	tmpDir := t.TempDir()
	cs := NewCronService(filepath.Join(tmpDir, "jobs.json"), nil)

	sched := CronSchedule{Kind: "cron", Expr: "0 9 * * *"}
	_, err := cs.AddJob("daily", sched, "good morning", "", false, "cli", "direct")
	if err != nil {
		t.Fatalf("first AddJob failed: %v", err)
	}
	_, err = cs.AddJob("daily", sched, "good morning", "", false, "cli", "direct")
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func TestAddJob_DeduplicateAt(t *testing.T) {
	tmpDir := t.TempDir()
	cs := NewCronService(filepath.Join(tmpDir, "jobs.json"), nil)

	atMS := int64(9999999999999)
	sched := CronSchedule{Kind: "at", AtMS: &atMS}
	_, err := cs.AddJob("once", sched, "reminder", "", true, "cli", "direct")
	if err != nil {
		t.Fatalf("first AddJob failed: %v", err)
	}
	_, err = cs.AddJob("once", sched, "reminder", "", true, "cli", "direct")
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func TestAddJob_DifferentScheduleAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	cs := NewCronService(filepath.Join(tmpDir, "jobs.json"), nil)

	_, err := cs.AddJob("a", CronSchedule{Kind: "every", EveryMS: int64Ptr(3600000)}, "ping", "", false, "cli", "direct")
	if err != nil {
		t.Fatalf("first AddJob failed: %v", err)
	}
	_, err = cs.AddJob("b", CronSchedule{Kind: "every", EveryMS: int64Ptr(7200000)}, "ping", "", false, "cli", "direct")
	if err != nil {
		t.Fatalf("different interval should be allowed: %v", err)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
