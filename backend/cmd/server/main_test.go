package main

import "testing"

func TestCookieSecureFromEnv(t *testing.T) {
	tests := []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{value: "", want: true},
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "FALSE", want: false},
		{value: "0", want: false},
		// A typo must stop the server instead of guessing.
		{value: "nein", wantErr: true},
		{value: "off", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("COOKIE_SECURE", tt.value)
			got, err := cookieSecureFromEnv()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
