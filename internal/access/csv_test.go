package access

// Test for CSV access list

import (
	. "entry-access-control/internal/config"
	"testing"
)

func TestCSVAccessList_AddFile_ParsesCSV(t *testing.T) {

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	files, err := getLists(cfg)
	if err != nil {
		t.Fatalf("getLists failed: %v", err)
	}
	if len(files) < 1 {
		t.Fatalf("Expected one or more CSV files in %s, found none", cfg.AccessListFolder)
	}

	accessList := NewCSVAccessList()
	for _, file := range files {
		t.Logf("Testing CSV file: %s", file)
		err := accessList.AddFile(file)
		if err != nil {
			t.Errorf("AddFile failed for %s: %v", file, err)
		}
	}
}
