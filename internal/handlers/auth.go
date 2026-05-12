package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"

    "github.com/gorilla/sessions"
    "golang.org/x/crypto/bcrypt"
)

var Store = sessions.NewCookieStore([]byte("your-secret-key"))

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
        if role == "" {
            role = "buyer"
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

        session, _ := Store.Get(r, "session")
        session.Values["user_id"] = userID
        session.Values["email"] = email
        session.Values["role"] = role
        session.Save(r, w)

        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
    }
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := Store.Get(r, "session")
    session.Values = make(map[interface{}]interface{})
    session.Save(r, w)
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

func GetUserProfile(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := Store.Get(r, "session")
        userID, ok := session.Values["user_id"].(int)
        if !ok {
            http.Error(w, "Unauthorized", 401)
            return
        }

        var user struct {
            ID        int     `json:"id"`
            Email     string  `json:"email"`
            FullName  *string `json:"full_name"`
            Phone     *string `json:"phone"`
            Role      string  `json:"role"`
            CreatedAt string  `json:"created_at"`
        }

        err := db.QueryRow(`
            SELECT id, email, full_name, phone, COALESCE(role, 'buyer'), 
                   COALESCE(created_at, datetime('now'))
            FROM users WHERE id = ?
        `, userID).Scan(&user.ID, &user.Email, &user.FullName, &user.Phone, &user.Role, &user.CreatedAt)

        if err != nil {
            http.Error(w, "User not found", 404)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(user)
    }
}
