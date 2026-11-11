package catalog

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeServicesDedupesInCatalogOrder(t *testing.T) {
	got, err := NormalizeServices([]string{"pipa", "listrik", "pipa"})
	if err != nil || !reflect.DeepEqual(got, []string{"listrik", "pipa"}) {
		t.Fatalf("NormalizeServices() = %v, %v; want [listrik pipa]", got, err)
	}
}

func TestNormalizeServicesRejectsUnknownAndEmpty(t *testing.T) {
	for _, in := range [][]string{{"pipa", "sihir"}, {}, nil} {
		if _, err := NormalizeServices(in); !errors.Is(err, ErrInvalidService) {
			t.Fatalf("NormalizeServices(%v) error = %v, want ErrInvalidService", in, err)
		}
	}
}

func TestNormalizeArea(t *testing.T) {
	if got, err := NormalizeArea("  Jakarta   Selatan "); err != nil || got != "Jakarta Selatan" {
		t.Fatalf("NormalizeArea() = %q, %v", got, err)
	}
	if _, err := NormalizeArea(" J "); !errors.Is(err, ErrInvalidArea) {
		t.Fatalf("NormalizeArea(short) error = %v, want ErrInvalidArea", err)
	}
}
