package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"
)

type passwordReset struct {
	UserID    string
	ExpiresAt time.Time
}

type attemptWindow struct {
	until    time.Time
	attempts int
}

type authLimiter struct {
	mu         sync.Mutex
	windows    map[string]attemptWindow
	trustProxy bool
}

func (app *application) allowAuth(w http.ResponseWriter, r *http.Request, action string, max int) bool {
	limiter := app.rate
	if limiter == nil {
		return true
	}
	address, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		address = r.RemoteAddr
	}
	if limiter.trustProxy {
		if forwarded := net.ParseIP(r.Header.Get("X-Real-IP")); forwarded != nil {
			address = forwarded.String()
		}
	}
	key := action + ":" + address
	now := time.Now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.windows) >= 4096 {
		for key, window := range limiter.windows {
			if !window.until.After(now) {
				delete(limiter.windows, key)
			}
		}
	}
	window, found := limiter.windows[key]
	if !found && len(limiter.windows) >= 4096 || window.until.After(now) && window.attempts >= max {
		w.Header().Set("Retry-After", "300")
		problem(w, http.StatusTooManyRequests, "Слишком много попыток. Повторите через несколько минут.")
		return false
	}
	if !window.until.After(now) {
		window = attemptWindow{until: now.Add(5 * time.Minute)}
	}
	window.attempts++
	limiter.windows[key] = window
	return true
}

func (app *application) recovery(w http.ResponseWriter, r *http.Request) {
	if !app.allowAuth(w, r, "recovery", 4) {
		return
	}
	if app.mail == nil {
		problem(w, http.StatusServiceUnavailable, "Восстановление по почте пока недоступно. Обратитесь в поддержку.")
		return
	}
	var form struct{ Email string }
	if !readJSON(w, r, &form) {
		return
	}
	email, err := validateEmail(form.Email)
	if err != nil {
		fail(w, err)
		return
	}
	var person user
	app.store.read(func(data *state) { person, _ = data.userByEmail(email) })
	if person.ID != "" {
		token, err := randomID()
		if err != nil {
			fail(w, err)
			return
		}
		now := time.Now()
		err = app.store.update(func(data *state) error {
			for key, entry := range data.Resets {
				if entry.UserID == person.ID || !entry.ExpiresAt.After(now) {
					delete(data.Resets, key)
				}
			}
			data.Resets[digest(token)] = passwordReset{person.ID, now.Add(30 * time.Minute)}
			return nil
		})
		if err != nil {
			fail(w, err)
			return
		}
		app.mail.queue(email, "Восстановление доступа к Lingua",
			"Чтобы установить новый пароль, откройте ссылку. Она действует 30 минут.\n"+app.publicURL+"/cabinet#reset="+token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

func (app *application) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !app.allowAuth(w, r, "reset", 8) {
		return
	}
	var form struct{ Token, Password string }
	if !readJSON(w, r, &form) {
		return
	}
	if len([]rune(form.Password)) < 8 || len(form.Password) > 256 {
		fail(w, invalid("Новый пароль должен содержать от 8 символов."))
		return
	}
	key := digest(form.Token)
	var valid bool
	app.store.read(func(data *state) { valid = data.Resets[key].ExpiresAt.After(time.Now()) })
	if !valid {
		fail(w, invalid("Ссылка недействительна или истекла. Запросите новую."))
		return
	}
	hash, err := hashPassword(form.Password)
	if err != nil {
		fail(w, err)
		return
	}
	err = app.store.update(func(data *state) error {
		reset, ok := data.Resets[key]
		person, found := data.Users[reset.UserID]
		if !ok || !found || !reset.ExpiresAt.After(time.Now()) {
			return invalid("Ссылка недействительна или истекла. Запросите новую.")
		}
		person.Password = hash
		data.Users[person.ID] = person
		delete(data.Resets, key)
		for id, session := range data.Sessions {
			if session.UserID == person.ID {
				delete(data.Sessions, id)
			}
		}
		return nil
	})
	if err != nil {
		fail(w, err)
		return
	}
	app.setSession(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

type mailConfig struct {
	address, host, user, password, from string
	directTLS                           bool
	jobs                                chan mailJob
}

type mailJob struct{ recipient, subject, message string }

func configureMail() (*mailConfig, error) {
	address := os.Getenv("LINGUA_SMTP_ADDR")
	if address == "" {
		return nil, nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, errors.New("invalid LINGUA_SMTP_ADDR")
	}
	from, err := validateEmail(os.Getenv("LINGUA_SMTP_FROM"))
	if err != nil {
		return nil, errors.New("invalid LINGUA_SMTP_FROM")
	}
	password, err := secret("LINGUA_SMTP_PASSWORD")
	if err != nil {
		return nil, err
	}
	return &mailConfig{address: address, host: host, user: os.Getenv("LINGUA_SMTP_USER"), password: password,
		from: from, directTLS: os.Getenv("LINGUA_SMTP_TLS") == "implicit", jobs: make(chan mailJob, 32)}, nil
}

func (m *mailConfig) work() {
	for job := range m.jobs {
		if err := m.send(job); err != nil {
			log.Print("SMTP notification failed")
		}
	}
}

func (m *mailConfig) queue(to, subject, message string) {
	select {
	case m.jobs <- mailJob{to, subject, message}:
	default:
		log.Print("SMTP notification queue is full")
	}
}

func (m *mailConfig) send(job mailJob) error {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConfig := &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	if m.directTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", m.address, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", m.address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if !m.directTLS {
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if m.user != "" {
		if err := client.Auth(smtp.PlainAuth("", m.user, m.password, m.host)); err != nil {
			return err
		}
	}
	if err := client.Mail(m.from); err != nil {
		return err
	}
	if err := client.Rcpt(job.recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		m.from, job.recipient, mime.BEncoding.Encode("UTF-8", job.subject), strings.ReplaceAll(job.message, "\n", "\r\n"))
	if err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (app *application) configure() error {
	app.rate = &authLimiter{windows: map[string]attemptWindow{}, trustProxy: os.Getenv("LINGUA_TRUST_PROXY") == "1"}
	app.publicURL = strings.TrimRight(os.Getenv("LINGUA_PUBLIC_URL"), "/")
	mail, err := configureMail()
	if err != nil {
		return err
	}
	if mail != nil && !validHTTPS(app.publicURL) {
		return errors.New("SMTP requires an HTTPS LINGUA_PUBLIC_URL")
	}
	app.mail = mail
	if mail != nil {
		go mail.work()
	}
	app.demoBilling = !app.secureCookies && os.Getenv("LINGUA_DEMO_BILLING") != "0"
	shop := os.Getenv("LINGUA_YOOKASSA_SHOP_ID")
	key, err := secret("LINGUA_YOOKASSA_SECRET")
	if err != nil {
		return err
	}
	if shop != "" || key != "" {
		if shop == "" || key == "" || !app.secureCookies || !validHTTPS(app.publicURL) ||
			app.school.Sample || app.school.SupportURL == "" || app.school.TermsURL == "" || app.school.PrivacyURL == "" || len(app.school.Prices) != len(plans) {
			return errors.New("checkout requires shop credentials, HTTPS, school contacts, terms, privacy and configured prices")
		}
		app.payment = &checkout{shop: shop, key: key, base: "https://api.yookassa.ru/v3", client: &http.Client{Timeout: 12 * time.Second}}
	}
	return nil
}
