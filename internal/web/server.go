package web

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/pronzzz/zenmonitor/internal/config"
	"github.com/pronzzz/zenmonitor/internal/monitor"
	"github.com/pronzzz/zenmonitor/internal/store"
)

type Server struct {
	Store *store.SQLiteStore
	Cfg   *config.Config
	Tmpl  *template.Template
}

type PageData struct {
	Now      time.Time
	Monitors []MonitorView
}

type MonitorView struct {
	Name    string
	IsUp    bool
	History []monitor.CheckResult
}

func NewHandler(st *store.SQLiteStore, cfg *config.Config) http.Handler {
	// Parse template
	tmplPath := filepath.Join("web", "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("Error parsing template (might trigger on first request if failing here): %v", err)
	}

	s := &Server{
		Store: st,
		Cfg:   cfg,
		Tmpl:  tmpl,
	}

	mux := http.NewServeMux()
	
	// Static files
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Main page
	mux.HandleFunc("/", s.handleIndex)

	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
    if s.Tmpl == nil {
        tmplPath := filepath.Join("web", "templates", "index.html")
        var err error
        s.Tmpl, err = template.ParseFiles(tmplPath)
        if err != nil {
            http.Error(w, "Template error: "+err.Error(), 500)
            return
        }
    }

	// Gather data
	var views []MonitorView
	for _, m := range s.Cfg.Monitors {
		// Get last 90 checks
		history, err := s.Store.GetHistory(m.Name, 90)
		if err != nil {
			log.Printf("Error fetching history for %s: %v", m.Name, err)
			continue
		}

		// Determine current status (latest check)
		isUp := false
		if len(history) > 0 {
			// history is reversed (oldest first) in store.go
			isUp = history[len(history)-1].Status
		}

		views = append(views, MonitorView{
			Name:    m.Name,
			IsUp:    isUp,
			History: history,
		})
	}

	data := PageData{
		Now:      time.Now(),
		Monitors: views,
	}

	if err := s.Tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}
