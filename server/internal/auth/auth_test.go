package auth

import "testing"

func TestWebSocketTicketIsSingleUse(t *testing.T) {
	manager := New("01234567890123456789012345678901")
	ticket, err := manager.IssueWebSocketTicket(42)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	if ticket == "" {
		t.Fatal("ticket is empty")
	}
	userID, err := manager.ConsumeWebSocketTicket(ticket)
	if err != nil || userID != 42 {
		t.Fatalf("consume ticket: user=%d err=%v", userID, err)
	}
	if _, err = manager.ConsumeWebSocketTicket(ticket); err == nil {
		t.Fatal("ticket was accepted more than once")
	}
}

func TestUnknownWebSocketTicketIsRejected(t *testing.T) {
	manager := New("01234567890123456789012345678901")
	if _, err := manager.ConsumeWebSocketTicket("not-issued"); err == nil {
		t.Fatal("unknown ticket was accepted")
	}
}
