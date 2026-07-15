package core

import (
	"testing"

	gocore "github.com/synology-community/go-synology/pkg/api/core"
)

func TestFindTaskResultByName(t *testing.T) {
	id := int64(42)
	task, err := findTaskResultByName([]gocore.TaskResult{
		{Name: "other"},
		{Name: "wanted", ID: &id},
	}, "wanted")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if task == nil || task.Name != "wanted" || task.ID == nil || *task.ID != 42 {
		t.Fatalf("unexpected task match: %#v", task)
	}
}

func TestFindTaskResultByNameRejectsDuplicates(t *testing.T) {
	id1 := int64(1)
	id2 := int64(2)
	if _, err := findTaskResultByName([]gocore.TaskResult{
		{Name: "dup", ID: &id1},
		{Name: "dup", ID: &id2},
	}, "dup"); err == nil {
		t.Fatal("expected duplicate task name to be rejected")
	}
}

func TestFindTaskResultByNameRequiresID(t *testing.T) {
	if _, err := findTaskResultByName([]gocore.TaskResult{
		{Name: "broken"},
	}, "broken"); err == nil {
		t.Fatal("expected missing task ID to be rejected")
	}
}
