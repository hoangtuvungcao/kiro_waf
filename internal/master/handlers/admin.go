package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kiro_waf/internal/master/db"
	"kiro_waf/internal/master/models"
	admin "kiro_waf/internal/master/templates/admin"
)

// RegisterAdminRoutes registers all admin routes on the given mux with
// IP allowlist, brute-force protection, and session middleware applied.
func RegisterAdminRoutes(mux *http.ServeMux, database *db.DB, config *AdminAuthConfig) {
	// Login/logout routes: protected by IP allowlist + brute-force, but NOT session.
	loginHandler := AdminIPAllowlistMiddleware(config,
		AdminBruteForceMiddleware(database,
			HandleAdminLogin(database, config),
		),
	)
	logoutHandler := AdminIPAllowlistMiddleware(config,
		AdminSessionMiddleware(database,
			HandleAdminLogout(database),
		),
	)

	mux.Handle("/admin/login", loginHandler)
	mux.Handle("/admin/logout", logoutHandler)

	// All other admin routes: IP allowlist + session required.
	sessionProtected := func(h http.Handler) http.Handler {
		return AdminIPAllowlistMiddleware(config,
			AdminSessionMiddleware(database, h),
		)
	}

	// Dashboard
	mux.Handle("/admin/", sessionProtected(handleAdminDashboard(database)))

	// Licenses
	mux.Handle("/admin/licenses", sessionProtected(handleAdminLicenses(database)))
	mux.Handle("/admin/licenses/", sessionProtected(handleAdminLicenseAction(database)))

	// Releases
	mux.Handle("/admin/releases", sessionProtected(handleAdminReleases(database)))
	mux.Handle("/admin/releases/", sessionProtected(handleAdminReleaseAction(database)))

	// Heartbeats
	mux.Handle("/admin/heartbeats", sessionProtected(handleAdminHeartbeats(database)))
}

// handleAdminDashboard renders the admin dashboard overview page.
func handleAdminDashboard(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Only match exact /admin/ path.
		if r.URL.Path != "/admin/" {
			http.NotFound(w, r)
			return
		}

		// Gather dashboard stats.
		statusCounts, err := database.CountLicensesByStatus()
		if err != nil {
			log.Printf("admin: dashboard stats error: %v", err)
		}

		totalLicenses := 0
		for _, c := range statusCounts {
			totalLicenses += c
		}

		recentHBCount, err := database.CountRecentHeartbeats(time.Now().Add(-1 * time.Hour))
		if err != nil {
			log.Printf("admin: dashboard recent heartbeats error: %v", err)
		}

		recentHeartbeats, err := database.ListHeartbeats(10)
		if err != nil {
			log.Printf("admin: dashboard heartbeats list error: %v", err)
		}

		stats := admin.DashboardStats{
			TotalLicenses:     totalLicenses,
			ActiveLicenses:    statusCounts["active"],
			SuspendedLicenses: statusCounts["suspended"],
			RevokedLicenses:   statusCounts["revoked"],
			ExpiredLicenses:   statusCounts["expired"],
			RecentHeartbeats:  recentHBCount,
			SystemStatus:      "healthy",
		}

		flash := admin.FlashFromRequest(r)
		data := &admin.DashboardData{
			PageData:         flash,
			Stats:            stats,
			RecentHeartbeats: recentHeartbeats,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := admin.RenderDashboard(w, data); err != nil {
			log.Printf("admin: render dashboard error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// handleAdminLicenses handles GET /admin/licenses (list) and POST /admin/licenses (create).
func handleAdminLicenses(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminLicensesList(w, r, database)
		case http.MethodPost:
			adminLicenseCreate(w, r, database)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// adminLicensesList renders the license list page.
func adminLicensesList(w http.ResponseWriter, r *http.Request, database *db.DB) {
	licenses, err := database.ListLicenses()
	if err != nil {
		log.Printf("admin: list licenses error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	flash := admin.FlashFromRequest(r)
	data := &admin.LicensesData{
		PageData: flash,
		Licenses: licenses,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := admin.RenderLicenses(w, data); err != nil {
		log.Printf("admin: render licenses error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// adminLicenseCreate handles POST /admin/licenses to create a new license.
func adminLicenseCreate(w http.ResponseWriter, r *http.Request, database *db.DB) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Dữ liệu form không hợp lệ"), http.StatusSeeOther)
		return
	}

	customerID := strings.TrimSpace(r.FormValue("customer_id"))
	customerName := strings.TrimSpace(r.FormValue("customer_name"))
	plan := strings.TrimSpace(r.FormValue("plan"))
	validDaysStr := strings.TrimSpace(r.FormValue("valid_days"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	if customerID == "" {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Customer ID là bắt buộc"), http.StatusSeeOther)
		return
	}

	if plan == "" {
		plan = "community"
	}

	validDays := 365
	if validDaysStr != "" {
		if v, err := strconv.Atoi(validDaysStr); err == nil && v > 0 {
			validDays = v
		}
	}

	// Generate license_id and license_key.
	licenseID, err := generateRandomID()
	if err != nil {
		log.Printf("admin: generate license_id error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi tạo license ID"), http.StatusSeeOther)
		return
	}

	licenseKey, err := generateRandomID()
	if err != nil {
		log.Printf("admin: generate license_key error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi tạo license key"), http.StatusSeeOther)
		return
	}

	now := time.Now().UTC()
	license := &models.License{
		LicenseID:    licenseID,
		LicenseKey:   licenseKey,
		CustomerID:   customerID,
		CustomerName: customerName,
		Plan:         plan,
		Status:       "active",
		ValidDays:    validDays,
		CreatedAt:    now,
		ExpiresAt:    now.AddDate(0, 0, validDays),
		Notes:        notes,
	}

	if err := database.CreateLicense(license); err != nil {
		log.Printf("admin: create license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi tạo license: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Tạo license thành công"), http.StatusSeeOther)
}

// handleAdminLicenseAction handles routes under /admin/licenses/:id/...
// Supports: PUT /admin/licenses/:id, DELETE /admin/licenses/:id,
// POST /admin/licenses/:id/renew, POST /admin/licenses/:id/rotate, POST /admin/licenses/:id/revoke
func handleAdminLicenseAction(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse path: /admin/licenses/{id} or /admin/licenses/{id}/{action}
		path := strings.TrimPrefix(r.URL.Path, "/admin/licenses/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}

		// Handle /admin/licenses/new → render create form
		if parts[0] == "new" && r.Method == http.MethodGet {
			flash := admin.FlashFromRequest(r)
			data := &admin.LicenseFormData{
				PageData: flash,
				IsEdit:   false,
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := admin.RenderLicenseForm(w, data); err != nil {
				log.Printf("admin: render new license form error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Determine sub-action.
		action := ""
		if len(parts) == 2 {
			action = parts[1]
		}

		switch {
		case action == "renew" && r.Method == http.MethodPost:
			adminLicenseRenew(w, r, database, id)
		case action == "rotate" && r.Method == http.MethodPost:
			adminLicenseRotate(w, r, database, id)
		case action == "revoke" && r.Method == http.MethodPost:
			adminLicenseRevoke(w, r, database, id)
		case action == "delete" && r.Method == http.MethodPost:
			adminLicenseDelete(w, r, database, id)
		case action == "activate" && r.Method == http.MethodPost:
			adminLicenseActivate(w, r, database, id)
		case action == "suspend" && r.Method == http.MethodPost:
			adminLicenseSuspend(w, r, database, id)
		case action == "upgrade" && r.Method == http.MethodPost:
			adminLicenseUpgrade(w, r, database, id)
		case action == "edit" && r.Method == http.MethodGet:
			adminLicenseEditForm(w, r, database, id)
		case action == "" && r.Method == http.MethodPut:
			adminLicenseUpdate(w, r, database, id)
		case action == "" && r.Method == http.MethodPost:
			// HTML forms can't send PUT/DELETE, so we use POST with _method field.
			if err := r.ParseForm(); err == nil {
				switch r.FormValue("_method") {
				case "PUT":
					adminLicenseUpdate(w, r, database, id)
				case "DELETE":
					adminLicenseDelete(w, r, database, id)
				default:
					adminLicenseUpdate(w, r, database, id)
				}
			} else {
				http.Error(w, "bad request", http.StatusBadRequest)
			}
		case action == "" && r.Method == http.MethodDelete:
			adminLicenseDelete(w, r, database, id)
		case action == "" && r.Method == http.MethodGet:
			adminLicenseEditForm(w, r, database, id)
		default:
			http.NotFound(w, r)
		}
	}
}

// adminLicenseEditForm renders the license edit form.
func adminLicenseEditForm(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	license, err := database.GetLicenseByID(id)
	if err != nil {
		log.Printf("admin: get license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi tải license"), http.StatusSeeOther)
		return
	}
	if license == nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "License không tồn tại"), http.StatusSeeOther)
		return
	}

	flash := admin.FlashFromRequest(r)
	data := &admin.LicenseFormData{
		PageData: flash,
		IsEdit:   true,
		License:  *license,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := admin.RenderLicenseForm(w, data); err != nil {
		log.Printf("admin: render license form error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// adminLicenseUpdate handles PUT /admin/licenses/:id (update license).
func adminLicenseUpdate(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Dữ liệu form không hợp lệ"), http.StatusSeeOther)
		return
	}

	license, err := database.GetLicenseByID(id)
	if err != nil || license == nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "License không tồn tại"), http.StatusSeeOther)
		return
	}

	// Update mutable fields from form.
	if v := strings.TrimSpace(r.FormValue("customer_name")); v != "" {
		license.CustomerName = v
	}
	if v := strings.TrimSpace(r.FormValue("client_ip")); v != "" {
		license.ClientIP = v
	}
	if v := strings.TrimSpace(r.FormValue("plan")); v != "" {
		license.Plan = v
	}
	if v := strings.TrimSpace(r.FormValue("status")); v != "" {
		license.Status = v
	}
	if v := strings.TrimSpace(r.FormValue("valid_days")); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days > 0 {
			license.ValidDays = days
		}
	}
	if v := strings.TrimSpace(r.FormValue("notes")); r.Form.Has("notes") {
		license.Notes = v
	}

	if err := database.UpdateLicense(license); err != nil {
		log.Printf("admin: update license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi cập nhật license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Cập nhật license thành công"), http.StatusSeeOther)
}

// adminLicenseDelete handles DELETE /admin/licenses/:id.
func adminLicenseDelete(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := database.DeleteLicense(id); err != nil {
		log.Printf("admin: delete license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi xóa license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Xóa license thành công"), http.StatusSeeOther)
}

// adminLicenseRenew handles POST /admin/licenses/:id/renew.
func adminLicenseRenew(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := database.RenewLicense(id); err != nil {
		log.Printf("admin: renew license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi gia hạn license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Gia hạn license thành công"), http.StatusSeeOther)
}

// adminLicenseRotate handles POST /admin/licenses/:id/rotate.
func adminLicenseRotate(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	newKey, err := database.RotateLicenseKey(id)
	if err != nil {
		log.Printf("admin: rotate license key error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi xoay key license"), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Xoay key thành công. Key mới: %s", newKey[:8]+"...")
	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", msg), http.StatusSeeOther)
}

// adminLicenseRevoke handles POST /admin/licenses/:id/revoke.
func adminLicenseRevoke(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := database.RevokeLicense(id); err != nil {
		log.Printf("admin: revoke license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi thu hồi license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Thu hồi license thành công"), http.StatusSeeOther)
}

// adminLicenseActivate handles POST /admin/licenses/:id/activate.
// Sets license status to "active".
func adminLicenseActivate(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	license, err := database.GetLicenseByID(id)
	if err != nil || license == nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "License không tồn tại"), http.StatusSeeOther)
		return
	}

	license.Status = "active"
	if err := database.UpdateLicense(license); err != nil {
		log.Printf("admin: activate license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi kích hoạt license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Kích hoạt license thành công"), http.StatusSeeOther)
}

// adminLicenseSuspend handles POST /admin/licenses/:id/suspend.
// Sets license status to "suspended".
func adminLicenseSuspend(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	license, err := database.GetLicenseByID(id)
	if err != nil || license == nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "License không tồn tại"), http.StatusSeeOther)
		return
	}

	license.Status = "suspended"
	if err := database.UpdateLicense(license); err != nil {
		log.Printf("admin: suspend license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi tạm ngưng license"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", "Tạm ngưng license thành công"), http.StatusSeeOther)
}

// adminLicenseUpgrade handles POST /admin/licenses/:id/upgrade.
// Upgrades a license to a higher plan while preserving license_key and fingerprint.
func adminLicenseUpgrade(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Dữ liệu form không hợp lệ"), http.StatusSeeOther)
		return
	}

	newPlan := strings.TrimSpace(r.FormValue("plan"))
	validDaysStr := strings.TrimSpace(r.FormValue("valid_days"))

	if newPlan != "pro" && newPlan != "enterprise" {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Gói nâng cấp phải là Pro hoặc Enterprise"), http.StatusSeeOther)
		return
	}

	validDays := 365
	if validDaysStr != "" {
		if v, err := strconv.Atoi(validDaysStr); err == nil && v > 0 {
			validDays = v
		}
	}

	license, err := database.GetLicenseByID(id)
	if err != nil || license == nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "License không tồn tại"), http.StatusSeeOther)
		return
	}

	now := time.Now().UTC()
	license.Plan = newPlan
	license.Status = "active"
	license.ValidDays = validDays
	license.ExpiresAt = now.AddDate(0, 0, validDays)

	if err := database.UpdateLicense(license); err != nil {
		log.Printf("admin: upgrade license error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_error", "Lỗi nâng cấp license"), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Nâng cấp license thành công lên gói %s (%d ngày)", newPlan, validDays)
	http.Redirect(w, r, admin.RedirectWithFlash("/admin/licenses", "flash_success", msg), http.StatusSeeOther)
}

// handleAdminReleases handles GET /admin/releases (list) and POST /admin/releases (create).
func handleAdminReleases(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminReleasesList(w, r, database)
		case http.MethodPost:
			adminReleaseCreate(w, r, database)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// adminReleasesList renders the releases list page.
func adminReleasesList(w http.ResponseWriter, r *http.Request, database *db.DB) {
	releases, err := database.ListReleases()
	if err != nil {
		log.Printf("admin: list releases error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	flash := admin.FlashFromRequest(r)
	data := &admin.ReleasesData{
		PageData: flash,
		Releases: releases,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := admin.RenderReleases(w, data); err != nil {
		log.Printf("admin: render releases error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// adminReleaseCreate handles POST /admin/releases to create a new release.
func adminReleaseCreate(w http.ResponseWriter, r *http.Request, database *db.DB) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_error", "Dữ liệu form không hợp lệ"), http.StatusSeeOther)
		return
	}

	component := strings.TrimSpace(r.FormValue("component"))
	channel := strings.TrimSpace(r.FormValue("channel"))
	version := strings.TrimSpace(r.FormValue("version"))
	artifactURL := strings.TrimSpace(r.FormValue("artifact_url"))
	sha256 := strings.TrimSpace(r.FormValue("sha256"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	minVersion := strings.TrimSpace(r.FormValue("min_version"))

	if component == "" || version == "" || artifactURL == "" || sha256 == "" {
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_error", "Component, version, artifact URL, và SHA256 là bắt buộc"), http.StatusSeeOther)
		return
	}

	if channel == "" {
		channel = "stable"
	}
	if minVersion == "" {
		minVersion = "0.0.0"
	}

	release := &models.Release{
		Component:   component,
		Channel:     channel,
		Version:     version,
		ArtifactURL: artifactURL,
		SHA256:      sha256,
		Notes:       notes,
		MinVersion:  minVersion,
		CreatedAt:   time.Now().UTC(),
	}

	if err := database.CreateRelease(release); err != nil {
		log.Printf("admin: create release error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_error", "Lỗi tạo release: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_success", "Tạo release thành công"), http.StatusSeeOther)
}

// handleAdminReleaseAction handles routes under /admin/releases/:id/...
func handleAdminReleaseAction(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse path: /admin/releases/{id}
		path := strings.TrimPrefix(r.URL.Path, "/admin/releases/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}

		// Handle /admin/releases/new → render create form
		if parts[0] == "new" && r.Method == http.MethodGet {
			flash := admin.FlashFromRequest(r)
			data := &admin.ReleaseFormData{
				PageData: flash,
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := admin.RenderReleaseForm(w, data); err != nil {
				log.Printf("admin: render new release form error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case http.MethodDelete:
			adminReleaseDelete(w, r, database, id)
		case http.MethodPost:
			// HTML forms can't send DELETE, so we use POST with _method field.
			if err := r.ParseForm(); err == nil && r.FormValue("_method") == "DELETE" {
				adminReleaseDelete(w, r, database, id)
			} else {
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}
}

// adminReleaseDelete handles DELETE /admin/releases/:id.
func adminReleaseDelete(w http.ResponseWriter, r *http.Request, database *db.DB, id int64) {
	if err := database.DeleteRelease(id); err != nil {
		log.Printf("admin: delete release error: %v", err)
		http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_error", "Lỗi xóa release"), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, admin.RedirectWithFlash("/admin/releases", "flash_success", "Xóa release thành công"), http.StatusSeeOther)
}

// handleAdminHeartbeats handles GET /admin/heartbeats.
func handleAdminHeartbeats(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		heartbeats, err := database.ListHeartbeats(100)
		if err != nil {
			log.Printf("admin: list heartbeats error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		flash := admin.FlashFromRequest(r)
		data := &admin.HeartbeatsData{
			PageData:   flash,
			Heartbeats: heartbeats,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := admin.RenderHeartbeats(w, data); err != nil {
			log.Printf("admin: render heartbeats error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// generateRandomID produces a random 16-byte hex-encoded string for IDs.
func generateRandomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
