package services

import (
	"strings"
	"testing"
)

func TestValidateStepDef_Approval_Valid(t *testing.T) {
	step := &StepDef{
		Name: "manual-gate",
		Type: "approval",
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateStepDef_Approval_NoImageRequired(t *testing.T) {
	step := &StepDef{
		Name: "gate",
		Type: "approval",
		// Image intentionally omitted — approval doesn't need an image
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("approval step should not require image, got: %v", err)
	}
}

func TestValidateStepDef_Approval_NoCommandRequired(t *testing.T) {
	step := &StepDef{
		Name: "gate",
		Type: "approval",
		// Command intentionally omitted
	}
	if err := ValidateStepDef(step); err != nil {
		t.Errorf("approval step should not require command, got: %v", err)
	}
}

func TestResolveImage_Approval_Empty(t *testing.T) {
	step := &StepDef{
		Name: "gate",
		Type: "approval",
	}
	img := ResolveImage(step)
	if img != "" {
		t.Errorf("expected empty image for approval step, got: %q", img)
	}
}

func TestGenerateCommand_Approval_Nil(t *testing.T) {
	step := &StepDef{
		Name: "gate",
		Type: "approval",
	}
	cmd, args := GenerateCommand(step)
	if cmd != nil {
		t.Errorf("expected nil command for approval step, got: %v", cmd)
	}
	if args != nil {
		t.Errorf("expected nil args for approval step, got: %v", args)
	}
}

func TestApprovalStepTypeInRegistry(t *testing.T) {
	info, ok := GetStepTypeInfo("approval")
	if !ok {
		t.Fatal("expected approval type to be registered")
	}
	if info.DefaultImage != "" {
		t.Errorf("expected empty default image, got: %q", info.DefaultImage)
	}
	if info.RequiresCommand {
		t.Error("expected RequiresCommand=false for approval type")
	}
	if !strings.Contains(info.Description, "approval") {
		t.Errorf("expected description to mention approval, got: %q", info.Description)
	}
}

func TestListStepTypes_IncludesApproval(t *testing.T) {
	types := ListStepTypes()
	found := false
	for _, st := range types {
		if st.Name == "approval" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected approval type in ListStepTypes result")
	}
}
