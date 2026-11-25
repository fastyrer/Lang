package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type user struct {
	ID           string
	Name         string
	Email        string
	Password     string
	Language     string
	Level        string
	Subscription *subscription
	Teacher      bool
}

type subscription struct {
	ID          string
	PlanID      string
	Remaining   int
	EndsAt      time.Time
	CancelledAt time.Time
}

func (sub *subscription) active(now time.Time) bool {
	return sub != nil && sub.CancelledAt.IsZero() && sub.EndsAt.After(now)
}

type booking struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId,omitempty"`
	Slot           slot      `json:"slot"`
	SubscriptionID string    `json:"subscriptionId,omitempty"`
	ModuleID       string    `json:"moduleId,omitempty"`
	Cancelled      bool      `json:"cancelled,omitempty"`
	CancelBefore   time.Time `json:"cancelBefore,omitzero"`
}

type session struct {
	UserID    string
	ExpiresAt time.Time
}

type state struct {
	Users           map[string]user
	Sessions        map[string]session
	Bookings        map[string]booking
	Slots           map[string]slot
	ScheduleEnabled bool
	Progress        map[string]learningProgress
	Resets          map[string]passwordReset
	Orders          map[string]order
}

type store struct {
	mu   sync.RWMutex
	path string
	data state
}

func openStore(path string) (*store, error) {
	s := &store{path: path, data: state{Users: map[string]user{}, Sessions: map[string]session{}, Bookings: map[string]booking{}}}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.data.initialize()
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(content, &s.data); err != nil {
		return nil, err
	}
	if s.data.Users == nil || s.data.Sessions == nil || s.data.Bookings == nil {
		return nil, errors.New("invalid data file")
	}
	s.data.initialize()
	return s, nil
}

func (s *store) read(read func(*state)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	read(&s.data)
}

func (s *store) update(update func(*state) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.data
	next.Users = maps.Clone(next.Users)
	next.Sessions = maps.Clone(next.Sessions)
	next.Bookings = maps.Clone(next.Bookings)
	next.Slots = maps.Clone(next.Slots)
	next.Progress = maps.Clone(next.Progress)
	next.Resets = maps.Clone(next.Resets)
	next.Orders = maps.Clone(next.Orders)
	if err := update(&next); err != nil {
		return err
	}
	if err := saveState(s.path, next); err != nil {
		return err
	}
	s.data = next
	return nil
}

func saveState(path string, data state) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".lingua-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := json.NewEncoder(temp).Encode(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func (s *state) user(r *http.Request, now time.Time) (user, error) {
	cookie, err := r.Cookie("lingua_session")
	if err != nil {
		return user{}, loginRequired
	}
	current, ok := s.Sessions[digest(cookie.Value)]
	if !ok || !current.ExpiresAt.After(now) {
		return user{}, loginRequired
	}
	person, ok := s.Users[current.UserID]
	if !ok {
		return user{}, loginRequired
	}
	if person.Subscription != nil {
		copy := *person.Subscription
		person.Subscription = &copy
	}
	return person, nil
}

func (s *state) userByEmail(email string) (user, bool) {
	for _, candidate := range s.Users {
		if candidate.Email == email {
			return candidate, true
		}
	}
	return user{}, false
}

type profile struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Language string `json:"language"`
	Level    string `json:"level"`
	Teacher  bool   `json:"teacher"`
}

func publicProfile(user user) profile {
	return profile{user.Name, user.Email, user.Language, user.Level, user.Teacher}
}

type subscriptionView struct {
	PlanID    string    `json:"planId"`
	Remaining int       `json:"remaining"`
	EndsAt    time.Time `json:"endsAt"`
	Active    bool      `json:"active"`
	Cancelled bool      `json:"cancelled"`
}

type accountView struct {
	Profile      profile            `json:"profile"`
	Subscription *subscriptionView  `json:"subscription"`
	Bookings     []booking          `json:"bookings"`
	Progress     []learningProgress `json:"progress"`
	PendingOrder *orderView         `json:"pendingOrder,omitempty"`
}

func (s *state) view(user user, now time.Time) accountView {
	view := accountView{Profile: publicProfile(user), Bookings: []booking{}, Progress: []learningProgress{}}
	if sub := user.Subscription; sub != nil {
		view.Subscription = &subscriptionView{sub.PlanID, sub.Remaining, sub.EndsAt, sub.active(now), !sub.CancelledAt.IsZero()}
	}
	for _, own := range s.Bookings {
		if own.UserID == user.ID {
			own.UserID, own.SubscriptionID = "", ""
			if own.Cancelled || !own.Slot.endsAt().After(now) {
				own.Slot.MeetingURL = ""
			}
			view.Bookings = append(view.Bookings, own)
		}
	}
	for _, progress := range s.Progress {
		if progress.UserID == user.ID {
			progress.UserID = ""
			view.Progress = append(view.Progress, progress)
		}
	}
	slices.SortFunc(view.Progress, func(a, b learningProgress) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	for _, pending := range s.Orders {
		if pending.UserID == user.ID && pending.pending() {
			status := pending.public()
			view.PendingOrder = &status
			break
		}
	}
	slices.SortFunc(view.Bookings, func(a, b booking) int { return a.Slot.StartsAt.Compare(b.Slot.StartsAt) })
	return view
}

const passwordRounds = 600_000

func randomID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func digest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordRounds, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key), nil
}

func verifyPassword(password, saved string) bool {
	if password == "" || len(password) > 256 {
		return false
	}
	saltText, keyText, found := strings.Cut(saved, ":")
	salt, saltErr := hex.DecodeString(saltText)
	expected, keyErr := hex.DecodeString(keyText)
	if !found || saltErr != nil || keyErr != nil || len(salt) != 16 || len(expected) != 32 {
		salt, expected = make([]byte, 16), make([]byte, 32)
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, passwordRounds, 32)
	return err == nil && subtle.ConstantTimeCompare(actual, expected) == 1
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if count := utf8.RuneCountInString(name); count < 2 || count > 60 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", invalid("Имя должно содержать от 2 до 60 символов.")
	}
	return name, nil
}

func validateEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || len(email) > 254 || address.Address != email {
		return "", invalid("Укажите корректный адрес почты.")
	}
	return email, nil
}

// initialize adds collections introduced after the first release.
func (s *state) initialize() {
	if s.Slots == nil {
		s.Slots = map[string]slot{}
	}
	if s.Progress == nil {
		s.Progress = map[string]learningProgress{}
	}
	if s.Resets == nil {
		s.Resets = map[string]passwordReset{}
	}
	if s.Orders == nil {
		s.Orders = map[string]order{}
	}
}
