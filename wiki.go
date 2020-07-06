package main

import (
  "fmt"
  "regexp"
  "io/ioutil"
  "log"
  "net/http"
  "errors"
  "html/template"
)

var pageTemplates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html", "tmpl/sub/cdn.html", "tmpl/sub/meta.html"))
var otherTemplates = template.Must(template.ParseFiles("tmpl/login.html"))

var pagePath = regexp.MustCompile("^/(view|edit|save)/([a-zA-Z0-9]+)$")

func renderTemplate(w http.ResponseWriter, tmpl string) {
  err := otherTemplates.ExecuteTemplate(w, tmpl+".html", nil)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
  }
}

func renderPageTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := pageTemplates.ExecuteTemplate(w, tmpl + ".html", p)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    //return
  }
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
  m := pagePath.FindStringSubmatch(r.URL.Path)
  if m == nil {
    http.NotFound(w, r)
    return "", errors.New("Invalid Page Title")
  }
  return m[2], nil // The title is the second subexpression
}

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

func loginHandler(w http.ResponseWriter, r *http.Request, title string) {
  if r.Method == "GET" {
    renderTemplate(w, "login")
  } else {
    r.ParseForm()
    fmt.Println("Username: ", r.Form["username"])
    fmt.Println("Password: ", r.Form["password"])
  }
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
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
  http.HandleFunc("/view/", makeHandler(viewHandler))
  http.HandleFunc("/edit/", makeHandler(editHandler))
  http.HandleFunc("/save/", makeHandler(saveHandler))
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
