package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
)

func AddToCart(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
            Quantity  int `json:"quantity"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Check if item already in cart
        var existingID int
        err := db.QueryRow("SELECT id FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID).Scan(&existingID)
        if err == nil {
            // Update existing
            db.Exec("UPDATE carts SET quantity = quantity + ? WHERE user_id = ? AND product_id = ?", req.Quantity, userID, req.ProductID)
        } else {
            // Insert new
            db.Exec("INSERT INTO carts (user_id, product_id, quantity) VALUES (?, ?, ?)", userID, req.ProductID, req.Quantity)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "added to cart"})
    }
}

func GetCart(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT c.product_id, p.title, p.price, c.quantity
            FROM carts c
            JOIN products p ON c.product_id = p.id
            WHERE c.user_id = ?
        `, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var items []map[string]interface{}
        var total float64
        for rows.Next() {
            var productID, quantity int
            var title string
            var price float64
            rows.Scan(&productID, &title, &price, &quantity)
            items = append(items, map[string]interface{}{
                "product_id": productID,
                "title":      title,
                "price":      price,
                "quantity":   quantity,
                "subtotal":   price * float64(quantity),
            })
            total += price * float64(quantity)
        }

        response := map[string]interface{}{
            "items": items,
            "total": total,
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func Checkout(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            JournalNumber string `json:"journal_number"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Get cart total
        var total float64
        db.QueryRow(`
            SELECT COALESCE(SUM(p.price * c.quantity), 0)
            FROM carts c
            JOIN products p ON c.product_id = p.id
            WHERE c.user_id = ?
        `, userID).Scan(&total)

        if total == 0 {
            http.Error(w, "Cart is empty", 400)
            return
        }

        // Create order
        result, err := db.Exec(`
            INSERT INTO orders (buyer_id, total, payment_journal_no, verified, status)
            VALUES (?, ?, ?, 0, 'pending')
        `, userID, total, req.JournalNumber)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        orderID, _ := result.LastInsertId()

        // Clear cart
        db.Exec("DELETE FROM carts WHERE user_id = ?", userID)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":   "order placed",
            "order_id": orderID,
        })
    }
}
