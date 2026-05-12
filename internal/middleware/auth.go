package middleware

import (
    "net/http"

    "github.com/gorilla/sessions"
)

var Store *sessions.CookieStore

func InitMiddleware(store *sessions.CookieStore) {
    Store = store
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, err := Store.Get(r, "session")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        userID, ok := session.Values["user_id"]
        if !ok || userID == nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        next(w, r)
    }
}

func RequireRole(role string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, err := Store.Get(r, "session")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        userRole, ok := session.Values["role"].(string)
        if !ok {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        if userRole != role && userRole != "both" {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        next(w, r)
    }
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        session, err := Store.Get(r, "session")
        if err != nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

        userID, ok := session.Values["user_id"]
        if !ok || userID == nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }

        next(w, r)
    }
}
