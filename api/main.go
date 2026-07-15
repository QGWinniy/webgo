package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
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
	HtmlPath    string   `json:"html_path"`
}

var db *sql.DB

func main() {
	var err error

	connStr := os.Getenv("DB_URL")
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/product/", getProductById)
	http.HandleFunc("/products", productGet)
	http.HandleFunc("/product/get_name/id", getNameById)
	// http.HandleFunc("/products/create", productPost)

	log.Println("server started on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func productGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. берём товары
	rows, err := db.Query("SELECT id, name, price, description, html_path FROM products")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var products []Product

	for rows.Next() {
		var p Product

		err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Description, &p.HtmlPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		
		imgRows, err := db.Query("SELECT url FROM product_images WHERE product_id = $1", p.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		var images []string
		for imgRows.Next() {
			var url string
			imgRows.Scan(&url)
			images = append(images, url)
		}
		imgRows.Close()

		p.Images = images

		products = append(products, p)
	}

	json.NewEncoder(w).Encode(products)
}

func productPost(w http.ResponseWriter, r *http.Request) {
	var p Product
	json.NewDecoder(r.Body).Decode(&p)

	err := db.QueryRow(
		"INSERT INTO products (name, price) VALUES ($1, $2) RETURNING id",
		p.Name,
		p.Price,
	).Scan(&p.ID)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(p)
}

func getProductById(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := strings.TrimPrefix(r.URL.Path, "/product/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	var p Product

	err = db.QueryRow(`
		SELECT id, name, price, description, html_path
		FROM products
		WHERE id = $1
	`, id).Scan(
		&p.ID,
		&p.Name,
		&p.Price,
		&p.Description,
		&p.HtmlPath,
	)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rows, err := db.Query(`
		SELECT url FROM product_images WHERE product_id = $1
	`, id)
	if err == nil {
		defer rows.Close()

		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err == nil {
				p.Images = append(p.Images, url)
			}
		}
	}

	json.NewEncoder(w).Encode(p)
}

func getNameById(w http.ResponseWriter, r *http.Request) {
	var id int
	err := json.NewDecoder(r.Body).Decode(&id)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var name string

	err = db.QueryRow(
		"SELECT name FROM products WHERE id = $1",
		id,
	).Scan(&name)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(name)
}
