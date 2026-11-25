package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func studioFixture(t *testing.T) (*application, func(string, string, string, any, int) *httptest.ResponseRecorder) {
	t.Helper()
	data, err := openStore(filepath.Join(t.TempDir(), "lingua.json"))
	if err != nil {
		t.Fatal(err)
	}
	err = data.update(func(data *state) error {
		data.Users["student"] = user{ID: "student", Name: "Ученик", Email: "student@example.test",
			Subscription: &subscription{ID: "package", PlanID: "group", Remaining: 4, EndsAt: time.Now().AddDate(0, 0, 30)}}
		data.Users["teacher"] = user{ID: "teacher", Name: "Преподаватель", Teacher: true}
		for _, id := range []string{"student", "teacher"} {
			data.Sessions[digest(id)] = session{id, time.Now().Add(time.Hour)}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	app := &application{store: data, catalog: catalog{demo: true}, school: defaultSchool()}
	server := app.handler(t.TempDir())
	request := func(method, path, actor string, payload any, status int) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if actor != "" {
			r.AddCookie(&http.Cookie{Name: "lingua_session", Value: actor})
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	return app, request
}

func decodeStudio[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var data T
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestManagedCalendarReschedulesWithoutAnotherCredit(t *testing.T) {
	_, request := studioFixture(t)
	start := time.Now().AddDate(0, 0, 1).Truncate(time.Minute)
	form := map[string]any{"language": "english", "format": "group", "moduleId": "en-intro", "duration": 55, "capacity": 4,
		"startsAt": start, "meetingUrl": "https://meet.example.test/library"}
	request("POST", "/api/teacher/slots", "student", form, http.StatusForbidden)
	request("POST", "/api/teacher/slots", "teacher", form, http.StatusNoContent)
	request("POST", "/api/teacher/slots", "teacher", form, http.StatusConflict)
	form["startsAt"] = start.Add(time.Hour)
	request("POST", "/api/teacher/slots", "teacher", form, http.StatusNoContent)

	catalog := request("GET", "/api/catalog", "", nil, http.StatusOK)
	if strings.Contains(catalog.Body.String(), "meet.example.test") || strings.Contains(catalog.Body.String(), "Составьте пять предложений") {
		t.Fatal("guest catalog exposed private data")
	}
	list := decodeStudio[struct {
		Slots        []slot
		DemoSchedule bool
	}](t, catalog)
	if len(list.Slots) != 2 || list.DemoSchedule || list.Slots[0].ModuleID != "en-intro" {
		t.Fatalf("invalid managed calendar: %+v", list)
	}
	type answer struct{ Account accountView }
	w := request("POST", "/api/bookings", "student", map[string]string{"slotId": list.Slots[0].ID}, http.StatusOK)
	first := decodeStudio[answer](t, w).Account
	if first.Subscription.Remaining != 3 || len(first.Bookings) != 1 || first.Bookings[0].ModuleID != "en-intro" {
		t.Fatal("missing module or incorrect balance")
	}
	bookingID := first.Bookings[0].ID
	w = request("PATCH", "/api/bookings/"+bookingID, "student", map[string]string{"slotId": list.Slots[1].ID}, http.StatusOK)
	moved := decodeStudio[answer](t, w).Account
	if moved.Subscription.Remaining != 3 || len(moved.Bookings) != 1 || moved.Bookings[0].ID != bookingID || moved.Bookings[0].Slot.ID != list.Slots[1].ID {
		t.Fatal("reschedule must preserve identity and balance")
	}
	request("DELETE", "/api/teacher/slots/"+list.Slots[1].ID, "teacher", nil, http.StatusNoContent)
	w = request("GET", "/api/account", "student", nil, http.StatusOK)
	canceled := decodeStudio[answer](t, w).Account
	if canceled.Subscription.Remaining != 4 || !canceled.Bookings[0].Cancelled || canceled.Bookings[0].Slot.MeetingURL != "" {
		t.Fatal("teacher cancellation must release credit and remove meeting access")
	}
}

func TestProgressFeedbackAndCheckoutArePersistent(t *testing.T) {
	app, request := studioFixture(t)
	request("PATCH", "/api/progress/en-intro", "student", map[string]bool{"prepared": true}, http.StatusOK)
	if err := app.store.update(func(data *state) error {
		data.Bookings["lesson"] = booking{ID: "lesson", UserID: "student", ModuleID: "en-intro", Slot: slot{Language: "english", StartsAt: time.Now().Add(-time.Hour)}}
		person := data.Users["student"]
		person.Subscription = nil
		data.Users[person.ID] = person
		data.Orders["order"] = order{ID: "order", UserID: "student", PlanID: "group", Price: 3600, Status: "pending"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	request("PATCH", "/api/teacher/feedback/lesson", "teacher",
		map[string]any{"moduleId": "en-intro", "nextModuleId": "en-daily", "feedback": "Хорошая практика.", "completed": true}, http.StatusNoContent)
	payment := gatewayPayment{ID: "vendor-order", Status: "succeeded", Paid: true, Metadata: map[string]string{"order_id": "order"}}
	payment.Amount.Currency, payment.Amount.Value = "RUB", "1.00"
	if _, err := app.applyPayment("order", payment); err == nil {
		t.Fatal("wrong payment amount accepted")
	}
	payment.Amount.Value = "3600.00"
	for i := 0; i < 2; i++ {
		if result, err := app.applyPayment("order", payment); err != nil || result.Status != "paid" {
			t.Fatalf("payment confirmation didn't succeed: %v", err)
		}
	}
	loaded := decodeStudio[struct{ Account accountView }](t, request("GET", "/api/account", "student", nil, http.StatusOK)).Account
	if loaded.Subscription.Remaining != 4 || len(loaded.Progress) != 1 || !loaded.Progress[0].Completed ||
		!loaded.Progress[0].Prepared || loaded.Progress[0].NextModuleID != "en-daily" {
		t.Fatalf("unexpected progress or package: %+v", loaded)
	}
}

func TestPasswordResetInvalidatesTokenAndSessions(t *testing.T) {
	app, request := studioFixture(t)
	if err := app.store.update(func(data *state) error {
		data.Resets[digest("test-reset")] = passwordReset{"student", time.Now().Add(time.Minute)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	form := map[string]string{"token": "test-reset", "password": "fresh-test-password"}
	request("POST", "/api/reset-password", "", form, http.StatusNoContent)
	request("POST", "/api/reset-password", "", form, http.StatusBadRequest)
	request("PATCH", "/api/progress/en-intro", "student", map[string]bool{"prepared": true}, http.StatusUnauthorized)
	request("POST", "/api/login", "", map[string]string{"email": "student@example.test", "password": "fresh-test-password"}, http.StatusOK)
}
