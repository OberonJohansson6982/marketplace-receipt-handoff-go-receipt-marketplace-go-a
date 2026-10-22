package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var ErrOrderNotPaid = errors.New("receipt requires a paid order")

type OrderHandoff struct {
	OrderID       string `json:"order_id"`
	PaymentStatus string `json:"payment_status"`
	Buyer         Buyer  `json:"buyer"`
	Seller        Seller `json:"seller"`
	TotalCents    int64  `json:"total_cents"`
	Currency      string `json:"currency"`
}

type Buyer struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Seller struct {
	DisplayName string `json:"display_name"`
	AssetName   string `json:"asset_name"`
	DownloadURL string `json:"download_url"`
}

type HandoffResult struct {
	OrderID   string `json:"order_id"`
	State     string `json:"state"`
	MessageID string `json:"message_id"`
}

type EmailSender interface {
	SendEmail(context.Context, Email, string) (SendResult, error)
}

type ReceiptSender struct {
	Email EmailSender
}

func (s ReceiptSender) Handoff(ctx context.Context, order OrderHandoff) (HandoffResult, error) {
	if order.PaymentStatus != "paid" {
		return HandoffResult{}, ErrOrderNotPaid
	}
	if order.OrderID == "" || order.Buyer.Email == "" || order.Seller.DisplayName == "" || order.Seller.AssetName == "" || order.Seller.DownloadURL == "" || order.TotalCents < 0 || order.Currency == "" {
		return HandoffResult{}, errors.New("order handoff is incomplete")
	}

	email := Email{
		To:      order.Buyer.Email,
		Subject: fmt.Sprintf("Receipt for %s from %s", order.Seller.AssetName, order.Seller.DisplayName),
		Text: fmt.Sprintf(
			"Hello %s,\n\nOrder %s is paid.\nSeller: %s\nAsset: %s\nTotal: %s %0.2f\nDownload: %s\n",
			order.Buyer.Name,
			order.OrderID,
			order.Seller.DisplayName,
			order.Seller.AssetName,
			order.Currency,
			float64(order.TotalCents)/100,
			order.Seller.DownloadURL,
		),
	}
	result, err := s.Email.SendEmail(ctx, email, "receipt:"+order.OrderID)
	if err != nil {
		return HandoffResult{}, err
	}
	return HandoffResult{OrderID: order.OrderID, State: "receipt_sent", MessageID: result.MessageID}, nil
}

type Handler struct {
	Sender ReceiptSender
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/orders/handoff" {
		http.NotFound(w, r)
		return
	}
	defer r.Body.Close()
	var order OrderHandoff
	if err := decodeJSON(r, &order); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order handoff"})
		return
	}
	result, err := h.Sender.Handoff(r.Context(), order)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrOrderNotPaid) || err.Error() == "order handoff is incomplete" {
			status = http.StatusUnprocessableEntity
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 && apiErr.HTTPStatus != http.StatusTooManyRequests {
			status = apiErr.HTTPStatus
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func decodeJSON(r *http.Request, value any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
