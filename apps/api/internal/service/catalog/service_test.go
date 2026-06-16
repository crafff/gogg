package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeQuerier struct {
	versions    []string
	versionsErr error
	regions     []string
	regionsErr  error
}

func (f fakeQuerier) ListVersionsWithData(_ context.Context) ([]string, error) {
	return f.versions, f.versionsErr
}
func (f fakeQuerier) ListRegionsWithData(_ context.Context) ([]string, error) {
	return f.regions, f.regionsErr
}

func TestListVersionsWithData_passesThrough(t *testing.T) {
	s := New(fakeQuerier{versions: []string{"15.1.1", "15.1"}})
	got, err := s.ListVersionsWithData(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"15.1.1", "15.1"}) {
		t.Errorf("got %v", got)
	}
}

func TestListVersionsWithData_nilToEmpty(t *testing.T) {
	s := New(fakeQuerier{versions: nil})
	got, err := s.ListVersionsWithData(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got == nil {
		t.Fatal("should not return nil slice")
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestListVersionsWithData_wrapsError(t *testing.T) {
	want := errors.New("db down")
	s := New(fakeQuerier{versionsErr: want})
	_, err := s.ListVersionsWithData(context.Background())
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want errors.Is(_, want)", err)
	}
}

func TestListRegionsWithData_passesThrough(t *testing.T) {
	s := New(fakeQuerier{regions: []string{"KR", "NA1"}})
	got, err := s.ListRegionsWithData(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"KR", "NA1"}) {
		t.Errorf("got %v", got)
	}
}

func TestListRegionsWithData_nilToEmpty(t *testing.T) {
	s := New(fakeQuerier{regions: nil})
	got, err := s.ListRegionsWithData(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want empty slice", got)
	}
}

func TestListRegionsWithData_wrapsError(t *testing.T) {
	want := errors.New("db down")
	s := New(fakeQuerier{regionsErr: want})
	_, err := s.ListRegionsWithData(context.Background())
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want errors.Is(_, want)", err)
	}
}
