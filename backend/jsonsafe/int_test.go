package jsonsafe

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestIntEncodesAsANumber(t *testing.T) {
	for _, tc := range []struct {
		in   Int
		want string
	}{
		{0, "0"},
		{1725580800, "1725580800"},
		{-42, "-42"},
		{MaxInt, "9007199254740991"},
		{-MaxInt, "-9007199254740991"},
	} {
		got, err := json.Marshal(struct {
			V Int `json:"v"`
		}{tc.in})
		if err != nil {
			t.Errorf("Marshal(%d): %v", tc.in, err)
			continue
		}
		if want := `{"v":` + tc.want + `}`; string(got) != want {
			t.Errorf("Marshal(%d) = %s, want %s", tc.in, got, want)
		}
	}
}

// A value a client would misread fails the whole encoding, so nothing holding
// it is sent.
func TestIntRefusesWhatAClientWouldMisread(t *testing.T) {
	for _, v := range []Int{MaxInt + 1, -MaxInt - 1, 1<<63 - 1, -1 << 63} {
		got, err := json.Marshal([]Int{1, v})
		if !errors.Is(err, ErrImprecise) {
			t.Errorf("Marshal(%d) = %s, %v; want ErrImprecise", v, got, err)
		}
	}
}

// Decoding needs nothing of its own: an Int is an int64 underneath.
func TestIntDecodes(t *testing.T) {
	var v struct {
		V Int `json:"v"`
	}
	if err := json.Unmarshal([]byte(`{"v":1725580800}`), &v); err != nil || v.V != 1725580800 {
		t.Errorf("Unmarshal = %d, %v; want 1725580800", v.V, err)
	}
}
