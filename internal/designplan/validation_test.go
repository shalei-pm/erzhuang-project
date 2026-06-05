package designplan

import (
	"errors"
	"testing"
)

func TestValidateStoreInputRequiresCoreFields(t *testing.T) {
	err := ValidateStoreInput(StoreInput{})
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}

	if validationError.Fields["name"] == "" {
		t.Fatal("expected name validation error")
	}
	if validationError.Fields["areas"] == "" {
		t.Fatal("expected areas validation error")
	}
}

func TestValidateStoreInputRequiresTreatmentAndConsultationNumbers(t *testing.T) {
	input := validStoreInput()
	input.Areas[0].Number = ""

	err := ValidateStoreInput(input)
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["areas[0].number"] == "" {
		t.Fatal("expected number validation error")
	}
}

func TestValidateStoreInputRejectsNonDigitNumber(t *testing.T) {
	input := validStoreInput()
	input.Areas[0].Number = "A1"

	err := ValidateStoreInput(input)
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["areas[0].number"] != "编号只能是数字" {
		t.Fatalf("unexpected number error: %q", validationError.Fields["areas[0].number"])
	}
}

func TestValidateStoreInputRejectsDuplicateNumberInSameType(t *testing.T) {
	input := validStoreInput()
	input.Areas = append(input.Areas, AreaInput{
		Name:   "治疗室 1 复用",
		Type:   AreaTypeTreatment,
		Number: "1",
		Box:    &Box{X: 0.5, Y: 0.1, Width: 0.2, Height: 0.2},
	})

	err := ValidateStoreInput(input)
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if validationError.Fields["areas[2].number"] != "同门店同类型编号必须唯一" {
		t.Fatalf("unexpected duplicate error: %q", validationError.Fields["areas[2].number"])
	}
}

func TestValidateStoreInputAllowsSameNumberAcrossDifferentTypes(t *testing.T) {
	input := validStoreInput()
	input.Areas[1].Number = "1"

	if err := ValidateStoreInput(input); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func validStoreInput() StoreInput {
	return StoreInput{
		Name: "杭州西湖店",
		Areas: []AreaInput{
			{
				Name:   "治疗室 1",
				Type:   AreaTypeTreatment,
				Number: "1",
				Box:    &Box{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2},
			},
			{
				Name:   "面诊室 2",
				Type:   AreaTypeConsultation,
				Number: "2",
				Box:    &Box{X: 0.4, Y: 0.1, Width: 0.2, Height: 0.2},
			},
		},
	}
}
