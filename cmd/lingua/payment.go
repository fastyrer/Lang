package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

type order struct {
	ID, UserID, PlanID, ProviderID, Status, URL, ReturnURL string
	Price                                                  int
	CreatedAt                                              time.Time
}

func (o order) pending() bool { return o.Status == "creating" || o.Status == "pending" }

type orderView struct {
	ID     string `json:"id"`
	PlanID string `json:"planId"`
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
}

func (o order) public() orderView { return orderView{o.ID, o.PlanID, o.Status, o.URL} }

type checkout struct {
	shop, key, base string
	client          *http.Client
}

type gatewayPayment struct {
	ID           string
	Status       string
	Paid         bool
	Amount       struct{ Value, Currency string }
	Confirmation struct {
		URL string `json:"confirmation_url"`
	}
	Metadata map[string]string
}

var providerID = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)

func (app *application) paymentMode() string {
	if app.payment != nil {
		return "live"
	}
	if app.demoBilling || app.rate == nil {
		return "demo"
	}
	return "disabled"
}

func (gateway *checkout) request(method, path, key string, payload any) (gatewayPayment, error) {
	var response gatewayPayment
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
	}
	if err != nil {
		return response, err
	}
	request, err := http.NewRequest(method, gateway.base+path, bytes.NewReader(data))
	if err != nil {
		return response, err
	}
	request.SetBasicAuth(gateway.shop, gateway.key)
	if key != "" {
		request.Header.Set("Idempotence-Key", key)
		request.Header.Set("Content-Type", "application/json")
	}
	reply, err := gateway.client.Do(request)
	if err != nil {
		return response, err
	}
	defer reply.Body.Close()
	if reply.StatusCode < 200 || reply.StatusCode > 299 {
		return response, fmt.Errorf("checkout returned HTTP %d", reply.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(reply.Body, 64<<10)).Decode(&response)
	return response, err
}

func amount(price int) string { return fmt.Sprintf("%d.00", price) }

func (app *application) createOrder(w http.ResponseWriter, r *http.Request) {
	if !app.allowAuth(w, r, "payment-create", 12) {
		return
	}
	if app.payment == nil {
		problem(w, http.StatusServiceUnavailable, "Онлайн-оплата пока недоступна.")
		return
	}
	var form struct {
		PlanID string `json:"planId"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	plan, ok := findPlan(form.PlanID)
	if !ok {
		fail(w, invalid("Выберите пакет занятий."))
		return
	}
	plan.Price = app.school.Prices[plan.ID]
	now := time.Now()
	var paymentOrder order
	err := app.store.update(func(data *state) error {
		person, err := data.user(r, now)
		if err != nil {
			return err
		}
		if person.Subscription.active(now) {
			return conflict("У вас уже есть действующий пакет.")
		}
		for _, record := range data.Orders {
			if record.UserID == person.ID && record.pending() {
				if record.PlanID != plan.ID {
					return conflict("Сначала завершите или проверьте текущую оплату.")
				}
				paymentOrder = record
				return nil
			}
		}
		id, err := randomID()
		if err != nil {
			return err
		}
		paymentOrder = order{ID: id, UserID: person.ID, PlanID: plan.ID, Price: plan.Price, Status: "creating",
			CreatedAt: now, ReturnURL: app.publicURL + "/cabinet?order=" + id}
		data.Orders[id] = paymentOrder
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	if paymentOrder.ProviderID == "" {
		remote, err := app.payment.request(http.MethodPost, "/payments", paymentOrder.ID, map[string]any{
			"amount":  map[string]string{"value": amount(paymentOrder.Price), "currency": "RUB"},
			"capture": true, "confirmation": map[string]string{"type": "redirect", "return_url": paymentOrder.ReturnURL},
			"description": "Lingua · " + plan.Name, "metadata": map[string]string{"order_id": paymentOrder.ID},
		})
		if err == nil {
			paymentOrder, err = app.applyPayment(paymentOrder.ID, remote)
		}
		if err != nil {
			fail(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"order": paymentOrder.public()})
}

func (app *application) applyPayment(id string, remote gatewayPayment) (order, error) {
	var result order
	err := app.store.update(func(data *state) error {
		current, ok := data.Orders[id]
		if !ok || !providerID.MatchString(remote.ID) || current.ProviderID != "" && current.ProviderID != remote.ID ||
			remote.Metadata["order_id"] != id || remote.Amount.Currency != "RUB" || remote.Amount.Value != amount(current.Price) {
			return errors.New("checkout response does not match the order")
		}
		if !current.pending() {
			result = current
			return nil
		}
		current.ProviderID = remote.ID
		switch remote.Status {
		case "pending":
			if validHTTPS(remote.Confirmation.URL) {
				current.URL = remote.Confirmation.URL
			}
			current.Status = "pending"
		case "canceled":
			current.Status, current.URL = "canceled", ""
		case "succeeded":
			person := data.Users[current.UserID]
			plan, ok := findPlan(current.PlanID)
			if !remote.Paid || !ok || person.ID == "" || person.Subscription.active(time.Now()) {
				return errors.New("checkout completed but could not activate the package")
			}
			person.Subscription = &subscription{ID: id, PlanID: plan.ID, Remaining: plan.Lessons,
				EndsAt: time.Now().AddDate(0, 0, plan.Days)}
			data.Users[person.ID] = person
			current.Status, current.URL = "paid", ""
		default:
			return errors.New("unexpected checkout status")
		}
		data.Orders[id], result = current, current
		return nil
	})
	return result, err
}

func (app *application) refreshOrder(w http.ResponseWriter, r *http.Request) {
	if app.payment == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !app.allowAuth(w, r, "payment-status", 24) {
		return
	}
	now := time.Now()
	id := r.PathValue("id")
	var current order
	app.store.read(func(data *state) {
		if person, err := data.user(r, now); err == nil && data.Orders[id].UserID == person.ID {
			current = data.Orders[id]
		}
	})
	if current.ID == "" {
		problem(w, http.StatusNotFound, "Платёж не найден.")
		return
	}
	if current.pending() && current.ProviderID != "" {
		remote, err := app.payment.request(http.MethodGet, "/payments/"+current.ProviderID, "", nil)
		if err != nil {
			fail(w, err)
			return
		}
		current, err = app.applyPayment(current.ID, remote)
		if err != nil {
			fail(w, err)
			return
		}
	}
	var view accountView
	app.store.read(func(data *state) { view = data.view(data.Users[current.UserID], now) })
	writeJSON(w, http.StatusOK, map[string]any{"account": view, "order": current.public()})
}

func (app *application) paymentNotification(w http.ResponseWriter, r *http.Request) {
	if app.payment == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var event struct {
		Event  string
		Object gatewayPayment
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if event.Event != "payment.succeeded" && event.Event != "payment.canceled" {
		w.WriteHeader(http.StatusOK)
		return
	}
	var current order
	id := event.Object.Metadata["order_id"]
	app.store.read(func(data *state) { current = data.Orders[id] })
	if current.ID == "" || !current.pending() || !providerID.MatchString(event.Object.ID) ||
		current.ProviderID != "" && current.ProviderID != event.Object.ID {
		w.WriteHeader(http.StatusOK)
		return
	}
	remote, err := app.payment.request(http.MethodGet, "/payments/"+event.Object.ID, "", nil)
	if err == nil {
		_, err = app.applyPayment(id, remote)
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
