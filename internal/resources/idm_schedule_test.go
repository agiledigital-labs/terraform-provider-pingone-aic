package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestScheduleModelPreservesKnownOptionalFields(t *testing.T) {
	isCron := false
	schedule := &client.Schedule{
		Name:              "Terraform_cleanup",
		Type:              "cron",
		InvokeService:     "script",
		IsCron:            &isCron,
		StartTime:         "2026-08-15T00:00:00Z",
		EndTime:           "2026-08-16T00:00:00Z",
		InvokeContextType: "text/javascript",
	}

	got := modelToSchedule(scheduleToModel(schedule, "cleanup", "Terraform_"))
	if got.IsCron == nil || *got.IsCron {
		t.Fatalf("isCron = %#v, want false", got.IsCron)
	}
	if got.StartTime != schedule.StartTime || got.EndTime != schedule.EndTime {
		t.Fatalf("time window = %q..%q, want %q..%q", got.StartTime, got.EndTime, schedule.StartTime, schedule.EndTime)
	}
	if got.InvokeContextType != schedule.InvokeContextType {
		t.Fatalf("invoke context type = %q, want %q", got.InvokeContextType, schedule.InvokeContextType)
	}
}
