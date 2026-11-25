package main

import (
	"errors"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

type learningProgress struct {
	UserID       string    `json:"userId,omitempty"`
	ModuleID     string    `json:"moduleId"`
	Prepared     bool      `json:"prepared"`
	Completed    bool      `json:"completed"`
	Feedback     string    `json:"feedback,omitempty"`
	NextModuleID string    `json:"nextModuleId,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func progressKey(userID, moduleID string) string { return userID + ":" + moduleID }

func (app *application) prepare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := app.school.module(id); !ok {
		problem(w, http.StatusNotFound, "Модуль не найден.")
		return
	}
	var form struct {
		Prepared bool `json:"prepared"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	app.changeAccount(w, r, func(data *state, u *user, now time.Time) error {
		key := progressKey(u.ID, id)
		p := data.Progress[key]
		p.UserID, p.ModuleID, p.Prepared, p.UpdatedAt = u.ID, id, form.Prepared, now
		data.Progress[key] = p
		return nil
	})
}

func setupTeacher(data *store, school school) error {
	email := os.Getenv("LINGUA_TEACHER_EMAIL")
	if email == "" {
		return nil
	}
	email, err := validateEmail(email)
	if err != nil {
		return err
	}
	var existing user
	data.read(func(data *state) { existing, _ = data.userByEmail(email) })
	if existing.ID != "" {
		if existing.Teacher {
			return nil
		}
		return errors.New("teacher email is already registered as a student")
	}
	password, err := secret("LINGUA_TEACHER_PASSWORD")
	if err != nil {
		return err
	}
	if len([]rune(password)) < 12 || len(password) > 256 {
		return errors.New("set LINGUA_TEACHER_PASSWORD or LINGUA_TEACHER_PASSWORD_FILE with a password of at least 12 characters")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	return data.update(func(data *state) error {
		for _, user := range data.Users {
			if user.Teacher {
				return errors.New("a teacher is already configured")
			}
		}
		data.Users[id] = user{ID: id, Name: school.Teacher.Name, Email: email, Password: hash, Teacher: true}
		return nil
	})
}

func secret(key string) (string, error) {
	if path := os.Getenv(key + "_FILE"); path != "" {
		data, err := os.ReadFile(path)
		return strings.TrimRight(string(data), "\r\n"), err
	}
	return os.Getenv(key), nil
}

func (app *application) calendar(data *state, now time.Time) []slot {
	slots := []slot{}
	if !app.catalog.demo || !data.ScheduleEnabled {
		slots = app.catalog.slots(now)
	}
	if app.catalog.demo && !data.ScheduleEnabled {
		for i, slot := range slots {
			if slot.Format != "group" {
				continue
			}
			eligible := []module{}
			for _, item := range app.school.Modules {
				if item.Language == slot.Language {
					eligible = append(eligible, item)
				}
			}
			if len(eligible) > 0 {
				slots[i].ModuleID = eligible[slot.StartsAt.Day()%len(eligible)].ID
			}
		}
	}
	for _, item := range data.Slots {
		if item.StartsAt.After(now) {
			slots = append(slots, item)
		}
	}
	slices.SortFunc(slots, func(a, b slot) int {
		if order := a.StartsAt.Compare(b.StartsAt); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return slots
}

func (app *application) teacherChange(w http.ResponseWriter, r *http.Request, apply func(*state, time.Time) error) bool {
	now := time.Now()
	err := app.store.update(func(data *state) error {
		person, err := data.user(r, now)
		if err != nil {
			return err
		}
		if !person.Teacher {
			return requestError{http.StatusForbidden, "Доступ только для преподавателя."}
		}
		return apply(data, now)
	})
	if err != nil {
		fail(w, err)
		return false
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}

type teachingBooking struct {
	Booking  booking          `json:"booking"`
	Student  string           `json:"student"`
	Progress learningProgress `json:"progress"`
}

func (app *application) teaching(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var rejection error
	slots, bookings := []slot{}, []teachingBooking{}
	app.store.read(func(data *state) {
		person, err := data.user(r, now)
		if err != nil {
			rejection = err
			return
		}
		if !person.Teacher {
			rejection = requestError{http.StatusForbidden, "Доступ только для преподавателя."}
			return
		}
		for _, item := range app.calendar(data, now) {
			if !strings.HasPrefix(item.ID, "demo-") {
				slots = append(slots, item)
			}
		}
		for _, record := range data.Bookings {
			if record.Cancelled {
				continue
			}
			p := data.Progress[progressKey(record.UserID, record.ModuleID)]
			p.UserID = ""
			student := data.Users[record.UserID]
			record.UserID, record.SubscriptionID = "", ""
			bookings = append(bookings, teachingBooking{record, student.Name, p})
		}
		slices.SortFunc(bookings, func(a, b teachingBooking) int { return b.Booking.Slot.StartsAt.Compare(a.Booking.Slot.StartsAt) })
	})
	if rejection != nil {
		fail(w, rejection)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots, "bookings": bookings})
}

func (app *application) saveSlot(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Language   string    `json:"language"`
		Format     string    `json:"format"`
		ModuleID   string    `json:"moduleId"`
		StartsAt   time.Time `json:"startsAt"`
		Duration   int       `json:"duration"`
		Capacity   int       `json:"capacity"`
		MeetingURL string    `json:"meetingUrl"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	id := r.PathValue("id")
	creating := id == ""
	if creating {
		var err error
		id, err = randomID()
		if err != nil {
			fail(w, err)
			return
		}
	}
	next := slot{ID: id, Language: form.Language, Format: form.Format, ModuleID: form.ModuleID,
		StartsAt: form.StartsAt, Duration: form.Duration, Capacity: form.Capacity, MeetingURL: strings.TrimSpace(form.MeetingURL), Managed: true}
	app.teacherChange(w, r, func(data *state, now time.Time) error {
		if !isLanguage(next.Language) || !next.StartsAt.After(now) || next.StartsAt.After(now.AddDate(1, 0, 0)) ||
			next.Duration < 10 || next.Duration > 180 || !validHTTPS(next.MeetingURL) {
			return invalid("Проверьте дату, длительность и HTTPS-ссылку на встречу.")
		}
		switch next.Format {
		case "trial", "individual":
			if next.Capacity != 1 {
				return invalid("Для индивидуальной встречи нужно одно место.")
			}
		case "group":
			if next.Capacity < 2 || next.Capacity > 5 || next.ModuleID == "" {
				return invalid("Для мини-группы выберите модуль и от 2 до 5 мест.")
			}
		default:
			return invalid("Выберите формат занятия.")
		}
		if next.ModuleID != "" {
			item, ok := app.school.module(next.ModuleID)
			if !ok || item.Language != next.Language {
				return invalid("Выберите модуль для этого языка.")
			}
		}
		if !creating {
			previous, found := data.Slots[id]
			if !found || !previous.StartsAt.After(now) {
				return requestError{http.StatusNotFound, "Занятие недоступно для изменения."}
			}
		}
		for _, existing := range app.calendar(data, now) {
			if existing.ID != id && !strings.HasPrefix(existing.ID, "demo-") && overlaps(next, existing) {
				return conflict("У преподавателя уже есть занятие в это время.")
			}
		}
		for bookingID, item := range data.Bookings {
			if item.Cancelled || !overlaps(next, item.Slot) {
				continue
			}
			if item.Slot.ID != id {
				return conflict("На это время у преподавателя уже есть подтверждённая запись.")
			}
			current := item.Slot
			if !current.StartsAt.Equal(next.StartsAt) || current.Duration != next.Duration ||
				current.Capacity != next.Capacity || current.ModuleID != next.ModuleID ||
				current.Language != next.Language || current.Format != next.Format {
				return conflict("Для занятия с учениками можно изменить ссылку. Чтобы поменять дату или тему, отмените занятие и создайте новое.")
			}
			item.Slot.MeetingURL = next.MeetingURL
			data.Bookings[bookingID] = item
		}
		for _, item := range data.Bookings {
			if !item.Cancelled && item.Slot.ID == id && !overlaps(next, item.Slot) {
				return conflict("Для занятия с учениками можно изменить только ссылку.")
			}
		}
		data.Slots[id] = next
		data.ScheduleEnabled = true
		return nil
	})
}

func overlaps(a, b slot) bool {
	return a.StartsAt.Before(b.endsAt()) && b.StartsAt.Before(a.endsAt())
}

func (app *application) removeSlot(w http.ResponseWriter, r *http.Request) {
	type recipient struct {
		email  string
		record booking
	}
	var recipients []recipient
	success := app.teacherChange(w, r, func(data *state, now time.Time) error {
		id := r.PathValue("id")
		lesson, found := data.Slots[id]
		if !found || !lesson.StartsAt.After(now) {
			return requestError{http.StatusNotFound, "Занятие недоступно для отмены."}
		}
		for bookingID, record := range data.Bookings {
			if record.Slot.ID != id || record.Cancelled {
				continue
			}
			student := data.Users[record.UserID]
			data.Users[student.ID] = creditBooking(student, record, now)
			record.Cancelled = true
			record.Slot.MeetingURL = ""
			data.Bookings[bookingID] = record
			recipients = append(recipients, recipient{student.Email, record})
		}
		delete(data.Slots, id)
		return nil
	})
	if success {
		for _, student := range recipients {
			app.sendBookingMail(student.email, student.record, "Занятие отменено преподавателем")
		}
	}
}

func (app *application) feedback(w http.ResponseWriter, r *http.Request) {
	var form struct {
		ModuleID     string `json:"moduleId"`
		Feedback     string `json:"feedback"`
		NextModuleID string `json:"nextModuleId"`
		Completed    bool   `json:"completed"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	app.teacherChange(w, r, func(data *state, now time.Time) error {
		record, ok := data.Bookings[r.PathValue("id")]
		if !ok || record.Cancelled || record.Slot.StartsAt.After(now) {
			return invalid("Обратная связь доступна после начала занятия.")
		}
		item, ok := app.school.module(form.ModuleID)
		if !ok || item.Language != record.Slot.Language || record.ModuleID != "" && record.ModuleID != form.ModuleID {
			return invalid("Выберите модуль занятия.")
		}
		if form.NextModuleID != "" {
			next, found := app.school.module(form.NextModuleID)
			if !found || next.Language != item.Language || next.ID == item.ID {
				return invalid("Выберите другой модуль этого языка.")
			}
		}
		if len([]rune(form.Feedback)) > 900 {
			return invalid("Комментарий должен быть короче 900 символов.")
		}
		key := progressKey(record.UserID, item.ID)
		p := data.Progress[key]
		p.UserID, p.ModuleID, p.Completed, p.UpdatedAt = record.UserID, item.ID, form.Completed, now
		p.Feedback, p.NextModuleID = strings.TrimSpace(form.Feedback), form.NextModuleID
		data.Progress[key] = p
		record.ModuleID = item.ID
		data.Bookings[record.ID] = record
		return nil
	})
}
