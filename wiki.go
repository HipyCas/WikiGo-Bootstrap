package main

import (
  "fmt"
  "regexp"
  "io/ioutil"
  "log"
  "net/http"
  "html/template"
)

var templates = template.Must(template.ParseFiles("tmpl/login.html", "tmpl/edit.html", "tmpl/view.html", "tmpl/sub/cdn.html", "tmpl/sub/meta.html", "tmpl/sub/alerts.html"))

var pagePath = regexp.MustCompile("^/(view|edit|save)/([a-zA-Z0-9]+)$")

type ViewData struct {
  Alerts []Alert
}

type PageViewData struct {
  Alerts []Alert
  WikiPage *Page
}

func renderTemplate(w http.ResponseWriter, tmpl string) {
  err := templates.ExecuteTemplate(w, tmpl+".html", ViewData{Alerts: getAlerts()})
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
  }
}

func renderPageTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl + ".html", PageViewData{Alerts: getAlerts(), WikiPage: p})
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

func viewHandler(w http.ResponseWriter, r *http.Request, title string)  {
	p, err := loadPage(title)
  if err != nil {
    http.Redirect(w, r, "/edit/" + title, http.StatusFound)
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
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
  //log.Println("Page " + p.title + " succesfully")
  http.Redirect(w, r, "/view/" + title, http.StatusFound)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
  if r.Method == "GET" {
    renderTemplate(w, "login")
  } else {
    r.ParseForm()
    if m, _ := regexp.MatchString("^[a-zA-Z0-9]+$", r.Form.Get("username")); !m {
      fmt.Println("Username: ", template.HTMLEscapeString(r.Form["username"][0]))
    }
    if n, _ := regexp.MatchString("^[a-zA-Z0-9$%&]+$", r.Form.Get("password")); !n {
      fmt.Println("Password: ", template.HTMLEscapeString(r.Form["password"][0]))
    }
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

func main()  {
  http.HandleFunc("/view/", makePageHandler(viewHandler))
  http.HandleFunc("/edit/", makePageHandler(editHandler))
  http.HandleFunc("/save/", makePageHandler(saveHandler))
  http.HandleFunc("/login/", loginHandler)
  log.Fatal(http.ListenAndServe(":8080", nil))
}

type Page struct {
  Title string
  Body []byte
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
  Level int
  Msg string
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
  if lvl < 0 || lvl > 7 {
    return Alert{Level: -1, Msg: ""}, &InvalidAlertLevel{Lvl: lvl}
  }
  a := Alert{Level: lvl, Msg: msg}
  alerts = append(alerts, a)
  return a, nil
}

func getAlerts() []Alert {
  toReturn := []Alert{}
  for i := 0; i < len(alerts); i++ {
    if alerts[i].Level == -1 {
      break
    }
    toReturn = append(toReturn, alerts[i])
    alerts[i] = Alert{-1, ""}
  }
  return toReturn
}
