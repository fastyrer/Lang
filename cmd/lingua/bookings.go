package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (app *application) book(w http.ResponseWriter, r *http.Request) {
	app.saveBooking(w, r, "")
}

func (app *application) reschedule(w http.ResponseWriter, r *http.Request) {
	app.saveBooking(w, r, r.PathValue("id"))
}

func (app *application) saveBooking(w http.ResponseWriter, r *http.Request, bookingID string) {
	var form struct {
		SlotID   string `json:"slotId"`
		ModuleID string `json:"moduleId"`
	}
	if !readJSON(w, r, &form) {
		return
	}
	var email string
	var record booking
	success := app.changeAccount(w, r, func(data *state, u *user, now time.Time) error {
		var lesson slot
		for _, next := range app.calendar(data, now) {
			if next.ID == form.SlotID {
				lesson = next
				break
			}
		}
		if lesson.ID == "" {
			return conflict("Это время уже недоступно. Выберите другое занятие.")
		}
		if lesson.ModuleID != "" && form.ModuleID != "" && lesson.ModuleID != form.ModuleID {
			return invalid("На выбранное время запланирован другой модуль.")
		}
		moduleID := form.ModuleID
		if lesson.ModuleID != "" {
			moduleID = lesson.ModuleID
		}
		if moduleID != "" {
			item, ok := app.school.module(moduleID)
			if !ok || item.Language != lesson.Language {
				return invalid("Выберите модуль для этого языка.")
			}
		}
		if bookingID != "" {
			old, ok := data.Bookings[bookingID]
			if !ok || old.UserID != u.ID || old.Cancelled {
				return requestError{http.StatusNotFound, "Запись не найдена."}
			}
			if !now.Before(old.changeDeadline()) {
				return conflict("Срок самостоятельного переноса прошёл. Свяжитесь с преподавателем.")
			}
			if old.Slot.Language != lesson.Language || old.Slot.Format != lesson.Format ||
				old.ModuleID != "" && old.ModuleID != moduleID {
				return conflict("Выберите занятие того же формата, языка и модуля.")
			}
			if old.Slot.ID == lesson.ID {
				return conflict("Вы уже записаны на это время.")
			}
			record = old
		}
		seats := 0
		for _, item := range data.Bookings {
			if item.Cancelled || item.ID == bookingID {
				continue
			}
			if item.Slot.ID == lesson.ID {
				seats++
			}
			if item.Slot.ID != lesson.ID && overlaps(item.Slot, lesson) {
				return conflict("У преподавателя уже есть подтверждённое занятие в это время.")
			}
			if item.UserID != u.ID {
				continue
			}
			if item.Slot.ID == lesson.ID {
				return conflict("Вы уже записаны на это занятие.")
			}
			if lesson.Format == "trial" && item.Slot.Format == "trial" {
				return conflict("Пробное занятие доступно один раз.")
			}
		}
		if seats >= lesson.Capacity {
			return conflict("Все места уже заняты. Выберите другое время.")
		}
		if lesson.Format != "trial" {
			sub := u.Subscription
			if !sub.active(now) {
				return conflict("Для записи нужен действующий пакет занятий.")
			}
			plan, _ := findPlan(sub.PlanID)
			if plan.Format != lesson.Format || lesson.endsAt().After(sub.EndsAt) {
				return conflict("Проверьте формат и срок действия пакета.")
			}
			if bookingID != "" {
				if record.SubscriptionID != sub.ID {
					return conflict("Запись оплачена другим пакетом. Свяжитесь с преподавателем.")
				}
			} else {
				if sub.Remaining < 1 {
					return conflict("В пакете закончились занятия.")
				}
				sub.Remaining--
				record.SubscriptionID = sub.ID
			}
		}
		if record.ID == "" {
			id, err := randomID()
			if err != nil {
				return err
			}
			record.ID, record.UserID = id, u.ID
		}
		record.Slot, record.ModuleID = lesson, moduleID
		record.CancelBefore = app.cancellationDeadline(lesson)
		data.Bookings[record.ID] = record
		email = u.Email
		return nil
	})
	if !success {
		return
	}
	title := "Запись на занятие подтверждена"
	if bookingID != "" {
		title = "Время занятия изменилось"
	}
	app.sendBookingMail(email, record, title)
}

func (item booking) changeDeadline() time.Time {
	if item.CancelBefore.IsZero() {
		return item.Slot.StartsAt
	}
	return item.CancelBefore
}

func creditBooking(u user, record booking, now time.Time) user {
	if sub := u.Subscription; sub.active(now) && sub.ID == record.SubscriptionID {
		copy := *sub
		copy.Remaining++
		u.Subscription = &copy
	}
	return u
}

func (app *application) cancelBooking(w http.ResponseWriter, r *http.Request) {
	var record booking
	var email string
	success := app.changeAccount(w, r, func(data *state, u *user, now time.Time) error {
		id := r.PathValue("id")
		var ok bool
		record, ok = data.Bookings[id]
		if !ok || record.UserID != u.ID || record.Cancelled {
			return requestError{http.StatusNotFound, "Запись не найдена."}
		}
		if !now.Before(record.changeDeadline()) {
			return conflict("Срок самостоятельной отмены прошёл. Свяжитесь с преподавателем.")
		}
		*u = creditBooking(*u, record, now)
		email = u.Email
		delete(data.Bookings, id)
		return nil
	})
	if success {
		app.sendBookingMail(email, record, "Запись на занятие отменена")
	}
}

func (app *application) sendBookingMail(email string, record booking, title string) {
	if app.mail == nil {
		return
	}
	message := fmt.Sprintf("%s\nДата и время по Москве: %s. Продолжительность: %d мин.\n",
		title, record.Slot.StartsAt.In(moscow).Format("02.01.2006 15:04"), record.Slot.Duration)
	if item, ok := app.school.module(record.ModuleID); ok {
		message += "Модуль: " + strings.TrimSpace(item.Title) + "\n"
	}
	message += "Личный кабинет: " + app.publicURL + "/cabinet"
	app.mail.queue(email, title, message)
}
