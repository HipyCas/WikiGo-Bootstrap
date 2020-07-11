package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"
)

var templates = template.Must(template.ParseFiles("tmpl/login.html", "tmpl/register.html", "tmpl/edit.html", "tmpl/view.html", "tmpl/sub/cdn.html", "tmpl/sub/meta.html", "tmpl/sub/alerts.html"))

var pagePath = regexp.MustCompile("^/(view|edit|save|download)/([a-zA-Z0-9]+)$")

type ViewData struct {
	Alerts      []Alert
	CurrentUser *User
}

type PageViewData struct {
	Alerts      []Alert
	CurrentUser *User
	WikiPage    *Page
}

func renderTemplate(w http.ResponseWriter, tmpl string) {
	err := templates.ExecuteTemplate(w, tmpl+".html", ViewData{Alerts: getAlerts(), CurrentUser: &currentUser})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPageTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	view := PageViewData{Alerts: getAlerts(), CurrentUser: &currentUser, WikiPage: p}
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
	return
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
		file, err := os.Open("user/" + r.Form["username"][0] + ".xml")
		if err != nil {
			log.Printf("Error while openng user/%s.xml: %v", r.Form["username"][0], err)
			addAlertCreate(5, "Username not found")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/login/", http.StatusFound)
			return
		}
		defer file.Close()
		data, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("Error while reading data from user/%s.xml: %v", r.Form["username"][0], err)
			addAlertCreate(5, "Internal error with login")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/login/", http.StatusInternalServerError)
			return
		}
		er := xml.Unmarshal(data, &currentUser)
		if er != nil {
			log.Printf("Error while parsing xml user/%s.xml: %v", r.Form["username"][0], er)
			addAlertCreate(5, "Internal error with login")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/login/", http.StatusInternalServerError)
			return
		}
		addAlertCreate(3, "Succesful login")
		addAlertCreate(2, "Logged in as user: "+currentUser.Username)
		if utf8.RuneCountInString(r.Form["password"][0]) < 5 {
			addAlertCreate(4, "Your password is too weak")
		}
		//log.Printf("Created alert: %s", a.Msg)
		log.Print(currentUser)
		http.Redirect(w, r, "/view/TestPage", http.StatusFound)
	}
}

func registerHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		renderTemplate(w, "register")
	} else {
		r.ParseForm()
		if r.Form.Get("password") != r.Form.Get("passwordRepeat") {
			log.Printf("The passwords %s and %s do not match", r.Form.Get("password"), r.Form.Get("repeatPassword"))
			addAlertCreate(5, "The passwords do match")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusFound)
		}
		valid, message := isValidPassword(r.Form.Get("password"))
		if !valid {
			log.Printf("The password %s is not valid: %s", r.Form["password"][0], message)
			addAlertCreate(5, message)
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusFound)
			return
		}
		_, err := os.Open("user/" + r.Form["username"][0] + ".xml")
		if err == nil {
			log.Printf("Username %s already exists: %v", r.Form.Get("username"), err)
			addAlertCreate(5, "Username not available")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusFound)
			return
		}
		user := User{Username: r.Form.Get("username") /*FirstName: r.Form.Get("firstName"), LastName: r.Form.Get("lastName"),*/, Password: r.Form.Get("password"), Email: r.Form.Get("email") /*, PhoneNumber: r.Form.Get("phoneNumber"), Country: r.Form.Get("country")*/}
		out, err := xml.MarshalIndent(user, " ", "\t")
		if err != nil {
			log.Printf("Error when generating XML for user %v: %v", user, err)
			addAlertCreate(5, "Internal server error while registering")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusInternalServerError)
			return
		}
		_, err = os.Stdout.Write([]byte(xml.Header))
		if err != nil {
			log.Printf("Error when writing XML header to console for user %v: %v", user, err)
			addAlertCreate(5, "Internal server error while registering")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusInternalServerError)
			return
		}
		_, err = os.Stdout.Write(out)
		if err != nil {
			log.Printf("Error when writing XML to console for user %v: %v", user, err)
			addAlertCreate(5, "Internal server error while registering")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusInternalServerError)
			return
		}
		err = ioutil.WriteFile("user/"+user.Username+".xml", out, os.ModePerm)
		if err != nil {
			log.Printf("Error when wsaving XML to file user/%s.xml for user %v: %v", user.Username, user, err)
			addAlertCreate(5, "Internal server error while registering")
			r.Header.Set("Method", "GET")
			http.Redirect(w, r, "/register/", http.StatusInternalServerError)
			return
		}
		addAlertCreate(3, "Succesfully created account!")
		addAlertCreate(2, "Login now to access it")
		http.Redirect(w, r, "/login/", http.StatusFound)
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
	http.HandleFunc("/register/", registerHandle)
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
		log.Printf("Got alert of msg: %s and level %d", alerts[i].Msg, alerts[i].Level)
		toReturn = append(toReturn, alerts[i])
	}
	for j := 0; j < len(alerts); j++ {
		removeAlert(j)
	}
	return toReturn
}

func removeAlert(index int) {
	alerts[index] = alerts[len(alerts)-1]
	alerts = alerts[:len(alerts)-1]
}

var currentUser User = User{}

type User struct {
	XMLName      xml.Name `xml:"user"`
	Username     string   `xml:"username"`
	Password     string   `xml:"password"`
	FirstName    string   `xml:"firstName"`
	LastName     string   `xml:"lastName"`
	Email        string   `xml:"email"`
	PhoneNumber  string   `xml:"phoneNumber"`
	Address      Address  `xml:"address"`
	Language     string   `xml:"language"`
	LanguageCode string   `xml:"languageCode"`
	Country      string   `xml:"country"`
	CountryCode  string   `xml:"countryCode"`
	About        []byte   `xml:"about"`
	Config       Config   `xml:"config"`
	Other        []byte   `xml:",any"`
	Comments     []byte   `xml:",comments"`
}

type Address struct {
	Number   string `xml:"number"`
	Street   string `xml:"streer"`
	City     string `xml:"city"`
	Zip      string `xml:"zip"`
	Province string `xml:"province"`
	Country  string `xml:"country"`
}

type Config struct{}

func isValidPassword(password string) (ok bool, msg string) {
	var (
		upp, low, num bool
		tot           int
	)
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			upp = true
			tot++
		case unicode.IsLower(char):
			low = true
			tot++
		case unicode.IsDigit(char):
			num = true
			tot++
			/*
				case unicode.IsSymbol(char):
					sym = true
					tot++
				default:
					return false, "Invalid character on password (only unicode characaters accepted)"*/
		}
	}

	if !upp {
		msg = "Password must include at least one uppercase letter"
		ok = false
	} else if !low {
		msg = "Password must include at least one lowercase letter"
		ok = false
	} else if !num {
		msg = "Password must include at least one digit/number"
		ok = false
		/*} else if !sym {
		msg = "Password must include at least one symbol"
		ok = false*/
	} else if tot < 8 {
		msg = "Password must be longer than 8"
		ok = false
	} else {
		msg = ""
		ok = true
	}
	return
}
