package edits

import "testing"

func TestInventoryProducesEditReadyKeysAndValues(t *testing.T) {
	got, err := Inventory([]byte(sample), "metallb.valuesObject")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"metallb.valuesObject.speaker.frr.enabled": "true",
		"metallb.valuesObject.frrk8s.enabled":      "true",
	}
	if len(got) != len(want) {
		t.Fatalf("want %d scalars, got %d: %+v", len(want), len(got), got)
	}
	for _, s := range got {
		v, ok := want[s.Key]
		if !ok {
			t.Errorf("unexpected key %q", s.Key)
			continue
		}
		if s.Value != v {
			t.Errorf("%s = %q, want %q", s.Key, s.Value, v)
		}
	}
}

// Whatever the inventory reports must be directly usable as an Edit, or the
// whole point of handing it to the model is lost.
func TestInventoryKeysAndValuesRoundTripThroughApply(t *testing.T) {
	root := repo(t, map[string]string{"addons/values.yaml": sample})
	inv, err := Inventory([]byte(sample), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) == 0 {
		t.Fatal("empty inventory")
	}
	for _, s := range inv {
		res, err := Apply(root, Policy{Allow: []string{"addons/**"}},
			[]Edit{{Path: "addons/values.yaml", Key: s.Key, From: s.Value, To: s.Value}})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Applied) != 1 {
			t.Errorf("inventory entry %q=%q was not applyable: %+v", s.Key, s.Value, res.Rejected)
		}
	}
}

func TestInventoryIncludesListIndices(t *testing.T) {
	inv, _ := Inventory([]byte(sample), "metallb.containers")
	if len(inv) != 1 || inv[0].Key != "metallb.containers.0.image" {
		t.Fatalf("want metallb.containers.0.image, got %+v", inv)
	}
}
