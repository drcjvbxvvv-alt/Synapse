package services

import (
	"testing"
)

// ---------------------------------------------------------------------------
// parseSecretRef
// ---------------------------------------------------------------------------

func TestParseSecretRef(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantMatch bool
	}{
		{"standard format", "${{ secrets.MY_TOKEN }}", "MY_TOKEN", true},
		{"no spaces", "${{secrets.MY_TOKEN}}", "MY_TOKEN", true},
		{"extra whitespace", "  ${{  secrets.DB_PASS  }}  ", "DB_PASS", true},
		{"plain value", "hello-world", "", false},
		{"empty secret name", "${{ secrets. }}", "", false},
		{"wrong prefix", "${{ vars.FOO }}", "", false},
		{"missing closing", "${{ secrets.TOKEN", "", false},
		{"missing opening", "secrets.TOKEN }}", "", false},
		{"empty string", "", "", false},
		{"just braces", "${{}}", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotMatch := parseSecretRef(tt.input)
			if gotName != tt.wantName {
				t.Errorf("parseSecretRef(%q) name = %q, want %q", tt.input, gotName, tt.wantName)
			}
			if gotMatch != tt.wantMatch {
				t.Errorf("parseSecretRef(%q) match = %v, want %v", tt.input, gotMatch, tt.wantMatch)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// topoSortSteps
// ---------------------------------------------------------------------------

func TestTopoSortSteps_Linear(t *testing.T) {
	// A → B → C
	steps := []StepDef{
		{Name: "C", DependsOn: []string{"B"}},
		{Name: "A"},
		{Name: "B", DependsOn: []string{"A"}},
	}

	sorted, err := topoSortSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(sorted))
	}

	// Build order map
	order := map[string]int{}
	for i, s := range sorted {
		order[s.Name] = i
	}

	if order["A"] >= order["B"] {
		t.Errorf("A should come before B, got A=%d B=%d", order["A"], order["B"])
	}
	if order["B"] >= order["C"] {
		t.Errorf("B should come before C, got B=%d C=%d", order["B"], order["C"])
	}
}

func TestTopoSortSteps_Diamond(t *testing.T) {
	//   A
	//  / \
	// B   C
	//  \ /
	//   D
	steps := []StepDef{
		{Name: "D", DependsOn: []string{"B", "C"}},
		{Name: "B", DependsOn: []string{"A"}},
		{Name: "C", DependsOn: []string{"A"}},
		{Name: "A"},
	}

	sorted, err := topoSortSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(sorted))
	}

	order := map[string]int{}
	for i, s := range sorted {
		order[s.Name] = i
	}

	if order["A"] >= order["B"] {
		t.Errorf("A must precede B")
	}
	if order["A"] >= order["C"] {
		t.Errorf("A must precede C")
	}
	if order["B"] >= order["D"] {
		t.Errorf("B must precede D")
	}
	if order["C"] >= order["D"] {
		t.Errorf("C must precede D")
	}
}

func TestTopoSortSteps_NoDeps(t *testing.T) {
	steps := []StepDef{
		{Name: "Z"},
		{Name: "A"},
		{Name: "M"},
	}

	sorted, err := topoSortSteps(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(sorted))
	}
	// With no deps, Kahn's algo sorts alphabetically (stable sort)
	if sorted[0].Name != "A" || sorted[1].Name != "M" || sorted[2].Name != "Z" {
		t.Errorf("expected alphabetical order, got %s %s %s",
			sorted[0].Name, sorted[1].Name, sorted[2].Name)
	}
}

func TestTopoSortSteps_CycleDetected(t *testing.T) {
	steps := []StepDef{
		{Name: "A", DependsOn: []string{"B"}},
		{Name: "B", DependsOn: []string{"A"}},
	}

	_, err := topoSortSteps(steps)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestTopoSortSteps_UnknownDep(t *testing.T) {
	steps := []StepDef{
		{Name: "A", DependsOn: []string{"X"}},
	}

	_, err := topoSortSteps(steps)
	if err == nil {
		t.Fatal("expected unknown dep error, got nil")
	}
}

func TestTopoSortSteps_Empty(t *testing.T) {
	sorted, err := topoSortSteps(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(sorted))
	}
}
