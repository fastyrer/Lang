package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type application struct {
	store         *store
	catalog       catalog
	secureCookies bool
	school        school
	publicURL     string
	mail          *mailConfig
	rate          *authLimiter
	payment       *checkout
	demoBilling   bool
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--check-store" {
		if _, err := os.Stat(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		if _, err := openStore(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		return
	}
	data, err := openStore(env("LINGUA_DATA", "data/lingua.json"))
	if err != nil {
		log.Fatal(err)
	}
	schedule, err := loadCatalog(os.Getenv("LINGUA_SCHEDULE"))
	if err != nil {
		log.Fatal(err)
	}
	school, err := loadSchool(os.Getenv("LINGUA_SCHOOL"))
	if err != nil {
		log.Fatal(err)
	}
	app := application{store: data, catalog: schedule, school: school,
		secureCookies: os.Getenv("LINGUA_SECURE_COOKIES") == "1"}
	if err := app.configure(); err != nil {
		log.Fatal(err)
	}
	if err := setupTeacher(data, school); err != nil {
		log.Fatal(err)
	}
	addr := env("LINGUA_ADDR", "127.0.0.1:8080")
	server := &http.Server{
		Addr: addr, Handler: app.handler(env("LINGUA_WEB", "web/dist")), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("Lingua: http://%s", addr)
	log.Fatal(server.ListenAndServe())
}

func (app *application) handler(bundle string) http.Handler {
	mux := http.NewServeMux()
	app.routes(mux)
	index := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(bundle, "index.html"))
	}
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /cabinet", index)
	files := http.FileServer(http.Dir(bundle))
	mux.HandleFunc("GET /assets/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(filepath.Base(r.URL.Path), "-") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
	mux.Handle("GET /images/", files)
	return headers(http.NewCrossOriginProtection().Handler(mux))
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil || d.Decode(new(any)) != io.EOF {
		problem(w, http.StatusBadRequest, "Проверьте данные и попробуйте ещё раз.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Print(err)
	}
}

func problem(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type requestError struct {
	status  int
	message string
}

func (e requestError) Error() string { return e.message }

func fail(w http.ResponseWriter, err error) {
	var rejection requestError
	if errors.As(err, &rejection) {
		problem(w, rejection.status, rejection.message)
		return
	}
	log.Print(err)
	problem(w, http.StatusInternalServerError, "Не удалось сохранить изменения. Попробуйте ещё раз.")
}

var loginRequired = requestError{http.StatusUnauthorized, "Войдите, чтобы продолжить."}

func invalid(message string) error  { return requestError{http.StatusBadRequest, message} }
func conflict(message string) error { return requestError{http.StatusConflict, message} }

func (app *application) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/catalog", app.getCatalog)
	mux.HandleFunc("POST /api/register", app.register)
	mux.HandleFunc("POST /api/login", app.login)
	mux.HandleFunc("POST /api/logout", app.logout)
	mux.HandleFunc("GET /api/account", app.account)
	mux.HandleFunc("PATCH /api/profile", app.updateProfile)
	mux.HandleFunc("POST /api/subscription", app.subscribe)
	mux.HandleFunc("DELETE /api/subscription", app.cancelSubscription)
	mux.HandleFunc("POST /api/bookings", app.book)
	mux.HandleFunc("DELETE /api/bookings/{id}", app.cancelBooking)
	mux.HandleFunc("PATCH /api/bookings/{id}", app.reschedule)
	mux.HandleFunc("PATCH /api/progress/{id}", app.prepare)
	mux.HandleFunc("GET /api/teacher", app.teaching)
	mux.HandleFunc("POST /api/teacher/slots", app.saveSlot)
	mux.HandleFunc("PUT /api/teacher/slots/{id}", app.saveSlot)
	mux.HandleFunc("DELETE /api/teacher/slots/{id}", app.removeSlot)
	mux.HandleFunc("PATCH /api/teacher/feedback/{id}", app.feedback)
	mux.HandleFunc("POST /api/recovery", app.recovery)
	mux.HandleFunc("POST /api/reset-password", app.resetPassword)
	mux.HandleFunc("POST /api/orders", app.createOrder)
	mux.HandleFunc("POST /api/orders/{id}/refresh", app.refreshOrder)
	mux.HandleFunc("POST /api/payment-notifications", app.paymentNotification)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		problem(w, http.StatusNotFound, "Страница не найдена.")
	})
}

func (app *application) getCatalog(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var slots []slot
	var demo, signedIn bool
	app.store.read(func(data *state) {
		demo = app.catalog.demo && !data.ScheduleEnabled
		_, err := data.user(r, now)
		signedIn = err == nil
		slots = app.calendar(data, now)
		seats := map[string]int{}
		for _, booking := range data.Bookings {
			if !booking.Cancelled && booking.Slot.StartsAt.After(now) {
				seats[booking.Slot.ID]++
			}
		}
		for i := range slots {
			slots[i].Available = max(0, slots[i].Capacity-seats[slots[i].ID])
			slots[i].MeetingURL = ""
		}
	})
	visible := app.school
	if !signedIn {
		visible.Modules = slices.Clone(visible.Modules)
		for i, item := range visible.Modules {
			if !item.Open {
				visible.Modules[i].Preparation = ""
				visible.Modules[i].MaterialURL = ""
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"slots": slots, "plans": app.publicPlans(), "demoSchedule": demo, "school": visible,
		"payments": app.paymentMode(), "recovery": app.mail != nil,
	})
}

func (app *application) account(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var account *accountView
	app.store.read(func(data *state) {
		if user, err := data.user(r, now); err == nil {
			view := data.view(user, now)
			account = &view
		}
	})
	writeJSON(w, http.StatusOK, map[string]any{"account": account})
}

func (app *application) changeAccount(w http.ResponseWriter, r *http.Request, change func(*state, *user, time.Time) error) bool {
	now := time.Now()
	var view accountView
	err := app.store.update(func(data *state) error {
		user, err := data.user(r, now)
		if err != nil {
			return err
		}
		if err := change(data, &user, now); err != nil {
			return err
		}
		data.Users[user.ID] = user
		view = data.view(user, now)
		return nil
	})
	if err != nil {
		fail(w, err)
		return false
	}
	writeJSON(w, http.StatusOK, map[string]any{"account": view})
	return true
}

func (app *application) updateProfile(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Name     string `json:"name"`
		Language string `json:"language"`
		Level    string `json:"level"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	app.changeAccount(w, r, func(data *state, user *user, _ time.Time) error {
		name, err := validateName(form.Name)
		if err != nil {
			return err
		}
		if form.Language != "" && !isLanguage(form.Language) {
			return invalid("Выберите язык из списка.")
		}
		if form.Level != "" && !isLevel(form.Level) {
			return invalid("Выберите уровень из списка.")
		}
		user.Name, user.Language, user.Level = name, form.Language, form.Level
		return nil
	})
}

func (app *application) subscribe(w http.ResponseWriter, r *http.Request) {
	if app.paymentMode() != "demo" {
		problem(w, http.StatusForbidden, "Оформите пакет через форму оплаты.")
		return
	}
	var form struct {
		PlanID string `json:"planId"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	app.changeAccount(w, r, func(_ *state, user *user, now time.Time) error {
		plan, ok := findPlan(form.PlanID)
		if !ok {
			return invalid("Выберите абонемент из списка.")
		}
		if user.Subscription.active(now) {
			return conflict("У вас уже есть активный абонемент.")
		}
		id, err := randomID()
		if err != nil {
			return err
		}
		user.Subscription = &subscription{
			ID: id, PlanID: plan.ID, Remaining: plan.Lessons, EndsAt: now.AddDate(0, 0, plan.Days),
		}
		return nil
	})
}

func (app *application) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	if app.paymentMode() != "demo" {
		problem(w, http.StatusForbidden, "По вопросам возврата свяжитесь со школой.")
		return
	}
	app.changeAccount(w, r, func(_ *state, user *user, now time.Time) error {
		if user.Subscription.active(now) {
			user.Subscription.CancelledAt = now
		}
		return nil
	})
}

func (app *application) setSession(w http.ResponseWriter, token string, age int) {
	http.SetCookie(w, &http.Cookie{
		Name: "lingua_session", Value: token, Path: "/", MaxAge: age, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: app.secureCookies,
	})
}

func sessionAge() int { return int((14 * 24 * time.Hour).Seconds()) }

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("lingua_session"); err == nil {
		if err := app.store.update(func(data *state) error {
			delete(data.Sessions, digest(cookie.Value))
			return nil
		}); err != nil {
			fail(w, err)
			return
		}
	}
	app.setSession(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) openSession(r *http.Request, data *state, user user, now time.Time) (string, error) {
	token, err := randomID()
	if err != nil {
		return "", err
	}
	if old, err := r.Cookie("lingua_session"); err == nil {
		delete(data.Sessions, digest(old.Value))
	}
	for key, session := range data.Sessions {
		if !session.ExpiresAt.After(now) {
			delete(data.Sessions, key)
		}
	}
	data.Sessions[digest(token)] = session{user.ID, now.Add(time.Duration(sessionAge()) * time.Second)}
	return token, nil
}

func (app *application) register(w http.ResponseWriter, r *http.Request) {
	if !app.allowAuth(w, r, "register", 6) {
		return
	}
	var form struct{ Name, Email, Password string }
	if !readJSON(w, r, &form) {
		return
	}
	name, err := validateName(form.Name)
	if err != nil {
		fail(w, err)
		return
	}
	email, err := validateEmail(form.Email)
	if err != nil {
		fail(w, err)
		return
	}
	if len([]rune(form.Password)) < 8 || len(form.Password) > 256 {
		fail(w, invalid("Пароль должен содержать от 8 символов и занимать до 256 байт."))
		return
	}
	password, err := hashPassword(form.Password)
	if err != nil {
		fail(w, err)
		return
	}
	id, err := randomID()
	if err != nil {
		fail(w, err)
		return
	}
	user := user{ID: id, Name: name, Email: email, Password: password}
	now := time.Now()
	var token string
	err = app.store.update(func(data *state) error {
		if _, found := data.userByEmail(email); found {
			return conflict("Профиль с этой почтой уже существует. Войдите в него.")
		}
		data.Users[id] = user
		token, err = app.openSession(r, data, user, now)
		return err
	})
	if err != nil {
		fail(w, err)
		return
	}
	app.setSession(w, token, sessionAge())
	writeJSON(w, http.StatusCreated, map[string]any{"account": accountView{Profile: publicProfile(user), Bookings: []booking{}}})
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	if !app.allowAuth(w, r, "login", 12) {
		return
	}
	var form struct{ Email, Password string }
	if !readJSON(w, r, &form) {
		return
	}
	var user user
	var found bool
	app.store.read(func(data *state) {
		user, found = data.userByEmail(strings.ToLower(strings.TrimSpace(form.Email)))
	})
	valid := verifyPassword(form.Password, user.Password)
	if !found || !valid {
		problem(w, http.StatusUnauthorized, "Почта или пароль указаны неверно.")
		return
	}
	now := time.Now()
	var token string
	var view accountView
	err := app.store.update(func(data *state) error {
		current := data.Users[user.ID]
		view = data.view(current, now)
		var err error
		token, err = app.openSession(r, data, current, now)
		return err
	})
	if err != nil {
		fail(w, err)
		return
	}
	app.setSession(w, token, sessionAge())
	writeJSON(w, http.StatusOK, map[string]any{"account": view})
}
