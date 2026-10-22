package receipt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type recordingSender struct {
	calls int
	email Email
	key   string
}

func (s *recordingSender) SendEmail(_ context.Context, email Email, key string) (SendResult, error) {
	s.calls++
	s.email = email
	s.key = key
	return SendResult{MessageID: "msg_receipt_42"}, nil
}

func TestReceiptDecision(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantErr   error
		wantCalls int
	}{
		{name: "paid order sends handoff", status: "paid", wantCalls: 1},
		{name: "pending order sends nothing", status: "pending", wantErr: ErrOrderNotPaid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mail := &recordingSender{}
			sender := ReceiptSender{Email: mail}
			order := sampleOrder()
			order.PaymentStatus = tt.status

			got, err := sender.Handoff(context.Background(), order)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if mail.calls != tt.wantCalls {
				t.Fatalf("email calls = %d, want %d", mail.calls, tt.wantCalls)
			}
			if tt.wantCalls == 1 {
				if got.State != "receipt_sent" || got.MessageID != "msg_receipt_42" {
					t.Fatalf("result = %+v", got)
				}
				if mail.key != "receipt:ord_42" {
					t.Fatalf("idempotency key = %q", mail.key)
				}
				if mail.email.To != "buyer@example.com" || mail.email.Subject == "" {
					t.Fatalf("email = %+v", mail.email)
				}
			}
		})
	}
}

func TestEmailClientRetriesRateLimitAndUsesRequestBoundary(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != emailSendPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "receipt:ord_42" {
			t.Fatalf("headers = %#v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		wantFields := map[string]bool{"to": true, "subject": true, "body": true}
		for field := range body {
			if !wantFields[field] {
				t.Fatalf("unexpected request field %q", field)
			}
		}
		if body["body"] != "Paid" {
			t.Fatalf("body = %v, want Paid", body["body"])
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"message_id":"msg_receipt_42"},"metadata":{}}`))
	}))
	defer server.Close()

	client := NewEmailClient("test-key")
	client.BaseURL = server.URL
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	got, err := client.SendEmail(context.Background(), Email{To: "buyer@example.com", Subject: "Receipt", Text: "Paid"}, "receipt:ord_42")
	if err != nil {
		t.Fatal(err)
	}
	if got.MessageID != "msg_receipt_42" || requests != 2 {
		t.Fatalf("result = %+v, requests = %d", got, requests)
	}
}

func sampleOrder() OrderHandoff {
	return OrderHandoff{
		OrderID:       "ord_42",
		PaymentStatus: "paid",
		Buyer:         Buyer{Email: "buyer@example.com", Name: "Avery"},
		Seller:        Seller{DisplayName: "Northwind Assets", AssetName: "Studio Presets", DownloadURL: "https://downloads.example.com/orders/ord_42"},
		TotalCents:    4900,
		Currency:      "USD",
	}
}
