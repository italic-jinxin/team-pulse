package credentials

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if _, err := store.Get(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty get error = %v", err)
	}

	want := Credential{Token: "secret", Source: "memory", Login: "alice"}
	if err := store.Set(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("credential = %#v, want %#v", got, want)
	}

	if err := store.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted get error = %v", err)
	}
}
