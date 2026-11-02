package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/database"
	"almak-back/internal/models"
	"almak-back/internal/routes"
	"almak-back/internal/users"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func isolatedDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ALMAK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ALMAK_TEST_DATABASE_URL to an isolated local test database")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil || !strings.HasPrefix(cfg.Database, "almak_test_") || (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		t.Fatal("integration tests require a loopback database named almak_test_*")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot connect to isolated test database")
	}
	schema := fmt.Sprintf("roles_%d", time.Now().UnixNano())
	if err = root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() {
		database.DB = previous
		connection, _ := db.DB()
		_ = connection.Close()
		root.Exec("DROP SCHEMA " + schema + " CASCADE")
		connection, _ = root.DB()
		_ = connection.Close()
	})
	return db
}

func migrate(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.User{}, &models.Catalog{}, &models.CatalogItem{}); err != nil {
		t.Fatal(err)
	}
}

func TestUserMigrationAndProvisionPreserveExistingAccounts(t *testing.T) {
	db := isolatedDB(t)
	if err := db.Exec("CREATE TABLE users (id bigserial PRIMARY KEY, login text NOT NULL, password text NOT NULL, created_at timestamptz)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_users_login ON users(login)").Error; err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("original-test-password"), bcrypt.MinCost)
	for _, login := range []string{"almak", "admin"} {
		if err := db.Exec("INSERT INTO users(login,password) VALUES (?,?)", login, string(hash)).Error; err != nil {
			t.Fatal(err)
		}
	}
	migrate(t, db)
	admin, err := users.GrantAdmin(db, "almak")
	if err != nil {
		t.Fatal(err)
	}
	id := admin.ID
	for i := 0; i < 2; i++ {
		migrate(t, db)
		admin, err = users.GrantAdmin(db, "almak")
		if err != nil {
			t.Fatal(err)
		}
		if admin.ID != id || admin.Password != string(hash) || admin.Role != models.RoleAdmin {
			t.Fatal("existing account changed")
		}
	}
	var retained models.User
	if err = db.Where("login = ?", "admin").First(&retained).Error; err != nil || retained.Password != string(hash) {
		t.Fatal("admin login deleted or password reset")
	}
	for _, login := range []string{"manager1", "manager2"} {
		first, created, err := users.Provision(db, login, "", models.RoleManager, []byte("unique-manager-test-password"))
		if err != nil || !created {
			t.Fatal("manager not created", err)
		}
		second, created, err := users.Provision(db, login, "", models.RoleManager, []byte("different-test-password"))
		if err != nil || created || first.ID != second.ID || first.Password != second.Password {
			t.Fatal("repeat provisioning reset account", err)
		}
	}
}

func TestProfilesPermissionsAndBothLoginFlows(t *testing.T) {
	db := isolatedDB(t)
	migrate(t, db)
	gin.SetMode(gin.TestMode)
	cfg := config.Config{JWTSecret: "isolated-role-test-secret", FrontendOrigins: []string{"http://localhost:4200"}}
	router := routes.SetupRouter(cfg)
	for _, role := range []models.UserRole{models.RoleAdmin, models.RoleManager} {
		user, _, err := users.Provision(db, string(role), "", role, []byte("isolated-profile-password"))
		if err != nil {
			t.Fatal(err)
		}
		for _, endpoint := range []string{"/api/login", "/api/desktop/login"} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", endpoint, strings.NewReader(fmt.Sprintf(`{"login":%q,"password":"isolated-profile-password","useCookie":true}`, user.Login)))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			if rec.Code != 200 {
				t.Fatalf("login %s: %d", endpoint, rec.Code)
			}
			var response struct {
				Token        string             `json:"token"`
				RefreshToken string             `json:"refreshToken"`
				User         models.UserProfile `json:"user"`
			}
			if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.User.Role != role || len(response.User.Permissions) != len(models.Permissions(models.RoleAdmin)) || strings.Contains(rec.Body.String(), user.Password) {
				t.Fatal("invalid public profile")
			}
			for _, path := range []string{"/api/me", "/me", "/api/catalogs"} {
				result := httptest.NewRecorder()
				request := httptest.NewRequest("GET", path, nil)
				request.Header.Set("Authorization", "Bearer "+response.Token)
				router.ServeHTTP(result, request)
				if result.Code != 200 {
					t.Fatalf("%s %s: %d", role, path, result.Code)
				}
			}
			refreshPath := "/api/refresh"
			body := "{}"
			refresh := httptest.NewRequest("POST", refreshPath, strings.NewReader(body))
			if endpoint == "/api/desktop/login" {
				refresh = httptest.NewRequest("POST", "/api/desktop/refresh", strings.NewReader(fmt.Sprintf(`{"refreshToken":%q}`, response.RefreshToken)))
			} else {
				if len(rec.Result().Cookies()) == 0 {
					t.Fatal("refresh cookie missing")
				}
				refresh.AddCookie(rec.Result().Cookies()[0])
			}
			refresh.Header.Set("Content-Type", "application/json")
			result := httptest.NewRecorder()
			router.ServeHTTP(result, refresh)
			if result.Code != 200 {
				t.Fatalf("refresh failed: %d", result.Code)
			}
		}
	}
}

func TestAuthorizationUsesCurrentDatabaseUser(t *testing.T) {
	db := isolatedDB(t)
	migrate(t, db)
	user, _, err := users.Provision(db, "existing", "", models.RoleAdmin, []byte("isolated-current-password"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "isolated-role-test-secret", FrontendOrigins: []string{"http://localhost:4200"}}
	router := routes.SetupRouter(cfg)
	claims := jwt.MapClaims{"sub": user.ID, "login": "untrusted-claim", "role": "admin", "type": "access", "exp": time.Now().Add(time.Hour).Unix()}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err = db.Model(&user).Update("role", models.RoleManager).Error; err != nil {
		t.Fatal(err)
	}
	request := func(token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, r)
		return w
	}
	result := request(token)
	var profile models.UserProfile
	json.Unmarshal(result.Body.Bytes(), &profile)
	if result.Code != 200 || profile.Role != models.RoleManager || profile.Login != user.Login {
		t.Fatal("stale JWT claims used")
	}
	for _, sub := range []any{0, -1, 1.5, "1"} {
		claims["sub"] = sub
		invalid, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
		if request(invalid).Code != 401 {
			t.Fatal("invalid subject accepted")
		}
	}
	db.Delete(&user)
	if request(token).Code != 401 {
		t.Fatal("deleted user accepted")
	}
}

func TestTeamProvisionIsAtomicAndRepeatable(t *testing.T) {
	db := isolatedDB(t)
	migrate(t, db)
	admin, _, err := users.Provision(db, "almak", "", models.RoleManager, []byte("isolated-admin-password"))
	if err != nil {
		t.Fatal(err)
	}
	input := users.InitialTeam{AdminLogin: "almak", Managers: []users.ManagerAccount{{Login: "manager1", Password: "first-test-password"}, {Login: "manager2", Password: "second-test-password"}}}
	// An existing conflicting account must roll back both promotion and manager1.
	conflict, _, err := users.Provision(db, "manager2", "", models.RoleAdmin, []byte("unchanged-conflict-password"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = users.ProvisionTeam(db, input); err == nil {
		t.Fatal("conflicting role accepted")
	}
	var after models.User
	db.First(&after, admin.ID)
	if after.Role != models.RoleManager {
		t.Fatal("partial promotion committed")
	}
	var count int64
	db.Model(&models.User{}).Where("login = ?", "manager1").Count(&count)
	if count != 0 {
		t.Fatal("partial account committed")
	}
	db.Delete(&conflict)
	if _, err = users.ProvisionTeam(db, input); err != nil {
		t.Fatal(err)
	}
	var first []models.User
	db.Order("id").Find(&first)
	if _, err = users.ProvisionTeam(db, input); err != nil {
		t.Fatal(err)
	}
	var second []models.User
	db.Order("id").Find(&second)
	if len(first) != 3 || len(second) != 3 {
		t.Fatal("wrong account count")
	}
	for i, user := range first {
		if user.ID != second[i].ID || user.Password != second[i].Password {
			t.Fatal("repeat setup changed identity or password")
		}
	}
}
