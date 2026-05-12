package handlers

import (
    "database/sql"
    "encoding/json"  // Add this line
    "net/http"
)

func DashboardHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        _ = userID // Mark as used

        // Get user info for dashboard
        var user struct {
            ID       int
            Email    string
            FullName string
            Role     string
        }
        
        err := db.QueryRow(`
            SELECT id, email, COALESCE(full_name, ''), COALESCE(role, 'buyer')
            FROM users WHERE id = ?
        `, userID).Scan(&user.ID, &user.Email, &user.FullName, &user.Role)
        
        if err != nil {
            http.Error(w, "User not found", 404)
            return
        }

        w.Header().Set("Content-Type", "text/html")
        http.ServeFile(w, r, "./views/dashboard.html")
    }
}

func GetUserOrders(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT id, total, payment_journal_no, verified, status, created_at
            FROM orders
            WHERE buyer_id = ?
            ORDER BY created_at DESC
        `, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var orders []map[string]interface{}
        for rows.Next() {
            var id int
            var total float64
            var journalNo string
            var verified int
            var status string
            var createdAt string
            rows.Scan(&id, &total, &journalNo, &verified, &status, &createdAt)
            
            orders = append(orders, map[string]interface{}{
                "id":          id,
                "total":       total,
                "journal_no":  journalNo,
                "verified":    verified == 1,
                "status":      status,
                "created_at":  createdAt,
            })
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(orders)
    }
}

func GetSellerProducts(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT id, title, description, price, condition, created_at
            FROM products
            WHERE seller_id = ?
            ORDER BY created_at DESC
        `, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var products []map[string]interface{}
        for rows.Next() {
            var id int
            var title, description, condition, createdAt string
            var price float64
            rows.Scan(&id, &title, &description, &price, &condition, &createdAt)
            
            products = append(products, map[string]interface{}{
                "id":          id,
                "title":       title,
                "description": description,
                "price":       price,
                "condition":   condition,
                "created_at":  createdAt,
            })
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(products)
    }
}
