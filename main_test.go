package main

import "testing"

func TestShouldSendReceipt(t *testing.T) {
	tests := []struct {
		name   string
		action donorAction
		want   bool
	}{
		{"normal donation", donorAction{DonorEmail: "a@example.org", Amount: 2500, DelayHours: 24}, true},
		{"zero donation", donorAction{DonorEmail: "a@example.org", Amount: 0, DelayHours: 24}, false},
		{"too long", donorAction{DonorEmail: "a@example.org", Amount: 2500, DelayHours: 73}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSendReceipt(tt.action); got != tt.want {
				t.Fatalf("shouldSendReceipt(%+v) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}
