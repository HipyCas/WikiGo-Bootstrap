package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"
)

var templates = template.Must(template.ParseFiles("tmpl/login.html", "tmpl/edit.html", "tmpl/view.html", "tmpl/sub/cdn.html", "tmpl/sub/meta.html", "tmpl/sub/alerts.html"))

var pagePath = regexp.MustCompile("^/(view|edit|save|download)/([a-zA-Z0-9]+)$")

type ViewData struct {
	Alerts []Alert
}

type PageViewData struct {
	Alerts   []Alert
	WikiPage *Page
}

func renderTemplate(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", ViewData{Alerts: getAlerts()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPageTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	view := PageViewData{Alerts: getAlerts(), WikiPage: p}
	err := templates.ExecuteTemplate(w, tmpl+".html", view)
	log.Println(view)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		//return
	}
}

/*
func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
  m := pagePath.FindStringSubmatch(r.URL.Path)
  if m == nil {
    http.NotFound(w, r)
    return "", errors.New("Invalid Page Title")
  }
  return m[2], nil // The title is the second subexpression
}*/

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderPageTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderPageTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		addAlertCreate(5, "Something went wrong and the page wasn't saved")
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		//return
	} else {
		addAlertCreate(3, "Page succesfully saved")
	}
	//log.Println("Page " + p.title + " succesfully")
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func downloadHandler(w http.ResponseWriter, r *http.Request, title string) {
	w.Header().Add("Content-Disposition", "Attachement")
	p, _ := loadPage(title)
	http.ServeContent(w, r, title, time.Now(), bytes.NewReader(p.Body))
	log.Printf("Downloaded page %s as txt", p.Title)
	http.Redirect(w, r, "/view/"+title, http.StatusOK)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		renderTemplate(w, "login")
	} else {
		r.ParseForm()
		if m, _ := regexp.MatchString("^[a-zA-Z0-9]+$", r.Form.Get("username")); !m {
			fmt.Println("Username: ", template.HTMLEscapeString(r.Form["username"][0]))
		}
		if m, _ := regexp.MatchString("^[a-zA-Z0-9$%&]+$", r.Form.Get("password")); !m {
			fmt.Println("Password: ", template.HTMLEscapeString(r.Form["password"][0]))
		}
		addAlertCreate(3, "Succesful login")
		addAlertCreate(2, "Logged in as user: "+r.Form["username"][0])
		if utf8.RuneCountInString(r.Form["password"][0]) < 5 {
			addAlertCreate(4, "Your password is too weak")
		}
		//log.Printf("Created alert: %s", a.Msg)
		http.Redirect(w, r, "/view/TestPage", http.StatusFound)
	}
}

func makePageHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := pagePath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	http.HandleFunc("/view/", makePageHandler(viewHandler))
	http.HandleFunc("/edit/", makePageHandler(editHandler))
	http.HandleFunc("/save/", makePageHandler(saveHandler))
	http.HandleFunc("/download/", makePageHandler(downloadHandler))
	http.HandleFunc("/login/", loginHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := "page/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

type Alert struct {
	Level     int
	Primary   bool
	Secondary bool
	Info      bool
	Success   bool
	Warning   bool
	Danger    bool
	Light     bool
	Dark      bool
	Msg       string
}

var alerts []Alert

type InvalidAlertLevel struct {
	Lvl int
}

func (e *InvalidAlertLevel) Error() string {
	return fmt.Sprintf("Invalid alert level: %d", e.Lvl)
}

func addAlert(alert Alert) error {
	if alert.Level < 0 || alert.Level > 7 {
		return &InvalidAlertLevel{Lvl: alert.Level}
	}
	alerts = append(alerts, alert)
	return nil
}

func addAlertCreate(lvl int, msg string) (Alert, error) {
	a := Alert{Level: lvl, Msg: msg}
	switch lvl {
	case 0:
		a.Primary = true
	case 1:
		a.Secondary = true
	case 2:
		a.Info = true
	case 3:
		a.Success = true
	case 4:
		a.Warning = true
	case 5:
		a.Danger = true
	case 6:
		a.Light = true
	case 7:
		a.Dark = true
	default:
		return Alert{}, &InvalidAlertLevel{Lvl: lvl}
	}
	alerts = append(alerts, a)
	return a, nil
}

func getAlerts() []Alert {
	toReturn := []Alert{}
	for i := 0; i < len(alerts); i++ {
		if alerts[i].Level == -1 {
			break
		}
		log.Printf("Got alert of msg: %s and level %d", alerts[i].Msg, alerts[i].Level)
		toReturn = append(toReturn, alerts[i])
		removeAlert(i)
	}
	return toReturn
}

func removeAlert(index int) {
	alerts[index] = alerts[len(alerts)-1]
	alerts = alerts[:len(alerts)-1]
}
