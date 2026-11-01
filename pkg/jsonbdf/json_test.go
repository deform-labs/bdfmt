package jsonbdf

import (
	"testing"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

func TestJSONRoundTrip(t *testing.T) {
	comp := bdf.NewTagCompound()
	comp.Set("name", bdf.TagString("Zombie"))
	comp.Set("health", bdf.TagInt(20))
	comp.Set("customNameVisible", bdf.TagByte(1))

	jsonBytes, err := ToJSON(comp, true)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	decoded, err := FromJSON(jsonBytes)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	decodedComp, ok := decoded.(*bdf.TagCompound)
	if !ok {
		t.Fatalf("Expected TagCompound, got %T", decoded)
	}

	name, ok := decodedComp.GetString("name")
	if !ok || name != "Zombie" {
		t.Errorf("Expected name 'Zombie', got %q", name)
	}
}
