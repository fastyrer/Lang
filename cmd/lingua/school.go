package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
)

type school struct {
	Teacher     teacherProfile `json:"teacher"`
	Books       []book         `json:"books"`
	Modules     []module       `json:"modules"`
	SupportURL  string         `json:"supportUrl,omitempty"`
	PrivacyURL  string         `json:"privacyUrl,omitempty"`
	TermsURL    string         `json:"termsUrl,omitempty"`
	CancelHours int            `json:"cancelHours"`
	Prices      map[string]int `json:"prices,omitempty"`
	Sample      bool           `json:"sample"`
}

type teacherProfile struct {
	Name      string   `json:"name"`
	Bio       string   `json:"bio"`
	PhotoURL  string   `json:"photoUrl,omitempty"`
	Languages []string `json:"languages"`
}

type book struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Details  string `json:"details"`
	URL      string `json:"url,omitempty"`
}

type module struct {
	ID          string   `json:"id"`
	BookID      string   `json:"bookId"`
	Language    string   `json:"language"`
	Title       string   `json:"title"`
	Level       string   `json:"level"`
	Chapter     string   `json:"chapter"`
	Outcome     string   `json:"outcome"`
	Preparation string   `json:"preparation"`
	Minutes     int      `json:"minutes"`
	MaterialURL string   `json:"materialUrl,omitempty"`
	Requires    []string `json:"requires,omitempty"`
	Open        bool     `json:"open"`
}

func defaultSchool() school {
	return school{
		Teacher: teacherProfile{
			Name:      "Преподаватель Lingua",
			Bio:       "Один преподаватель ведёт английский и китайский. На пробном уроке вместе выберете учебник и тему, с которой удобно начать.",
			Languages: []string{"english", "chinese"},
		},
		Books: []book{
			{"en-sample", "english", "Английский · пример программы", "Знакомство, повседневные разговоры и самостоятельная подготовка.", ""},
			{"zh-sample", "chinese", "Китайский · пример программы", "Произношение, первые диалоги и чтение пиньиня.", ""},
		},
		Modules: []module{
			{ID: "en-intro", BookID: "en-sample", Language: "english", Title: "Знакомство", Level: "beginner", Chapter: "Глава 1", Outcome: "Представиться и задать три вопроса собеседнику.", Preparation: "Запишите три фразы о себе и составьте вопросы с what, where и how.", Minutes: 20, Open: true},
			{ID: "en-daily", BookID: "en-sample", Language: "english", Title: "Распорядок дня", Level: "elementary", Chapter: "Глава 2", Outcome: "Рассказать, как проходит ваш день, и спросить о привычках.", Preparation: "Составьте пять предложений о буднях. Прочитайте их вслух.", Minutes: 25},
			{ID: "zh-tones", BookID: "zh-sample", Language: "chinese", Title: "Пиньинь и тоны", Level: "beginner", Chapter: "Глава 1", Outcome: "Различать четыре тона и прочитать первые слоги в пиньине.", Preparation: "Возьмите слог ma, попробуйте произнести его с четырьмя тонами и запишите, какие даются сложнее.", Minutes: 20, Open: true},
			{ID: "zh-intro", BookID: "zh-sample", Language: "chinese", Title: "Первые диалоги", Level: "beginner", Chapter: "Глава 2", Outcome: "Поздороваться, спросить имя и ответить на тот же вопрос.", Preparation: "Подготовьте короткий диалог из приветствия, вопроса об имени и ответа.", Minutes: 25, Requires: []string{"zh-tones"}},
		},
		CancelHours: 8, Sample: true,
	}
}

func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

func loadSchool(path string) (school, error) {
	if path == "" {
		return defaultSchool(), nil
	}
	var school school
	b, err := os.ReadFile(path)
	if err != nil {
		return school, err
	}
	if err := json.Unmarshal(b, &school); err != nil {
		return school, err
	}
	if school.Teacher.PhotoURL != "" && (!strings.HasPrefix(school.Teacher.PhotoURL, "/images/") || strings.Contains(school.Teacher.PhotoURL, "..")) {
		return school, fmt.Errorf("invalid teacher photo path")
	}
	if school.Teacher.Name == "" || len(school.Books) == 0 || len(school.Modules) == 0 ||
		school.CancelHours < 0 || school.CancelHours > 168 {
		return school, fmt.Errorf("invalid school profile or cancellation terms")
	}
	if school.PrivacyURL != "" && !validHTTPS(school.PrivacyURL) || school.TermsURL != "" && !validHTTPS(school.TermsURL) ||
		school.SupportURL != "" && !validHTTPS(school.SupportURL) && !strings.HasPrefix(school.SupportURL, "mailto:") {
		return school, fmt.Errorf("invalid school links")
	}
	books := map[string]book{}
	for _, book := range school.Books {
		if book.ID == "" || books[book.ID].ID != "" || !isLanguage(book.Language) || book.Title == "" ||
			book.URL != "" && !validHTTPS(book.URL) {
			return school, fmt.Errorf("invalid school book %q", book.ID)
		}
		books[book.ID] = book
	}
	seen := map[string]bool{}
	for _, module := range school.Modules {
		if module.ID == "" || seen[module.ID] || !isLanguage(module.Language) || books[module.BookID].Language != module.Language ||
			!isLevel(module.Level) || module.Title == "" || module.Outcome == "" || module.Chapter == "" || module.Minutes < 1 ||
			module.MaterialURL != "" && !validHTTPS(module.MaterialURL) {
			return school, fmt.Errorf("invalid school module %q", module.ID)
		}
		seen[module.ID] = true
	}
	for _, module := range school.Modules {
		for _, id := range module.Requires {
			if target, found := school.module(id); !found || id == module.ID || target.Language != module.Language {
				return school, fmt.Errorf("invalid prerequisite %q for module %q", id, module.ID)
			}
		}
	}
	for id, price := range school.Prices {
		if _, ok := findPlan(id); !ok || price < 1 || price > 1_000_000 {
			return school, fmt.Errorf("invalid price for %q", id)
		}
	}
	return school, nil
}

func isLevel(level string) bool {
	return level == "beginner" || level == "elementary" || level == "intermediate"
}

func (school school) module(id string) (module, bool) {
	for _, module := range school.Modules {
		if module.ID == id {
			return module, true
		}
	}
	return module{}, false
}

func (app *application) publicPlans() []plan {
	available := slices.Clone(plans)
	for i := range available {
		if price, ok := app.school.Prices[available[i].ID]; ok {
			available[i].Price = price
		}
	}
	return available
}

func (app *application) cancellationDeadline(slot slot) time.Time {
	return slot.StartsAt.Add(-time.Duration(app.school.CancelHours) * time.Hour)
}
