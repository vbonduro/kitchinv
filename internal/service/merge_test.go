package service

import (
	"testing"

	"github.com/vbonduro/kitchinv/internal/vision"
)

func ptr4(a, b, c, d float64) *[4]float64 {
	v := [4]float64{a, b, c, d}
	return &v
}

func TestMergeDetectedItems(t *testing.T) {
	t.Run("no duplicates passthrough", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Milk", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Butter", Quantity: "1", BBox: ptr4(0.4, 0.1, 0.6, 0.3)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 2 {
			t.Fatalf("expected 2 items, got %d", len(got))
		}
		if len(got[0].bboxes) != 1 || len(got[1].bboxes) != 1 {
			t.Errorf("expected 1 bbox each, got %d and %d", len(got[0].bboxes), len(got[1].bboxes))
		}
	})

	t.Run("duplicate names merge into one with both bboxes", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Chickpeas Box", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Chickpeas Box", Quantity: "1", BBox: ptr4(0.5, 0.5, 0.7, 0.7)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(got))
		}
		if got[0].name != "Chickpeas Box" {
			t.Errorf("expected name %q, got %q", "Chickpeas Box", got[0].name)
		}
		if got[0].quantity != "2" {
			t.Errorf("expected quantity %q, got %q", "2", got[0].quantity)
		}
		if len(got[0].bboxes) != 2 {
			t.Errorf("expected 2 bboxes, got %d", len(got[0].bboxes))
		}
	})

	t.Run("case-insensitive name matching", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Whole Milk", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "whole milk", Quantity: "1", BBox: ptr4(0.4, 0.4, 0.6, 0.6)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(got))
		}
		// Preserves casing of first occurrence.
		if got[0].name != "Whole Milk" {
			t.Errorf("expected name %q, got %q", "Whole Milk", got[0].name)
		}
		if got[0].quantity != "2" {
			t.Errorf("expected quantity %q, got %q", "2", got[0].quantity)
		}
	})

	t.Run("whitespace trimmed before comparison", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "  Butter  ", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Butter", Quantity: "1", BBox: ptr4(0.4, 0.4, 0.6, 0.6)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(got))
		}
	})

	t.Run("non-integer quantity falls back to first occurrence value", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Jam", Quantity: "few", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Jam", Quantity: "few", BBox: ptr4(0.4, 0.4, 0.6, 0.6)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 item, got %d", len(got))
		}
		if got[0].quantity != "few" {
			t.Errorf("expected quantity %q, got %q", "few", got[0].quantity)
		}
	})

	t.Run("items with no bbox still merge", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Salt", Quantity: "1", BBox: nil},
			{Name: "Salt", Quantity: "1", BBox: nil},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(got))
		}
		if len(got[0].bboxes) != 0 {
			t.Errorf("expected 0 bboxes, got %d", len(got[0].bboxes))
		}
		if got[0].quantity != "2" {
			t.Errorf("expected quantity %q, got %q", "2", got[0].quantity)
		}
	})

	t.Run("item with bbox merges with item without bbox", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Pepper", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Pepper", Quantity: "1", BBox: nil},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 merged item, got %d", len(got))
		}
		if len(got[0].bboxes) != 1 {
			t.Errorf("expected 1 bbox, got %d", len(got[0].bboxes))
		}
	})

	t.Run("empty name items are skipped", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "   ", Quantity: "1", BBox: ptr4(0.4, 0.4, 0.6, 0.6)},
			{Name: "Milk", Quantity: "1", BBox: ptr4(0.7, 0.7, 0.9, 0.9)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 item (empty names skipped), got %d", len(got))
		}
		if got[0].name != "Milk" {
			t.Errorf("expected %q, got %q", "Milk", got[0].name)
		}
	})

	t.Run("insertion order preserved", func(t *testing.T) {
		input := []vision.DetectedItem{
			{Name: "Butter", Quantity: "1", BBox: ptr4(0.4, 0.1, 0.6, 0.3)},
			{Name: "Milk", Quantity: "1", BBox: ptr4(0.1, 0.1, 0.3, 0.3)},
			{Name: "Butter", Quantity: "1", BBox: ptr4(0.7, 0.1, 0.9, 0.3)},
		}
		got := mergeDetectedItems(input)
		if len(got) != 2 {
			t.Fatalf("expected 2 items, got %d", len(got))
		}
		if got[0].name != "Butter" {
			t.Errorf("expected first item %q, got %q", "Butter", got[0].name)
		}
		if got[1].name != "Milk" {
			t.Errorf("expected second item %q, got %q", "Milk", got[1].name)
		}
	})
}
