package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"dormhub/auth"
	"dormhub/config"
	"dormhub/handler"
	"dormhub/mw"
	"dormhub/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด ", err)
	}

	tickets := &auth.Issuer{Secret: []byte(cfg.JWTSecret), TTL: config.TokenTTL}
	h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}

	// ครอบ handler ด้วยด่านตรวจสิทธิ์ อ่านแล้วรู้เลยว่าเส้นไหนใครเข้าได้
	member := func(f http.HandlerFunc) http.Handler { return mw.RequireAuth(tickets, f) }
	admin := func(f http.HandlerFunc) http.Handler { return mw.RequireAdmin(tickets, f) }

	mux := http.NewServeMux()

	// ---------- Guest: ไม่ต้อง login ----------
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/zones", h.ListZones)
	mux.HandleFunc("GET /api/dorms", h.ListDorms)
	mux.HandleFunc("GET /api/dorms/{id}", h.GetDorm)
	mux.HandleFunc("GET /api/dorms/{id}/reviews", h.ListReviews)

	// ---------- Member: ต้องมีตั๋ว ----------
	mux.Handle("GET /api/me", member(h.Me))
	mux.Handle("PUT /api/me", member(h.UpdateMe))
	mux.Handle("PUT /api/me/password", member(h.ChangePassword))
	mux.Handle("GET /api/me/favorites", member(h.ListFavorites))
	mux.Handle("PUT /api/me/favorites/{dormId}", member(h.AddFavorite))
	mux.Handle("DELETE /api/me/favorites/{dormId}", member(h.RemoveFavorite))
	mux.Handle("GET /api/me/bookings", member(h.ListMyBookings))
	mux.Handle("POST /api/me/bookings/{id}/cancel", member(h.CancelMyBooking))
	mux.Handle("POST /api/dorms/{id}/reviews", member(h.CreateReview))
	mux.Handle("POST /api/dorms/{id}/bookings", member(h.CreateBooking))
	mux.Handle("DELETE /api/reviews/{id}", member(h.DeleteReview)) // เจ้าของ หรือ Admin

	// ---------- Admin: ต้องมีตั๋วและ role = admin ----------
	mux.Handle("GET /api/admin/stats", admin(h.AdminStats))
	mux.Handle("GET /api/admin/dorms", admin(h.AdminListDorms))
	mux.Handle("GET /api/admin/dorms/{id}", admin(h.AdminGetDorm))
	mux.Handle("POST /api/admin/dorms", admin(h.AdminCreateDorm))
	mux.Handle("PUT /api/admin/dorms/{id}", admin(h.AdminUpdateDorm))
	mux.Handle("DELETE /api/admin/dorms/{id}", admin(h.AdminDeleteDorm))
	mux.Handle("GET /api/admin/bookings", admin(h.AdminListBookings))
	mux.Handle("PUT /api/admin/bookings/{id}", admin(h.AdminDecideBooking))
	mux.Handle("GET /api/admin/members", admin(h.AdminListMembers))
	mux.Handle("PUT /api/admin/members/{id}/role", admin(h.AdminSetRole))

	// ลำดับการห่อ: Logging (นอกสุด) -> Recovery -> CORS -> mux
	app := mw.Logging(mw.Recovery(mw.CORS(cfg.AllowedOrigins, mux)))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("DormHub API ฟังอยู่ที่ :%s (อนุญาต origin: %v)", cfg.Port, cfg.AllowedOrigins)
	log.Fatal(srv.ListenAndServe())
}
