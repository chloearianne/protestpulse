package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/chloearianne/protestpulse/session"
	"github.com/gorilla/mux"
)

var humanDateFormat = "Jan 02, 2006"

// IndexGET handles GET requests for '/'.
func (a *App) IndexGET(w http.ResponseWriter, r *http.Request) {
	p, err := session.GetProfile(r, a.cookieStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data := map[string]interface{}{
		"Page":    "Home",
		"Profile": p,
	}
	a.renderTemplate(w, r, "index.tmpl", data)
}

// EventsPOST handles POST requests for '/events'.
func (a *App) EventsPOST(w http.ResponseWriter, r *http.Request) {
	p, err := session.GetProfile(r, a.cookieStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	r.ParseForm()
	var dateTimeFormat = "2006-01-02 15:04"
	startDateTime := fmt.Sprintf("%s %s", r.FormValue("start_date"), r.FormValue("start_time"))
	startTS, err := time.Parse(dateTimeFormat, startDateTime)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse date time")
	}
	endDateTime := fmt.Sprintf("%s %s", r.FormValue("end_date"), r.FormValue("end_time"))
	endTS, err := time.Parse(dateTimeFormat, endDateTime)
	if err != nil {
		logrus.WithError(err).Error("Failed to parse date time")
	}

	query := `INSERT INTO event (
		        creator_id, title, start_timestamp,
		        end_timestamp, description, event_topic,
		        event_type, location, user_count
			  )
			  VALUES (
			  	$1, $2, $3,
			  	$4, $5, $6,
			  	$7, $8, $9
			  )`
	_, err = a.db.Exec(query,
		p.UserID, r.FormValue("title"), startTS,
		endTS, r.FormValue("description"), r.FormValue("event_topic"),
		r.FormValue("event_type"), r.FormValue("location"), 0,
	)
	if err != nil {
		logrus.WithError(err).Error("Failed to save event")
	}

	a.EventsGET(w, r)
	return
}

// Event contains the metadata related to an activism event.
type Event struct {
	ID        int
	Title     string
	Timestamp string
}

// EventsGET handles GET requests for '/events'.
func (a *App) EventsGET(w http.ResponseWriter, r *http.Request) {
	p, err := session.GetProfile(r, a.cookieStore)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	query := `SELECT
				id, title, start_timestamp
			FROM event
			WHERE creator_id = $1`
	rows, err := a.db.Query(query, p.UserID)
	if err != nil {
		logrus.Error(err)
	}
	defer rows.Close()

	eventsMap := []Event{}
	var id int
	var title string
	var startTS time.Time
	for rows.Next() {
		if err := rows.Scan(&id, &title, &startTS); err != nil {
			logrus.Error(err)
		}
		eventsMap = append(eventsMap, Event{
			ID:        id,
			Title:     title,
			Timestamp: startTS.Format(humanDateFormat),
		})
	}

	data := map[string]interface{}{
		"Page":   "Events",
		"Events": eventsMap,
	}
	a.renderTemplate(w, r, "events.tmpl", data)
}

// EventGET handles GET requests for a single event at '/events/{id}'.
func (a *App) EventGET(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var title, desc, location string
	var startTime, endTime time.Time
	var eventType, topic int

	query := `SELECT
				title, start_timestamp, end_timestamp,
				description, event_type, event_topic,
				location
			FROM event
			WHERE id = $1`
	err := a.db.QueryRow(query, id).Scan(
		&title, &startTime, &endTime,
		&desc, &eventType, &topic,
		&location,
	)
	if err != nil {
		logrus.Error(err)
	}

	data := map[string]interface{}{
		"Page":     "Events",
		"Title":    title,
		"Start":    startTime.Format(humanDateFormat),
		"End":      endTime.Format(humanDateFormat),
		"Desc":     desc,
		"Type":     eventType,
		"Topic":    topic,
		"Location": location,
	}
	a.renderTemplate(w, r, "event.tmpl", data)
}
