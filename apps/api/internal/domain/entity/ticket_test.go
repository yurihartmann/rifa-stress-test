package entity

import "testing"

func TestFormatTicketNumber(t *testing.T) {
	if got := FormatTicketNumber(1, 1000); got != "0001" {
		t.Fatalf("got %s", got)
	}
	if got := FormatTicketNumber(1000, 1000); got != "1000" {
		t.Fatalf("got %s", got)
	}
	if got := FormatTicketNumber(7, 10); got != "07" {
		t.Fatalf("got %s", got)
	}
}
