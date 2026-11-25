package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBookingCapacityAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lingua.json")
	data, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	lesson := slot{
		ID: "one-seat", Language: "english", Format: "individual", Capacity: 1,
		Duration: 55, StartsAt: time.Now().Add(time.Hour), MeetingURL: "https://meet.example.com/lesson",
	}
	app := application{store: data, catalog: catalog{scheduled: []slot{lesson}}}
	handler := app.handler(t.TempDir())

	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	expectStatus := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d, want %d: %s", w.Code, status, w.Body.String())
		}
	}
	readAccount := func(w *httptest.ResponseRecorder) accountView {
		t.Helper()
		expectStatus(w, http.StatusOK)
		var result struct{ Account accountView }
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Account
	}

	guestCatalog := request("GET", "/api/catalog", "", nil)
	expectStatus(guestCatalog, http.StatusOK)
	if strings.Contains(guestCatalog.Body.String(), lesson.MeetingURL) {
		t.Fatal("catalog exposed a private meeting link")
	}

	cookies := make([]*http.Cookie, 0, 2)
	for _, email := range []string{"anya@example.test", "alex@example.test"} {
		response := request("POST", "/api/register", `{"name":"Анна","email":"`+email+`","password":"correct horse"}`, nil)
		expectStatus(response, http.StatusCreated)
		cookie := response.Result().Cookies()[0]
		response = request("POST", "/api/subscription", `{"planId":"individual"}`, cookie)
		expectStatus(response, http.StatusOK)
		cookies = append(cookies, cookie)
	}

	responses := make([]*httptest.ResponseRecorder, len(cookies))
	var group sync.WaitGroup
	for i, cookie := range cookies {
		group.Go(func() {
			responses[i] = request("POST", "/api/bookings", `{"slotId":"one-seat"}`, cookie)
		})
	}
	group.Wait()
	winner, loser := 0, 1
	if responses[0].Code != http.StatusOK {
		winner, loser = 1, 0
	}
	expectStatus(responses[loser], http.StatusConflict)

	account := readAccount(responses[winner])
	if account.Subscription.Remaining != 3 || len(account.Bookings) != 1 || account.Bookings[0].Slot.MeetingURL != lesson.MeetingURL {
		t.Fatalf("booking, balance, or meeting link missing: %+v", account)
	}
	id := account.Bookings[0].ID
	expectStatus(request("DELETE", "/api/bookings/"+id, "", cookies[loser]), http.StatusNotFound)
	account = readAccount(request("DELETE", "/api/bookings/"+id, "", cookies[winner]))
	if account.Subscription.Remaining != 4 || len(account.Bookings) != 0 {
		t.Fatalf("cancellation did not restore the credit: %+v", account)
	}

	app.store, err = openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	account = readAccount(request("POST", "/api/bookings", `{"slotId":"one-seat"}`, cookies[loser]))
	if account.Subscription.Remaining != 3 || len(account.Bookings) != 1 {
		t.Fatalf("freed seat or subscription unavailable after reopening the store: %+v", account)
	}
}
