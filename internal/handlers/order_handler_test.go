package handlers

import "testing"

func TestCalculateInteriorDoorPriceAllowsMissingSecondLeafPrice(t *testing.T) {
	count2 := 1
	item := interiorDoorRequest{
		Price:    100,
		Count:    2,
		LeafType: "Double",
		Count2:   &count2,
	}

	got := calculateInteriorDoorPrice(item)
	if got != 200 {
		t.Fatalf("calculateInteriorDoorPrice() = %v, want 200", got)
	}
}

func TestMapInteriorDoorsForCreateAllowsMissingSecondLeafPrice(t *testing.T) {
	width2 := 40
	height2 := 200
	count2 := 1
	items := mapInteriorDoorsForCreate([]interiorDoorRequest{{
		Model:    "Door",
		Color:    "White",
		Price:    100,
		Width:    80,
		Width2:   &width2,
		Height:   200,
		Height2:  &height2,
		LeafType: "Double",
		Count:    1,
		Count2:   &count2,
		Covering: "PVC",
	}})

	if len(items) != 1 {
		t.Fatalf("mapInteriorDoorsForCreate() returned %d items, want 1", len(items))
	}
	if items[0].Price2 != nil {
		t.Fatalf("Price2 = %v, want nil", *items[0].Price2)
	}
}
