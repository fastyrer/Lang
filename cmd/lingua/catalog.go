package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"time"
)

type plan struct {
	ID      string `json:"id"`
	Format  string `json:"format"`
	Name    string `json:"name"`
	Lessons int    `json:"lessons"`
	Days    int    `json:"days"`
	Price   int    `json:"price"`
}

var plans = []plan{
	{ID: "group", Format: "group", Name: "Мини-группа", Lessons: 4, Days: 30, Price: 3600},
	{ID: "individual", Format: "individual", Name: "Индивидуально", Lessons: 4, Days: 30, Price: 6400},
}

func findPlan(id string) (plan, bool) {
	for _, plan := range plans {
		if plan.ID == id {
			return plan, true
		}
	}
	return plan{}, false
}

type slot struct {
	ID         string    `json:"id"`
	Language   string    `json:"language"`
	Format     string    `json:"format"`
	StartsAt   time.Time `json:"startsAt"`
	Duration   int       `json:"duration"`
	Capacity   int       `json:"capacity"`
	Available  int       `json:"available"`
	MeetingURL string    `json:"meetingUrl,omitempty"`
	ModuleID   string    `json:"moduleId,omitempty"`
	Managed    bool      `json:"managed,omitempty"`
}

func (slot slot) endsAt() time.Time {
	return slot.StartsAt.Add(time.Duration(slot.Duration) * time.Minute)
}

type catalog struct {
	demo      bool
	scheduled []slot
}

func isLanguage(language string) bool {
	return language == "english" || language == "chinese"
}

func loadCatalog(path string) (catalog, error) {
	if path == "" {
		return catalog{demo: true}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return catalog{}, err
	}
	var slots []slot
	if err := json.Unmarshal(content, &slots); err != nil {
		return catalog{}, err
	}
	seen := map[string]bool{}
	for _, slot := range slots {
		link, err := url.Parse(slot.MeetingURL)
		validCapacity := slot.Capacity == 1 && (slot.Format == "trial" || slot.Format == "individual") ||
			slot.Format == "group" && slot.Capacity >= 2 && slot.Capacity <= 5
		if slot.ID == "" || seen[slot.ID] || !isLanguage(slot.Language) || !validCapacity ||
			slot.Duration < 1 || slot.Duration > 180 || slot.StartsAt.IsZero() || err != nil ||
			link.Scheme != "https" || link.Hostname() == "" || link.User != nil {
			return catalog{}, fmt.Errorf("invalid schedule slot %q", slot.ID)
		}
		seen[slot.ID] = true
	}
	return catalog{scheduled: slots}, nil
}

func (cat catalog) slots(now time.Time) []slot {
	available := []slot{}
	if cat.demo {
		available = demoSlots(now)
	} else {
		for _, slot := range cat.scheduled {
			if slot.StartsAt.After(now) {
				available = append(available, slot)
			}
		}
	}
	slices.SortFunc(available, func(a, b slot) int { return a.StartsAt.Compare(b.StartsAt) })
	return available
}

func (cat catalog) findSlot(id string, now time.Time) (slot, bool) {
	for _, slot := range cat.slots(now) {
		if slot.ID == id {
			return slot, true
		}
	}
	return slot{}, false
}

var moscow = time.FixedZone("MSK", 3*60*60)

func demoSlots(now time.Time) []slot {
	templates := []struct {
		language, format         string
		hour, duration, capacity int
	}{
		{"english", "trial", 10, 25, 1},
		{"chinese", "trial", 11, 25, 1},
		{"english", "individual", 12, 55, 1},
		{"chinese", "individual", 14, 55, 1},
		{"english", "group", 17, 55, 5},
		{"chinese", "group", 19, 55, 5},
	}
	now = now.In(moscow)
	slots := []slot{}
	for offset := 0; offset < 56; offset++ {
		day := now.AddDate(0, 0, offset)
		if day.Weekday() == time.Sunday {
			continue
		}
		for i, template := range templates {
			start := time.Date(day.Year(), day.Month(), day.Day(), template.hour, 0, 0, 0, moscow)
			if start.After(now) {
				slots = append(slots, slot{
					ID:       fmt.Sprintf("demo-%s-%d", start.Format("20060102"), i),
					Language: template.language, Format: template.format, StartsAt: start,
					Duration: template.duration, Capacity: template.capacity,
				})
			}
		}
	}
	return slots
}
