package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

type Product struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Price       int      `json:"price"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
	// HtmlPath    string   `json:"html_path"`
}

type CartItem struct {
	Product Product
	Qty     int
	Sum     int
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func dict(values ...interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := 0; i < len(values); i += 2 {
		key := values[i].(string)
		m[key] = values[i+1]
	}
	return m
}

func handler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://api:3000/products")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	counts := make(map[int]int)

	if cookie, err := r.Cookie("cart"); err == nil && cookie.Value != "" {
		ids := strings.Split(cookie.Value, ",")

		for _, idStr := range ids {
			id, err := strconv.Atoi(idStr)
			if err == nil {
				counts[id]++
			}
		}
	}

	data := struct {
		Products []Product
		Counts   map[int]int
	}{
		Products: products,
		Counts:   counts,
	}

	tmpl, err := template.New("index.html").
		Funcs(template.FuncMap{
			"json": toJSON,
			"dict": dict,
		}).
		ParseFiles(
			"templates/index.html",
			"templates/product_card.html",
		)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Println("TEMPLATE ERROR:", err)
		http.Error(w, err.Error(), 500)
	}
}

func cartHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("cart")
	if err != nil || cookie.Value == "" {
		tmpl := template.Must(template.ParseFiles("templates/cart.html"))
		tmpl.Execute(w, nil)
		return
	}

	ids := strings.Split(cookie.Value, ",")

	counts := make(map[int]int)

	for _, idStr := range ids {
		if idStr == "" {
			continue
		}

		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		counts[id]++
	}

	resp, err := http.Get("http://api:3000/products")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	var products []Product
	json.NewDecoder(resp.Body).Decode(&products)

	var cart []CartItem

	for _, p := range products {
		if qty, ok := counts[p.ID]; ok {
			cart = append(cart, CartItem{
				Product: p,
				Qty:     qty,
				Sum:     p.Price * qty,
			})
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/cart.html"))
	tmpl.Execute(w, cart)
}

func removeToCart(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id := r.Form.Get("id")
	returnURL := r.Form.Get("return")

	cookie, err := r.Cookie("cart")
	if err != nil {
		http.Redirect(w, r, returnURL, http.StatusSeeOther)
		return
	}

	arr := strings.Split(cookie.Value, ",")

	var result []string

	removed := false

	for _, el := range arr {
		if el == id && !removed {
			removed = true
			continue
		}
		result = append(result, el)
	}

	newCart := strings.Join(result, ",")

	http.SetCookie(w, &http.Cookie{
		Name:  "cart",
		Value: newCart,
		Path:  "/",
	})

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

func addToCart(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	id := r.Form.Get("id")
	qtyStr := r.Form.Get("qty")
	returnURL := r.Form.Get("return")

	qty := 1
	if qtyStr != "" {
		if v, err := strconv.Atoi(qtyStr); err == nil && v > 0 {
			qty = v
		}
	}

	cookie, err := r.Cookie("cart")
	cart := ""

	if err == nil {
		cart = cookie.Value
	}

	for i := 0; i < qty; i++ {
		if cart == "" {
			cart = id
		} else {
			cart = cart + "," + id
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "cart",
		Value: cart,
		Path:  "/",
	})

	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

func sendMail(body string) error {
	from := os.Getenv("SMTP_FROM")
	password := os.Getenv("SMTP_PASSWORD")
	to := os.Getenv("SMTP_TO")

	msg := []byte(
		"To: " + to + "\r\n" +
			"Subject: Новый заказ\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	auth := smtp.PlainAuth(
		"",
		from,
		password,
		"smtp.mail.ru",
	)

	return smtp.SendMail(
		"smtp.mail.ru:587",
		auth,
		from,
		[]string{to},
		msg,
	)
}

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	name := r.Form.Get("name")
	phone := r.Form.Get("phone")

	cookie, err := r.Cookie("cart")
	if err != nil {
		http.Error(w, "Корзина пустая", 400)
		return
	}

	order := cookie.Value

	var productsName []string

	for _, el := range strings.Split(order, ",") {
		elint, _ := strconv.Atoi(el)
		data, _ := json.Marshal(elint)
		resp, err := http.Post(
			"http://api:3000/product/get_name/id",
			"application/json",
			bytes.NewBuffer(data),
		)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		defer resp.Body.Close()
		var productName string
		json.NewDecoder(resp.Body).Decode(&productName)
		productsName = append(productsName, productName)
	}

	log.Printf(
		"НОВЫЙ ЗАКАЗ\nИмя: %s\nТелефон: %s\nТовары: %s",
		name,
		phone,
		productsName,
	)

	http.SetCookie(w, &http.Cookie{
		Name:   "cart",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	body := fmt.Sprintf(
		"Имя: %s\nТелефон: %s\nКорзина: %s",
		name,
		phone,
		productsName,
	)

	go sendMail(body)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// func productHandler(w http.ResponseWriter, r *http.Request) {
// 	id := strings.TrimPrefix(r.URL.Path, "/product/")

// 	resp, err := http.Get("http://api:3000/product/" + id)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	var product Product
// 	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}


// 	count := 0
// 	if cookie, err := r.Cookie("cart"); err == nil && cookie.Value != "" {
// 		for _, idStr := range strings.Split(cookie.Value, ",") {
// 			if idStr == strconv.Itoa(product.ID) {
// 				count++
// 			}
// 		}
// 	}

// 	relPath := strings.Trim(product.HtmlPath, "/")
// 	basePath := "./static/" + relPath

// 	htmlBytes, err := os.ReadFile(basePath + "/index.html")
// 	if err != nil {
// 		http.Error(w, "HTML not found: "+err.Error(), 500)
// 		return
// 	}

// 	baseHref := "/static/" + relPath + "/"
// 	htmlStr := string(htmlBytes)
// 	baseTag := `<base href="` + baseHref + `">`
// 	lower := strings.ToLower(htmlStr)
// 	if idx := strings.Index(lower, "<head>"); idx != -1 {
// 		insertAt := idx + len("<head>")
// 		htmlStr = htmlStr[:insertAt] + baseTag + htmlStr[insertAt:]
// 	} else {
// 		htmlStr = baseTag + htmlStr
// 	}

// 	tmpl, err := template.New("product").
// 		Funcs(template.FuncMap{
// 			"json": toJSON,
// 		}).
// 		Parse(htmlStr)
// 	if err != nil {
// 		http.Error(w, err.Error(), 500)
// 		return
// 	}

// 	data := struct {
// 		Product Product
// 		Count   int
// 	}{
// 		Product: product,
// 		Count:   count,
// 	}

// 	if err := tmpl.Execute(w, data); err != nil {
// 		http.Error(w, err.Error(), 500)
// 	}
// }

func main() {
	http.HandleFunc("/", handler)

	http.Handle("/image/",
		http.StripPrefix("/image/",
			http.FileServer(http.Dir("./image")),
		),
	)

	http.HandleFunc("/cart/add", addToCart)

	http.HandleFunc("/cart/remove", removeToCart)

	http.HandleFunc("/checkout", checkoutHandler)

	// http.HandleFunc("/product/", productHandler)

	http.HandleFunc("/cart", cartHandler)

	// http.Handle("/static/",
	// 	http.StripPrefix("/static/",
	// 		http.FileServer(http.Dir("./static")),
	// 	),
	// )

	http.ListenAndServe(":8080", nil)
}
