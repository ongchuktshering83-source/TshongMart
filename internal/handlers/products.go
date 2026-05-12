package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
)

func GetAllProducts(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rows, err := db.Query(`
            SELECT p.id, p.title, p.description, p.price, p.condition, 
                   COALESCE(p.whatsapp_link, ''), COALESCE(p.telegram_link, ''),
                   COALESCE(u.full_name, 'Unknown')
            FROM products p
            LEFT JOIN users u ON p.seller_id = u.id
            ORDER BY p.created_at DESC
        `)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var products []map[string]interface{}
        for rows.Next() {
            var id int
            var title, description, condition, whatsapp, telegram, sellerName string
            var price float64
            rows.Scan(&id, &title, &description, &price, &condition, &whatsapp, &telegram, &sellerName)

            products = append(products, map[string]interface{}{
                "id":          id,
                "title":       title,
                "description": description,
                "price":       price,
                "condition":   condition,
                "whatsapp":    whatsapp,
                "telegram":    telegram,
                "seller":      sellerName,
            })
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(products)
    }
}

func GetProductByID(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        id := vars["id"]

        var product = make(map[string]interface{})
        var title, description, condition, whatsapp, telegram, sellerName string
        var price float64
        var productID int

        err := db.QueryRow(`
            SELECT p.id, p.title, p.description, p.price, p.condition, 
                   COALESCE(p.whatsapp_link, ''), COALESCE(p.telegram_link, ''), 
                   COALESCE(u.full_name, 'Unknown')
            FROM products p
            LEFT JOIN users u ON p.seller_id = u.id
            WHERE p.id = ?
        `, id).Scan(&productID, &title, &description, &price, &condition, &whatsapp, &telegram, &sellerName)

        if err != nil {
            http.Error(w, "Product not found", 404)
            return
        }

        product["id"] = productID
        product["title"] = title
        product["description"] = description
        product["price"] = price
        product["condition"] = condition
        product["whatsapp"] = whatsapp
        product["telegram"] = telegram
        product["seller"] = sellerName

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(product)
    }
}

func CreateProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var product struct {
            Title        string  `json:"title"`
            Description  string  `json:"description"`
            Price        float64 `json:"price"`
            Condition    string  `json:"condition"`
            WhatsappLink string  `json:"whatsapp_link"`
            TelegramLink string  `json:"telegram_link"`
        }

        json.NewDecoder(r.Body).Decode(&product)

        _, err := db.Exec(`
            INSERT INTO products (seller_id, title, description, price, condition, whatsapp_link, telegram_link)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `, userID, product.Title, product.Description, product.Price, product.Condition, product.WhatsappLink, product.TelegramLink)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product created"})
    }
}
