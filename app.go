package main

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/Sirupsen/logrus"
	"github.com/chloearianne/protestpulse/db"
	"github.com/chloearianne/protestpulse/session"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// App bundles resources used by the application.
type App struct {
	db          *db.Database
	templateMap map[string]*template.Template
	cookieStore *sessions.CookieStore
	loginState  bool
}

// AppConfig is a container for all app configuration parameters
// that are to be extracted from the YAML config file.
type AppConfig struct {
	CookieKey string    `yaml:"cookie_key"`
	DBConfig  db.Config `yaml:"db_config"`
}

func main() {
	// Load the .env file for environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Load AppConfig
	c := loadConfig(fmt.Sprintf(os.Getenv("CONFIG_PATH")))

	// Set up the database
	ppdb := db.New(c.DBConfig)
	defer ppdb.Close()

	// Create App object
	app := App{
		db:          ppdb,
		cookieStore: sessions.NewCookieStore([]byte(c.CookieKey)),
		templateMap: getTemplateMap(),
	}

	// Register types to be stored on session
	gob.Register(map[string]interface{}{})
	gob.Register(&session.Profile{})

	// Set up routes
	r := mux.NewRouter()
	// Handle authentication.
	r.HandleFunc("/auth/logout", app.LogoutHandler)
	r.HandleFunc("/auth/login", app.LoginHandler)
	r.HandleFunc("/auth/callback", app.CallbackHandler)
	// Handle app routes.
	r.HandleFunc("/", app.IndexGET).Methods("GET")
	r.HandleFunc("/events", app.EventsGET).Methods("GET")
	r.HandleFunc("/events", app.EventsPOST).Methods("POST")
	r.HandleFunc("/events/{id}", app.EventGET).Methods("GET")

	// Set up middleware stack
	n := negroni.New(
		negroni.NewRecovery(),
		negroni.HandlerFunc(app.IsAuthenticated),
		negroni.NewStatic(http.Dir("public")),
	)
	n.UseHandler(handlers.LoggingHandler(os.Stdout, r))
	n.Run(":" + os.Getenv("PORT"))
}

// renderTemplate is a wrapper around template.ExecuteTemplate.
func (a *App) renderTemplate(w http.ResponseWriter, r *http.Request, filename string, data map[string]interface{}) {
	// Ensure the template exists in the map.
	tmpl, ok := a.templateMap[filename]
	if !ok {
		err := fmt.Errorf("The template %s does not exist.", filename)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add loginState to data
	data["LoggedIn"] = a.loginState

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		logrus.WithError(err).Error("Failed to ExecuteTemplate")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getTemplateMap generates a map of template file name to complete templates.
// If any failures occur when compiling the templates, a fatal error will be logged.
func getTemplateMap() map[string]*template.Template {
	var templateMap = make(map[string]*template.Template)
	templates, err := filepath.Glob("public/templates/*.tmpl")
	if err != nil {
		logrus.Fatal(err)
	}
	layouts, err := filepath.Glob("public/layouts/*.tmpl")
	if err != nil {
		logrus.Fatal(err)
	}
	for _, tmpl := range templates {
		files := append(layouts, tmpl)
		templateMap[filepath.Base(tmpl)] = template.Must(template.ParseFiles(files...))
	}

	return templateMap
}

// loadConfig extracts the configuration file into an AppConfig object.
func loadConfig(path string) *AppConfig {
	if _, err := os.Stat(path); err != nil {
		logrus.WithField("path", path).WithError(err).Fatal("Could not find config file")
	}
	logrus.Infof("Using config file at %q", path)
	config, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Fatal(err)
	}

	c := &AppConfig{}
	err = yaml.Unmarshal([]byte(config), c)
	if err != nil {
		logrus.Fatal(err)
	}

	return c
}
