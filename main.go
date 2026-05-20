package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/gorilla/mux"
    "github.com/gorilla/sessions"
    _ "github.com/mattn/go-sqlite3"
    "golang.org/x/crypto/bcrypt"
)

var store = sessions.NewCookieStore([]byte("your-secret-key"))

func main() {
    // Create SQLite database
    db, err := sql.Open("sqlite3", "./tshongmart.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create uploads directory if not exists
    os.MkdirAll("./static/uploads", os.ModePerm)

    // Create tables
    createTables(db)

   r := mux.NewRouter()

// Static files
r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

// Page routes
r.HandleFunc("/", HomeHandler).Methods("GET")
r.HandleFunc("/products", ProductsHandler).Methods("GET")
r.HandleFunc("/register", RegisterHandler(db)).Methods("GET", "POST")
r.HandleFunc("/login", LoginHandler(db)).Methods("GET", "POST")
r.HandleFunc("/logout", LogoutHandler).Methods("POST")
r.HandleFunc("/contact", ContactHandler).Methods("GET")
r.HandleFunc("/dashboard", DashboardHandler(db)).Methods("GET")
r.HandleFunc("/profile", ProfileHandler).Methods("GET")
r.HandleFunc("/seller-dashboard", SellerDashboardHandler).Methods("GET")

// API routes - Products
r.HandleFunc("/api/products", GetProductsAPI(db)).Methods("GET")
r.HandleFunc("/api/products", CreateProduct(db)).Methods("POST")
r.HandleFunc("/api/product/{id}", GetProductAPI(db)).Methods("GET")

// API routes - Cart (Protected - with ban check)
r.HandleFunc("/api/cart", checkBanned(db, GetCartHandler(db))).Methods("GET")
r.HandleFunc("/api/cart/add", checkBanned(db, AddToCartHandler(db))).Methods("POST")
r.HandleFunc("/api/cart/update", checkBanned(db, UpdateCartQuantity(db))).Methods("PUT")
r.HandleFunc("/api/cart/remove", checkBanned(db, RemoveFromCart(db))).Methods("DELETE")
r.HandleFunc("/api/cart/save-for-later", checkBanned(db, SaveForLater(db))).Methods("POST")
r.HandleFunc("/api/saved-items", checkBanned(db, GetSavedItems(db))).Methods("GET")
r.HandleFunc("/api/move-to-cart", checkBanned(db, MoveToCart(db))).Methods("POST")

// API routes - Checkout & Orders (Protected)
r.HandleFunc("/api/checkout", checkBanned(db, CheckoutHandler(db))).Methods("POST")

// API routes - Profile (Protected)
r.HandleFunc("/api/user/profile", checkBanned(db, GetUserProfile(db))).Methods("GET")
r.HandleFunc("/api/user/profile", checkBanned(db, UpdateUserProfile(db))).Methods("PUT")
r.HandleFunc("/api/user/change-password", checkBanned(db, ChangePassword(db))).Methods("POST")
r.HandleFunc("/api/user/upload-avatar", checkBanned(db, UploadAvatarHandler(db))).Methods("POST")

// API routes - Upload (Protected)
r.HandleFunc("/api/upload", checkBanned(db, UploadImageHandler(db))).Methods("POST")

// Seller routes (Protected)
r.HandleFunc("/api/seller/products", checkBanned(db, GetSellerProducts(db))).Methods("GET")
r.HandleFunc("/api/seller/products/{id}", checkBanned(db, UpdateProduct(db))).Methods("PUT")
r.HandleFunc("/api/seller/products/{id}", checkBanned(db, DeleteProduct(db))).Methods("DELETE")
r.HandleFunc("/api/seller/products/{id}/status", checkBanned(db, UpdateProductStatus(db))).Methods("PUT")
r.HandleFunc("/api/seller/orders", checkBanned(db, GetSellerOrders(db))).Methods("GET")

// Buyer order routes (Protected)
r.HandleFunc("/api/my-orders", checkBanned(db, GetMyOrders(db))).Methods("GET")
r.HandleFunc("/api/orders/{id}/track", checkBanned(db, TrackOrder(db))).Methods("GET")
r.HandleFunc("/api/orders/{id}/cancel", checkBanned(db, CancelOrder(db))).Methods("POST")

// Messaging routes (Protected - but banned users can still message admin)
// Note: These are NOT wrapped with ban check so banned users can contact admin
r.HandleFunc("/api/messages", GetMessages(db)).Methods("GET")
r.HandleFunc("/api/messages", SendMessage(db)).Methods("POST")
r.HandleFunc("/api/messages/{id}/read", MarkMessageRead(db)).Methods("PUT")
r.HandleFunc("/api/messages/unread-count", GetUnreadCount(db)).Methods("GET")

// Report routes (Protected)
r.HandleFunc("/api/reports", checkBanned(db, CreateReport(db))).Methods("POST")

// Admin routes (No ban check for admin endpoints - admins can work even if banned? 
// Actually admins shouldn't be banned, but keep these without ban check)
r.HandleFunc("/admin", AdminHandler(db)).Methods("GET")
r.HandleFunc("/api/admin/products", AdminGetAllProducts(db)).Methods("GET")
r.HandleFunc("/api/admin/products/{id}", AdminDeleteProduct(db)).Methods("DELETE")
r.HandleFunc("/api/admin/products/{id}", AdminUpdateProduct(db)).Methods("PUT")
r.HandleFunc("/api/admin/products", AdminCreateProduct(db)).Methods("POST")
r.HandleFunc("/api/admin/users", AdminGetAllUsers(db)).Methods("GET")
r.HandleFunc("/api/admin/users/{id}/role", AdminUpdateUserRole(db)).Methods("PUT")
r.HandleFunc("/api/admin/users/{id}/reset-password", AdminResetPassword(db)).Methods("POST")
r.HandleFunc("/api/admin/users/{id}/ban", BanUser(db)).Methods("POST")
r.HandleFunc("/api/admin/users/{id}/unban", UnbanUser(db)).Methods("POST")
r.HandleFunc("/api/admin/users/{id}/send-message", AdminSendMessage(db)).Methods("POST")
r.HandleFunc("/api/admin/reports", GetReports(db)).Methods("GET")
r.HandleFunc("/api/admin/reports/{id}", UpdateReportStatus(db)).Methods("PUT")

// Inbox page
r.HandleFunc("/inbox", InboxHandler).Methods("GET")

log.Println("✅ Tshongmart Server running at http://localhost:8080")
log.Fatal(http.ListenAndServe(":8080", r))
}

func createTables(db *sql.DB) {
    usersTable := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        full_name TEXT,
        phone TEXT,
        role TEXT DEFAULT 'buyer',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`

    productsTable := `
    CREATE TABLE IF NOT EXISTS products (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        seller_id INTEGER,
        title TEXT NOT NULL,
        description TEXT,
        price REAL,
        condition TEXT,
        image_url TEXT,
        whatsapp_link TEXT,
        telegram_link TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(seller_id) REFERENCES users(id)
    );`

    cartsTable := `
    CREATE TABLE IF NOT EXISTS carts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        product_id INTEGER,
        quantity INTEGER DEFAULT 1,
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(product_id) REFERENCES products(id),
        UNIQUE(user_id, product_id)
    );`

    ordersTable := `
    CREATE TABLE IF NOT EXISTS orders (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        buyer_id INTEGER,
        total REAL,
        payment_journal_no TEXT,
        verified INTEGER DEFAULT 0,
        status TEXT DEFAULT 'pending',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(buyer_id) REFERENCES users(id)
    );`

    reviewsTable := `
    CREATE TABLE IF NOT EXISTS reviews (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        product_id INTEGER,
        reviewer_id INTEGER,
        rating INTEGER CHECK (rating >= 1 AND rating <= 5),
        comment TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(product_id) REFERENCES products(id),
        FOREIGN KEY(reviewer_id) REFERENCES users(id)
    );`

    savedForLaterTable := `
    CREATE TABLE IF NOT EXISTS saved_for_later (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER,
        product_id INTEGER,
        saved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(user_id) REFERENCES users(id),
        FOREIGN KEY(product_id) REFERENCES products(id),
        UNIQUE(user_id, product_id)
    );`

    db.Exec(usersTable)
    db.Exec(productsTable)
    db.Exec(cartsTable)
    db.Exec(ordersTable)
    db.Exec(reviewsTable)
    db.Exec(savedForLaterTable)

    // Create test accounts
    var count int
    db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    if count == 0 {
        // Hash password "password123"
        hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

        // Create seller
        db.Exec(`INSERT INTO users (email, password_hash, full_name, role) VALUES (?, ?, ?, ?)`,
            "seller@tshongmart.com", string(hashedPassword), "Demo Seller", "seller")

        // Create buyer
        db.Exec(`INSERT INTO users (email, password_hash, full_name, role) VALUES (?, ?, ?, ?)`,
            "buyer@tshongmart.com", string(hashedPassword), "Demo Buyer", "buyer")

        // Create admin
        db.Exec(`INSERT INTO users (email, password_hash, full_name, role) VALUES (?, ?, ?, ?)`,
            "admin@tshongmart.com", string(hashedPassword), "Admin", "both")
    }

    // Create sample products
    db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
    if count == 0 {
        db.Exec(`INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link) 
                 VALUES (1, 'Siemens PLC S7-1200', 'Used but in good working condition. Perfect for automation projects. Includes power supply and programming cable.', 25000, 'Good', '', 'https://wa.me/97512345678', 'https://t.me/tshongmart')`)

        db.Exec(`INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link) 
                 VALUES (1, 'Allen Bradley MicroLogix 1400', 'Second hand automation controller, includes cables and manual. Well maintained.', 35000, 'Very Good', '', 'https://wa.me/97512345678', 'https://t.me/tshongmart')`)

        db.Exec(`INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link) 
                 VALUES (1, 'Industrial Power Supply 24V 10A', 'Reliable power supply for automation systems. Perfect condition.', 5500, 'Like New', '', 'https://wa.me/97512345678', 'https://t.me/tshongmart')`)

        db.Exec(`INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link) 
                 VALUES (1, 'HMI Touch Screen 7"', 'Used HMI for industrial automation. Screen has minor scratches but works perfectly.', 18000, 'Good', '', 'https://wa.me/97512345678', 'https://t.me/tshongmart')`)

        db.Exec(`INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link) 
                 VALUES (1, 'VFD Drive 5.5kW', 'Variable frequency drive for motor control. Tested working.', 22000, 'Used', '', 'https://wa.me/97512345678', 'https://t.me/tshongmart')`)
    }
}

// Page Handlers
func HomeHandler(w http.ResponseWriter, r *http.Request) {
    // Check if user is logged in
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    
    if ok && userID > 0 {
        // User is logged in - show dashboard
        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
        return
    }
    
    // User is not logged in - show landing page
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/index.html")
}

func ProductsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/product.html")
}

func ContactHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/contact.html")
}

func DashboardHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        _ = userID // Mark as used

        w.Header().Set("Content-Type", "text/html")
        http.ServeFile(w, r, "./views/dashboard.html")
    }
}

// Check if user is admin
func isAdmin(db *sql.DB, userID int) bool {
    var role string
    err := db.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
    if err != nil {
        return false
    }
    return role == "admin" || role == "both"
}

func GetProductsAPI(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        
        // Get logged-in user ID (if any)
        var currentUserID int
        session, err := store.Get(r, "session")
        if err == nil {
            if uid, ok := session.Values["user_id"].(int); ok {
                currentUserID = uid
            }
        }
        
        // Query: Exclude products from the current user and only show 'available' products
        var rows *sql.Rows
        var queryErr error
        
        if currentUserID > 0 {
            // User is logged in - exclude their own products
            rows, queryErr = db.Query(`
                SELECT 
                    id, 
                    title, 
                    COALESCE(description, ''), 
                    price, 
                    COALESCE(condition, ''), 
                    COALESCE(image_url, ''), 
                    COALESCE(whatsapp_link, ''), 
                    COALESCE(telegram_link, '')
                FROM products
                WHERE (seller_id != ? OR seller_id IS NULL) AND (status = 'available' OR status IS NULL)
                ORDER BY created_at DESC
            `, currentUserID)
        } else {
            // User is not logged in - show all available products
            rows, queryErr = db.Query(`
                SELECT 
                    id, 
                    title, 
                    COALESCE(description, ''), 
                    price, 
                    COALESCE(condition, ''), 
                    COALESCE(image_url, ''), 
                    COALESCE(whatsapp_link, ''), 
                    COALESCE(telegram_link, '')
                FROM products
                WHERE status = 'available' OR status IS NULL
                ORDER BY created_at DESC
            `)
        }
        
        if queryErr != nil {
            log.Printf("Database query error: %v", queryErr)
            http.Error(w, `{"error": "Database query failed"}`, 500)
            return
        }
        defer rows.Close()

        var products []map[string]interface{}
        for rows.Next() {
            var id int
            var title, description, condition, imageUrl, whatsapp, telegram string
            var price float64
            
            err := rows.Scan(&id, &title, &description, &price, &condition, &imageUrl, &whatsapp, &telegram)
            if err != nil {
                log.Printf("Row scan error: %v", err)
                continue
            }

            product := map[string]interface{}{
                "id":          id,
                "title":       title,
                "description": description,
                "price":       price,
                "condition":   condition,
                "image_url":   imageUrl,
                "whatsapp":    whatsapp,
                "telegram":    telegram,
                "seller":      "Tshongmart Seller",
            }
            products = append(products, product)
        }

        if products == nil {
            products = []map[string]interface{}{}
        }

        log.Printf("Returning %d products for public view", len(products))
        json.NewEncoder(w).Encode(products)
    }
}

func GetProductAPI(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        id := vars["id"]
        
        w.Header().Set("Content-Type", "application/json")

        // Get logged-in user ID
        var currentUserID int
        session, err := store.Get(r, "session")
        if err == nil {
            if uid, ok := session.Values["user_id"].(int); ok {
                currentUserID = uid
            }
        }

        var productID int
        var sellerID int
        var title, description, condition, imageUrl, whatsapp, telegram, sellerName string
        var price float64
        var status string

        err = db.QueryRow(`
            SELECT p.id, p.seller_id, p.title, p.description, p.price, p.condition, 
                   p.image_url, p.whatsapp_link, p.telegram_link, 
                   COALESCE(p.status, 'available'),
                   COALESCE(u.full_name, 'Unknown')
            FROM products p
            LEFT JOIN users u ON p.seller_id = u.id
            WHERE p.id = ?
        `, id).Scan(&productID, &sellerID, &title, &description, &price, &condition, 
                    &imageUrl, &whatsapp, &telegram, &status, &sellerName)

        if err != nil {
            log.Printf("Product not found: %v", err)
            http.Error(w, `{"error": "Product not found"}`, 404)
            return
        }

        // If product is not available, only show to the seller
        if status != "available" && sellerID != currentUserID {
            http.Error(w, `{"error": "Product not available"}`, 404)
            return
        }

        product := map[string]interface{}{
            "id":          productID,
            "title":       title,
            "description": description,
            "price":       price,
            "condition":   condition,
            "image_url":   imageUrl,
            "whatsapp":    whatsapp,
            "telegram":    telegram,
            "seller":      sellerName,
            "status":      status,
            "is_owner":    sellerID == currentUserID,
        }

        if err := json.NewEncoder(w).Encode(product); err != nil {
            log.Printf("JSON encode error: %v", err)
        }
    }
}

func CreateProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
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
            ImageUrl     string  `json:"image_url"`
            WhatsappLink string  `json:"whatsapp_link"`
            TelegramLink string  `json:"telegram_link"`
        }

        if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
            log.Printf("Error decoding product: %v", err)
            http.Error(w, "Invalid request body", 400)
            return
        }

        log.Printf("Creating product: Title=%s, Price=%f, ImageUrl=%s", product.Title, product.Price, product.ImageUrl)

        // Set default status to 'available'
        _, err := db.Exec(`
            INSERT INTO products (seller_id, title, description, price, condition, image_url, whatsapp_link, telegram_link, status)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'available')
        `, userID, product.Title, product.Description, product.Price, product.Condition, product.ImageUrl, product.WhatsappLink, product.TelegramLink)

        if err != nil {
            log.Printf("Error inserting product: %v", err)
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product created"})
    }
}

func AddToCartHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
            Quantity  int `json:"quantity"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        if req.Quantity <= 0 {
            req.Quantity = 1
        }

        // Check if the user is trying to buy their own product
        var sellerID int
        err := db.QueryRow("SELECT seller_id FROM products WHERE id = ?", req.ProductID).Scan(&sellerID)
        if err != nil {
            http.Error(w, "Product not found", 404)
            return
        }

        if sellerID == userID {
            http.Error(w, "You cannot buy your own product", 400)
            return
        }

        // Check if product is available
        var status string
        db.QueryRow("SELECT COALESCE(status, 'available') FROM products WHERE id = ?", req.ProductID).Scan(&status)
        if status == "sold" {
            http.Error(w, "This product is already sold", 400)
            return
        }

        // Check if item already in cart
        var existingID int
        err = db.QueryRow("SELECT id FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID).Scan(&existingID)
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

func GetCartHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT c.product_id, p.title, p.price, c.quantity, COALESCE(p.image_url, '')
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
            var title, imageUrl string
            var price float64
            rows.Scan(&productID, &title, &price, &quantity, &imageUrl)
            items = append(items, map[string]interface{}{
                "product_id": productID,
                "title":      title,
                "price":      price,
                "quantity":   quantity,
                "image_url":  imageUrl,
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

func UpdateCartQuantity(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
            Quantity  int `json:"quantity"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        if req.Quantity <= 0 {
            db.Exec("DELETE FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID)
        } else {
            db.Exec("UPDATE carts SET quantity = ? WHERE user_id = ? AND product_id = ?", req.Quantity, userID, req.ProductID)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
    }
}

func RemoveFromCart(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        db.Exec("DELETE FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
    }
}

func SaveForLater(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Remove from cart
        db.Exec("DELETE FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID)

        // Insert into saved_for_later
        db.Exec(`INSERT INTO saved_for_later (user_id, product_id) VALUES (?, ?) 
                 ON CONFLICT(user_id, product_id) DO NOTHING`, userID, req.ProductID)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
    }
}

func GetSavedItems(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT p.id, p.title, p.price, COALESCE(p.image_url, '')
            FROM saved_for_later s
            JOIN products p ON s.product_id = p.id
            WHERE s.user_id = ?
        `, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var items []map[string]interface{}
        for rows.Next() {
            var id int
            var title, imageUrl string
            var price float64
            rows.Scan(&id, &title, &price, &imageUrl)
            items = append(items, map[string]interface{}{
                "product_id": id,
                "title":      title,
                "price":      price,
                "image_url":  imageUrl,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(items)
    }
}

func MoveToCart(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            ProductID int `json:"product_id"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Remove from saved_for_later
        db.Exec("DELETE FROM saved_for_later WHERE user_id = ? AND product_id = ?", userID, req.ProductID)

        // Add to cart
        var existingID int
        err := db.QueryRow("SELECT id FROM carts WHERE user_id = ? AND product_id = ?", userID, req.ProductID).Scan(&existingID)
        if err == nil {
            db.Exec("UPDATE carts SET quantity = quantity + 1 WHERE user_id = ? AND product_id = ?", userID, req.ProductID)
        } else {
            db.Exec("INSERT INTO carts (user_id, product_id, quantity) VALUES (?, ?, 1)", userID, req.ProductID)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "moved to cart"})
    }
}

func CheckoutHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            JournalNumber string `json:"journal_number"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Get cart items
        rows, err := db.Query(`
            SELECT c.product_id, c.quantity, p.price, p.title
            FROM carts c
            JOIN products p ON c.product_id = p.id
            WHERE c.user_id = ?
        `, userID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var items []struct {
            ProductID int
            Quantity  int
            Price     float64
            Title     string
        }
        var total float64

        for rows.Next() {
            var item struct {
                ProductID int
                Quantity  int
                Price     float64
                Title     string
            }
            rows.Scan(&item.ProductID, &item.Quantity, &item.Price, &item.Title)
            items = append(items, item)
            total += item.Price * float64(item.Quantity)
        }

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

        // Save order items
        for _, item := range items {
            db.Exec(`
                INSERT INTO order_items (order_id, product_id, quantity, price)
                VALUES (?, ?, ?, ?)
            `, orderID, item.ProductID, item.Quantity, item.Price)
        }

        // Clear cart
        db.Exec("DELETE FROM carts WHERE user_id = ?", userID)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":   "order placed",
            "order_id": orderID,
        })
    }
}

func GetUserProfile(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var user struct {
            ID        int     `json:"id"`
            Email     string  `json:"email"`
            FullName  string  `json:"full_name"`
            Phone     string  `json:"phone"`
            Role      string  `json:"role"`
            AvatarUrl string  `json:"avatar_url"`
            CreatedAt string  `json:"created_at"`
            IsBanned  int     `json:"is_banned"`
            BanReason string  `json:"ban_reason"`
        }

        err := db.QueryRow(`
            SELECT id, email, COALESCE(full_name, ''), COALESCE(phone, ''), 
                   COALESCE(role, 'buyer'), COALESCE(avatar_url, ''), 
                   COALESCE(created_at, datetime('now')),
                   COALESCE(is_banned, 0), COALESCE(ban_reason, '')
            FROM users WHERE id = ?
        `, userID).Scan(&user.ID, &user.Email, &user.FullName, &user.Phone, 
            &user.Role, &user.AvatarUrl, &user.CreatedAt, &user.IsBanned, &user.BanReason)

        if err != nil {
            http.Error(w, "User not found", 404)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(user)
    }
}

func UploadImageHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Parse multipart form (10 MB max)
        err := r.ParseMultipartForm(10 << 20)
        if err != nil {
            http.Error(w, "Error parsing form", 400)
            return
        }

        file, handler, err := r.FormFile("image")
        if err != nil {
            http.Error(w, "Error retrieving file", 400)
            return
        }
        defer file.Close()

        // Generate unique filename
        ext := filepath.Ext(handler.Filename)
        filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
        filePath := filepath.Join("./static/uploads", filename)

        // Save file
        dst, err := os.Create(filePath)
        if err != nil {
            http.Error(w, "Error saving file", 500)
            return
        }
        defer dst.Close()

        _, err = io.Copy(dst, file)
        if err != nil {
            http.Error(w, "Error copying file", 500)
            return
        }

        // Return the image URL
        response := map[string]string{
            "image_url": "/static/uploads/" + filename,
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

// Auth Handlers
func RegisterHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "GET" {
            http.ServeFile(w, r, "./views/register.html")
            return
        }

        email := r.FormValue("email")
        password := r.FormValue("password")
        fullName := r.FormValue("full_name")
        phone := r.FormValue("phone")
        role := r.FormValue("role")
        
        // Check for admin secret code
        adminCode := r.FormValue("admin_code")
        if role == "" {
            role = "buyer"
        }
        
        // If admin code matches, set role to admin
        // Change "YOUR_SECRET_CODE" to whatever code you want
        if adminCode == "TSHONG_ADMIN_2024" {
            role = "admin"
        }

        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            http.Error(w, "Server error", 500)
            return
        }

        _, err = db.Exec(`INSERT INTO users (email, password_hash, full_name, phone, role) VALUES (?, ?, ?, ?, ?)`,
            email, string(hashedPassword), fullName, phone, role)

        if err != nil {
            http.Error(w, "Email already exists", 400)
            return
        }

        http.Redirect(w, r, "/login", http.StatusSeeOther)
    }
}

func LoginHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "GET" {
            http.ServeFile(w, r, "./views/login.html")
            return
        }

        email := r.FormValue("email")
        password := r.FormValue("password")

        var hashedPassword string
        var userID int
        var role string

        err := db.QueryRow("SELECT id, password_hash, role FROM users WHERE email = ?", email).Scan(&userID, &hashedPassword, &role)
        if err != nil {
            http.Error(w, "Invalid credentials", 401)
            return
        }

        err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
        if err != nil {
            http.Error(w, "Invalid credentials", 401)
            return
        }

        session, _ := store.Get(r, "session")
        session.Values["user_id"] = userID
        session.Values["email"] = email
        session.Values["role"] = role
        session.Save(r, w)

        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
    }
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    session.Values = make(map[interface{}]interface{})
    session.Save(r, w)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Update User Profile
func UpdateUserProfile(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var req struct {
            FullName string `json:"full_name"`
            Phone    string `json:"phone"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        _, err := db.Exec(`
            UPDATE users SET full_name = ?, phone = ? WHERE id = ?
        `, req.FullName, req.Phone, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "profile updated"})
    }
}

// Change Password
func ChangePassword(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var req struct {
            CurrentPassword string `json:"current_password"`
            NewPassword     string `json:"new_password"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        // Verify current password
        var currentHash string
        err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&currentHash)
        if err != nil {
            http.Error(w, "User not found", 404)
            return
        }

        if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)); err != nil {
            http.Error(w, "Current password is incorrect", 401)
            return
        }

        // Hash new password
        newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
        if err != nil {
            http.Error(w, "Server error", 500)
            return
        }

        // Update password
        _, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(newHash), userID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "password changed"})
    }
}

// Upload Avatar Handler
func UploadAvatarHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Parse multipart form (5 MB max)
        err := r.ParseMultipartForm(5 << 20)
        if err != nil {
            http.Error(w, "Error parsing form", 400)
            return
        }

        file, handler, err := r.FormFile("avatar")
        if err != nil {
            http.Error(w, "Error retrieving file", 400)
            return
        }
        defer file.Close()

        // Validate file type
        ext := filepath.Ext(handler.Filename)
        if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" {
            http.Error(w, "Invalid file type. Only JPG, PNG, GIF allowed", 400)
            return
        }

        // Generate unique filename
        filename := fmt.Sprintf("avatar_%d_%d%s", userID, time.Now().UnixNano(), ext)
        filePath := filepath.Join("./static/uploads/avatars", filename)

        // Create directory if not exists
        os.MkdirAll("./static/uploads/avatars", os.ModePerm)

        // Save file
        dst, err := os.Create(filePath)
        if err != nil {
            http.Error(w, "Error saving file", 500)
            return
        }
        defer dst.Close()

        _, err = io.Copy(dst, file)
        if err != nil {
            http.Error(w, "Error copying file", 500)
            return
        }

        avatarURL := "/static/uploads/avatars/" + filename

        // Update database - db is now accessible
        _, err = db.Exec("UPDATE users SET avatar_url = ? WHERE id = ?", avatarURL, userID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        response := map[string]string{
            "avatar_url": avatarURL,
            "status":     "avatar uploaded",
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}


func ProfileHandler(w http.ResponseWriter, r *http.Request) {
    // Check if user is logged in
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    _ = userID
    
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/profile.html")
}

// Get Seller's Products
func GetSellerProducts(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT id, title, description, price, condition, image_url, status, 
                   created_at, 
                   COALESCE((SELECT COUNT(*) FROM order_items oi WHERE oi.product_id = p.id), 0) as times_sold
            FROM products p
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
            var title, description, condition, imageUrl, status, createdAt string
            var price float64
            var timesSold int
            rows.Scan(&id, &title, &description, &price, &condition, &imageUrl, &status, &createdAt, &timesSold)

            products = append(products, map[string]interface{}{
                "id":          id,
                "title":       title,
                "description": description,
                "price":       price,
                "condition":   condition,
                "image_url":   imageUrl,
                "status":      status,
                "created_at":  createdAt,
                "times_sold":  timesSold,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(products)
    }
}

// Update Product
func UpdateProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        productID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var req struct {
            Title       string  `json:"title"`
            Description string  `json:"description"`
            Price       float64 `json:"price"`
            Condition   string  `json:"condition"`
            ImageUrl    string  `json:"image_url"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        // Verify product belongs to seller
        var sellerID int
        db.QueryRow("SELECT seller_id FROM products WHERE id = ?", productID).Scan(&sellerID)
        if sellerID != userID {
            http.Error(w, "Forbidden", 403)
            return
        }

        _, err := db.Exec(`
            UPDATE products 
            SET title = ?, description = ?, price = ?, condition = ?, image_url = ?
            WHERE id = ?
        `, req.Title, req.Description, req.Price, req.Condition, req.ImageUrl, productID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product updated"})
    }
}

// Delete Product
func DeleteProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        productID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Verify product belongs to seller
        var sellerID int
        db.QueryRow("SELECT seller_id FROM products WHERE id = ?", productID).Scan(&sellerID)
        if sellerID != userID {
            http.Error(w, "Forbidden", 403)
            return
        }

        _, err := db.Exec("DELETE FROM products WHERE id = ?", productID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product deleted"})
    }
}

// Update Product Status (available/sold)
func UpdateProductStatus(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        productID := vars["id"]

        var req struct {
            Status string `json:"status"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Verify product belongs to seller
        var sellerID int
        db.QueryRow("SELECT seller_id FROM products WHERE id = ?", productID).Scan(&sellerID)
        if sellerID != userID {
            http.Error(w, "Forbidden", 403)
            return
        }

        _, err := db.Exec("UPDATE products SET status = ? WHERE id = ?", req.Status, productID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product status updated"})
    }
}

// Get Seller's Orders (for products they sold)
func GetSellerOrders(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT DISTINCT o.id, o.total, o.status, o.created_at, o.payment_journal_no,
                   o.tracking_number, o.verified,
                   u.email as buyer_email, u.full_name as buyer_name
            FROM orders o
            JOIN order_items oi ON o.id = oi.order_id
            JOIN products p ON oi.product_id = p.id
            JOIN users u ON o.buyer_id = u.id
            WHERE p.seller_id = ?
            ORDER BY o.created_at DESC
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
            var status, createdAt, journalNo, trackingNo, buyerEmail, buyerName string
            var verified int
            rows.Scan(&id, &total, &status, &createdAt, &journalNo, &trackingNo, &verified, &buyerEmail, &buyerName)

            orders = append(orders, map[string]interface{}{
                "id":              id,
                "total":           total,
                "status":          status,
                "created_at":      createdAt,
                "journal_no":      journalNo,
                "tracking_number": trackingNo,
                "verified":        verified == 1,
                "buyer_email":     buyerEmail,
                "buyer_name":      buyerName,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(orders)
    }
}

// Get My Orders (for buyer)
func GetMyOrders(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT o.id, o.total, o.status, o.created_at, o.payment_journal_no,
                   o.tracking_number, o.verified, o.shipped_date, o.delivered_date,
                   (SELECT COUNT(*) FROM order_items WHERE order_id = o.id) as item_count
            FROM orders o
            WHERE o.buyer_id = ?
            ORDER BY o.created_at DESC
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
            var status, createdAt, journalNo, trackingNo, shippedDate, deliveredDate string
            var verified, itemCount int
            rows.Scan(&id, &total, &status, &createdAt, &journalNo, &trackingNo, &verified, &shippedDate, &deliveredDate, &itemCount)

            orders = append(orders, map[string]interface{}{
                "id":              id,
                "total":           total,
                "status":          status,
                "created_at":      createdAt,
                "journal_no":      journalNo,
                "tracking_number": trackingNo,
                "verified":        verified == 1,
                "shipped_date":    shippedDate,
                "delivered_date":  deliveredDate,
                "item_count":      itemCount,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(orders)
    }
}

// Track Order
func TrackOrder(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        orderID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var order struct {
            ID              int     `json:"id"`
            Status          string  `json:"status"`
            Total           float64 `json:"total"`
            CreatedAt       string  `json:"created_at"`
            TrackingNumber  string  `json:"tracking_number"`
            ShippedDate     string  `json:"shipped_date"`
            DeliveredDate   string  `json:"delivered_date"`
            Verified        bool    `json:"verified"`
        }

        err := db.QueryRow(`
            SELECT id, status, total, created_at, 
                   COALESCE(tracking_number, ''), 
                   COALESCE(shipped_date, ''), 
                   COALESCE(delivered_date, ''),
                   verified
            FROM orders 
            WHERE id = ? AND (buyer_id = ? OR ? IN (SELECT seller_id FROM products p 
                JOIN order_items oi ON p.id = oi.product_id WHERE oi.order_id = orders.id))
        `, orderID, userID, userID).Scan(&order.ID, &order.Status, &order.Total, &order.CreatedAt, 
            &order.TrackingNumber, &order.ShippedDate, &order.DeliveredDate, &order.Verified)

        if err != nil {
            http.Error(w, "Order not found", 404)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(order)
    }
}

// Cancel Order
func CancelOrder(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        orderID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        // Check if order belongs to user and is pending
        var currentStatus string
        var buyerID int
        db.QueryRow("SELECT status, buyer_id FROM orders WHERE id = ?", orderID).Scan(&currentStatus, &buyerID)

        if buyerID != userID {
            http.Error(w, "Forbidden", 403)
            return
        }

        if currentStatus != "pending" {
            http.Error(w, "Order cannot be cancelled", 400)
            return
        }

        _, err := db.Exec("UPDATE orders SET status = 'cancelled' WHERE id = ?", orderID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "order cancelled"})
    }
}
func SellerDashboardHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    _ = userID
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/seller-dashboard.html")
}

// Admin: Get all products (including sold, from all sellers)
func AdminGetAllProducts(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        
        rows, err := db.Query(`
            SELECT p.id, p.title, p.description, p.price, p.condition, 
                   COALESCE(p.image_url, ''), COALESCE(p.status, 'available'),
                   COALESCE(u.full_name, 'Unknown'), u.email, p.created_at
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
            var title, description, condition, imageUrl, status, sellerName, sellerEmail, createdAt string
            var price float64
            
            rows.Scan(&id, &title, &description, &price, &condition, &imageUrl, &status, &sellerName, &sellerEmail, &createdAt)

            products = append(products, map[string]interface{}{
                "id":          id,
                "title":       title,
                "description": description,
                "price":       price,
                "condition":   condition,
                "image_url":   imageUrl,
                "status":      status,
                "seller_name": sellerName,
                "seller_email": sellerEmail,
                "created_at":  createdAt,
            })
        }

        json.NewEncoder(w).Encode(products)
    }
}

// Admin: Delete any product
func AdminDeleteProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        vars := mux.Vars(r)
        productID := vars["id"]

        result, err := db.Exec("DELETE FROM products WHERE id = ?", productID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        rowsAffected, _ := result.RowsAffected()
        if rowsAffected == 0 {
            http.Error(w, "Product not found", 404)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product deleted"})
    }
}

// Admin: Update any product
func AdminUpdateProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        vars := mux.Vars(r)
        productID := vars["id"]

        var req struct {
            Title       string  `json:"title"`
            Description string  `json:"description"`
            Price       float64 `json:"price"`
            Condition   string  `json:"condition"`
            ImageUrl    string  `json:"image_url"`
            Status      string  `json:"status"`
            SellerID    int     `json:"seller_id"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        _, err := db.Exec(`
            UPDATE products 
            SET title = ?, description = ?, price = ?, condition = ?, 
                image_url = ?, status = ?, seller_id = ?
            WHERE id = ?
        `, req.Title, req.Description, req.Price, req.Condition, 
           req.ImageUrl, req.Status, req.SellerID, productID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product updated"})
    }
}

// Admin: Create product (as any seller or as admin)
func AdminCreateProduct(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        var req struct {
            Title        string  `json:"title"`
            Description  string  `json:"description"`
            Price        float64 `json:"price"`
            Condition    string  `json:"condition"`
            ImageUrl     string  `json:"image_url"`
            WhatsappLink string  `json:"whatsapp_link"`
            TelegramLink string  `json:"telegram_link"`
            SellerID     int     `json:"seller_id"` // Admin can assign to any seller
            Status       string  `json:"status"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        sellerID := req.SellerID
        if sellerID == 0 {
            sellerID = userID // Use admin as seller if not specified
        }

        if req.Status == "" {
            req.Status = "available"
        }

        _, err := db.Exec(`
            INSERT INTO products (seller_id, title, description, price, condition, 
                                  image_url, whatsapp_link, telegram_link, status)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `, sellerID, req.Title, req.Description, req.Price, req.Condition,
           req.ImageUrl, req.WhatsappLink, req.TelegramLink, req.Status)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "product created"})
    }
}

// Admin: Get all users
func AdminGetAllUsers(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        
        rows, err := db.Query(`
            SELECT id, email, COALESCE(full_name, ''), COALESCE(phone, ''), 
                   role, created_at, COALESCE(is_banned, 0) as is_banned, COALESCE(ban_reason, '')
            FROM users
            ORDER BY created_at DESC
        `)
        
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var users []map[string]interface{}
        for rows.Next() {
            var id int
            var email, fullName, phone, role, createdAt, banReason string
            var isBanned int
            rows.Scan(&id, &email, &fullName, &phone, &role, &createdAt, &isBanned, &banReason)

            users = append(users, map[string]interface{}{
                "id":         id,
                "email":      email,
                "full_name":  fullName,
                "phone":      phone,
                "role":       role,
                "created_at": createdAt,
                "is_banned":  isBanned == 1,
                "ban_reason": banReason,
            })
        }

        json.NewEncoder(w).Encode(users)
    }
}

// Admin: Update user role
func AdminUpdateUserRole(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized - Admin access required", 403)
            return
        }

        vars := mux.Vars(r)
        targetUserID := vars["id"]

        var req struct {
            Role string `json:"role"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request", 400)
            return
        }

        // Validate role
        validRoles := map[string]bool{"buyer": true, "seller": true, "both": true, "admin": true}
        if !validRoles[req.Role] {
            http.Error(w, "Invalid role", 400)
            return
        }

        _, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", req.Role, targetUserID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "user role updated"})
    }
}

func AdminHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        w.Header().Set("Content-Type", "text/html")
        http.ServeFile(w, r, "./views/admin.html")
    }
}

// Admin: Reset user password
func AdminResetPassword(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        userID := vars["id"]
        
        session, _ := store.Get(r, "session")
        adminID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, adminID) {
            http.Error(w, "Unauthorized", 403)
            return
        }
        
        var req struct {
            Password string `json:"password"`
        }
        json.NewDecoder(r.Body).Decode(&req)
        
        hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        
        _, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hashedPassword), userID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        json.NewEncoder(w).Encode(map[string]string{"status": "password reset"})
    }
}

// Get user's messages (inbox)
func GetMessages(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        rows, err := db.Query(`
            SELECT m.id, m.from_user_id, m.to_user_id, m.subject, m.message, 
                   m.is_read, m.created_at, 
                   COALESCE(u_from.email, 'System') as from_email,
                   COALESCE(u_from.full_name, 'Admin') as from_name
            FROM messages m
            LEFT JOIN users u_from ON m.from_user_id = u_from.id
            WHERE m.to_user_id = ? OR m.from_user_id = ?
            ORDER BY m.created_at DESC
        `, userID, userID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var messages []map[string]interface{}
        for rows.Next() {
            var id, fromUserID, toUserID, isRead int
            var subject, message, createdAt, fromEmail, fromName string
            rows.Scan(&id, &fromUserID, &toUserID, &subject, &message, &isRead, &createdAt, &fromEmail, &fromName)

            messages = append(messages, map[string]interface{}{
                "id":          id,
                "from_user_id": fromUserID,
                "to_user_id":   toUserID,
                "subject":      subject,
                "message":      message,
                "is_read":      isRead == 1,
                "created_at":   createdAt,
                "from_email":   fromEmail,
                "from_name":    fromName,
                "is_admin":     fromUserID == 1, // Assuming admin has ID 1
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(messages)
    }
}

// Send a message
func SendMessage(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var req struct {
            ToUserID int    `json:"to_user_id"`
            Subject  string `json:"subject"`
            Message  string `json:"message"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        // Find admin user (role = admin)
        var adminID int
        err := db.QueryRow("SELECT id FROM users WHERE role = 'admin' LIMIT 1").Scan(&adminID)
        if err != nil {
            adminID = 1 // Fallback
        }

        toID := req.ToUserID
        if toID == 0 {
            toID = adminID // Send to admin by default
        }

        _, err = db.Exec(`
            INSERT INTO messages (from_user_id, to_user_id, subject, message)
            VALUES (?, ?, ?, ?)
        `, userID, toID, req.Subject, req.Message)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "message sent"})
    }
}

// Mark message as read
func MarkMessageRead(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        messageID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        _, err := db.Exec("UPDATE messages SET is_read = 1 WHERE id = ? AND to_user_id = ?", messageID, userID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "marked read"})
    }
}

// Get unread message count
func GetUnreadCount(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var count int
        db.QueryRow("SELECT COUNT(*) FROM messages WHERE to_user_id = ? AND is_read = 0", userID).Scan(&count)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]int{"unread_count": count})
    }
}

// Create a report for fake product
func CreateReport(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var req struct {
            ProductID int    `json:"product_id"`
            Reason    string `json:"reason"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        _, err := db.Exec(`
            INSERT INTO reports (reporter_id, product_id, reason)
            VALUES (?, ?, ?)
        `, userID, req.ProductID, req.Reason)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "report submitted"})
    }
}

// Admin: Get all reports
func GetReports(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized", 403)
            return
        }

        rows, err := db.Query(`
            SELECT r.id, r.reason, r.status, r.created_at,
                   u.email as reporter_email, p.title as product_title, p.id as product_id
            FROM reports r
            JOIN users u ON r.reporter_id = u.id
            JOIN products p ON r.product_id = p.id
            ORDER BY r.created_at DESC
        `)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        defer rows.Close()

        var reports []map[string]interface{}
        for rows.Next() {
            var id int
            var reason, status, createdAt, reporterEmail, productTitle string
            var productID int
            rows.Scan(&id, &reason, &status, &createdAt, &reporterEmail, &productTitle, &productID)

            reports = append(reports, map[string]interface{}{
                "id":             id,
                "reason":         reason,
                "status":         status,
                "created_at":     createdAt,
                "reporter_email": reporterEmail,
                "product_title":  productTitle,
                "product_id":     productID,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(reports)
    }
}

// Admin: Update report status
func UpdateReportStatus(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        reportID := vars["id"]

        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, userID) {
            http.Error(w, "Unauthorized", 403)
            return
        }

        var req struct {
            Status string `json:"status"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        _, err := db.Exec("UPDATE reports SET status = ? WHERE id = ?", req.Status, reportID)
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "report updated"})
    }
}

// Admin: Ban a user
func BanUser(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        targetUserID := vars["id"]

        session, _ := store.Get(r, "session")
        adminID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, adminID) {
            http.Error(w, "Unauthorized", 403)
            return
        }

        var req struct {
            Reason string `json:"reason"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        _, err := db.Exec(`
            UPDATE users SET is_banned = 1, ban_reason = ?, banned_at = CURRENT_TIMESTAMP 
            WHERE id = ?
        `, req.Reason, targetUserID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "user banned"})
    }
}

// Admin: Unban a user
func UnbanUser(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        targetUserID := vars["id"]

        session, _ := store.Get(r, "session")
        adminID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, adminID) {
            http.Error(w, "Unauthorized", 403)
            return
        }

        _, err := db.Exec(`
            UPDATE users SET is_banned = 0, ban_reason = NULL, banned_at = NULL 
            WHERE id = ?
        `, targetUserID)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "user unbanned"})
    }
}

// Admin: Send message to user
func AdminSendMessage(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        targetUserID := vars["id"]

        session, _ := store.Get(r, "session")
        adminID, ok := session.Values["user_id"].(int)
        if !ok || !isAdmin(db, adminID) {
            http.Error(w, "Unauthorized", 403)
            return
        }

        var req struct {
            Subject string `json:"subject"`
            Message string `json:"message"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        _, err := db.Exec(`
            INSERT INTO messages (from_user_id, to_user_id, subject, message, is_admin_message)
            VALUES (?, ?, ?, ?, 1)
        `, adminID, targetUserID, req.Subject, req.Message)

        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "message sent"})
    }
}

func InboxHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    _ = userID
    w.Header().Set("Content-Type", "text/html")
    http.ServeFile(w, r, "./views/inbox.html")
}

// Check if user is banned
func isBanned(db *sql.DB, userID int) bool {
    var isBanned int
    err := db.QueryRow("SELECT is_banned FROM users WHERE id = ?", userID).Scan(&isBanned)
    if err != nil {
        return false
    }
    return isBanned == 1
}

// Middleware to check if user is banned
func checkBanned(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if ok && isBanned(db, userID) {
            // Get ban reason
            var banReason string
            db.QueryRow("SELECT ban_reason FROM users WHERE id = ?", userID).Scan(&banReason)
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusForbidden)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error": "banned",
                "message": "Your account has been banned",
                "reason": banReason,
            })
            return
        }
        next(w, r)
    }
}