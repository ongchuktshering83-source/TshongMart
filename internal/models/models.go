package models

import (
    "time"
)

type User struct {
    ID           int       `json:"id"`
    Email        string    `json:"email"`
    PasswordHash string    `json:"-"`
    FullName     string    `json:"full_name"`
    Phone        string    `json:"phone"`
    Role         string    `json:"role"` // buyer, seller, both
    CreatedAt    time.Time `json:"created_at"`
}

type Product struct {
    ID           int       `json:"id"`
    SellerID     int       `json:"seller_id"`
    Title        string    `json:"title"`
    Description  string    `json:"description"`
    Price        float64   `json:"price"`
    Condition    string    `json:"condition"`
    WhatsappLink string    `json:"whatsapp_link"`
    TelegramLink string    `json:"telegram_link"`
    CreatedAt    time.Time `json:"created_at"`
    SellerName   string    `json:"seller_name,omitempty"`
}

type CartItem struct {
    ID        int     `json:"id"`
    UserID    int     `json:"user_id"`
    ProductID int     `json:"product_id"`
    Quantity  int     `json:"quantity"`
    Title     string  `json:"title"`
    Price     float64 `json:"price"`
    Subtotal  float64 `json:"subtotal"`
}

type Order struct {
    ID               int       `json:"id"`
    BuyerID          int       `json:"buyer_id"`
    Total            float64   `json:"total"`
    PaymentJournalNo string    `json:"payment_journal_no"`
    Verified         bool      `json:"verified"`
    Status           string    `json:"status"` // pending, confirmed, shipped, delivered
    CreatedAt        time.Time `json:"created_at"`
}

type Review struct {
    ID         int       `json:"id"`
    ProductID  int       `json:"product_id"`
    ReviewerID int       `json:"reviewer_id"`
    Rating     int       `json:"rating"`
    Comment    string    `json:"comment"`
    CreatedAt  time.Time `json:"created_at"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    FullName string `json:"full_name"`
    Phone    string `json:"phone"`
    Role     string `json:"role"`
}

type AddToCartRequest struct {
    ProductID int `json:"product_id"`
    Quantity  int `json:"quantity"`
}

type CheckoutRequest struct {
    JournalNumber string `json:"journal_number"`
}