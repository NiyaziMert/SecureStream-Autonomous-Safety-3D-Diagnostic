package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

const (
	apiKey     = "dev-api-key-12345"
	tenantName = "Acme E-Commerce Ltd."
)

var baseURL string

func init() {
	baseURL = os.Getenv("BACKEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080/api"
	}
}

// ---- Veri Yapıları ----

type TopologyNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Val         int    `json:"val"`
	Group       int    `json:"group"`
	Color       string `json:"color"`
	Parent      string `json:"parent,omitempty"`
	Description string `json:"description,omitempty"`
	Tech        string `json:"tech,omitempty"`
	NodeType    string `json:"node_type,omitempty"`
}

type TopologyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Val    int    `json:"val"`
	Label  string `json:"label,omitempty"`
}

type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Links []TopologyLink `json:"links"`
}

type IngestRequest struct {
	Source string `json:"source"`
	RawLog string `json:"raw_log"`
}

// ---- Fake Kullanıcı Veri Havuzu ----

var firstNames = []string{
	"Ahmet", "Mehmet", "Ayşe", "Fatma", "Ali", "Kemal", "Selin", "Canan",
	"James", "Emily", "Carlos", "Maria", "Lucas", "Sara", "Noah", "Liam",
	"yuki", "hiroshi", "priya", "raj", "chen", "wang", "amina", "kwame",
}
var lastNames = []string{
	"Yılmaz", "Kaya", "Demir", "Smith", "Johnson", "Garcia", "Tanaka",
	"Müller", "Dubois", "Rossi", "Santos", "Kim", "Osei", "Patel",
}

var sessionIDs []string

func init() {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 200; i++ {
		sessionIDs = append(sessionIDs, fmt.Sprintf("sess_%08x", rand.Int31()))
	}
}

func randomUser() string {
	return firstNames[rand.Intn(len(firstNames))] + "_" + lastNames[rand.Intn(len(lastNames))]
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(220)+10, rand.Intn(255), rand.Intn(255), rand.Intn(255))
}

func randomSession() string {
	return sessionIDs[rand.Intn(len(sessionIDs))]
}

// ---- Topoloji Tanımı ----
// buildTopology() is defined in topology.go

// ---- Log Üreticileri ----

// normalLogs: Sisteme gerçekçi normal trafik logları üretir
func buildNormalLog() (string, string) {
	roll := rand.Intn(150)
	user := randomUser()
	session := randomSession()
	ip := randomIP()
	productID := rand.Intn(5000) + 1
	orderID := rand.Intn(100000) + 10000

	switch {
	// ── Auth service logs ──
	case roll < 8:
		return "auth_service", fmt.Sprintf(`[INFO] User %s authenticated successfully from %s session=%s`, user, ip, session)
	case roll < 12:
		return "auth_jwt", fmt.Sprintf(`[DEBUG] JWT issued for user=%s exp=3600s session=%s`, user, session)
	case roll < 14:
		return "auth_oauth", fmt.Sprintf(`[INFO] OAuth2 token refresh for user=%s provider=google`, user)
	case roll < 18:
		return "auth_session", fmt.Sprintf(`[DEBUG] Session %s extended TTL=1800 user=%s ip=%s`, session, user, ip)
	case roll < 20:
		return "auth_mfa", fmt.Sprintf(`[INFO] MFA TOTP verified user=%s attempts=1 ip=%s`, user, ip)

	// ── User service logs ──
	case roll < 24:
		return "user_service", fmt.Sprintf(`[INFO] GET /users/%s profile_loaded=true cache=%s latency=%dms`, user, []string{"HIT", "MISS"}[rand.Intn(2)], rand.Intn(30)+5)
	case roll < 26:
		return "user_service", fmt.Sprintf(`[INFO] PUT /users/%s fields_updated=[email,preferences] ip=%s`, user, ip)

	// ── Product service logs ──
	case roll < 32:
		return "product_service", fmt.Sprintf(`[INFO] GET /api/v1/products?page=%d&limit=20 user=%s 200 %dms`, rand.Intn(100), user, rand.Intn(80)+10)
	case roll < 36:
		return "prod_search", fmt.Sprintf(`[INFO] Search query="%s" hits=%d user=%s took=%dms`,
			[]string{"sneakers", "laptop", "headphones", "jacket", "watch", "phone case", "backpack"}[rand.Intn(7)], rand.Intn(200)+1, user, rand.Intn(50)+5)
	case roll < 40:
		return "prod_inventory", fmt.Sprintf(`[DEBUG] Stock check product_id=%d qty=%d warehouse=IST-%02d`, productID, rand.Intn(500), rand.Intn(5)+1)
	case roll < 44:
		return "prod_recommend", fmt.Sprintf(`[INFO] Recommendations generated for user=%s model=collaborative items=%d latency=%dms`, user, rand.Intn(10)+5, rand.Intn(120)+20)
	case roll < 46:
		return "prod_pricing", fmt.Sprintf(`[DEBUG] CalcPrice() product_id=%d base=%.2f discount=%.0f%% final=%.2f`, productID, float64(rand.Intn(10000)+100)/100.0, float64(rand.Intn(30)), float64(rand.Intn(8000)+100)/100.0)

	// ── Order service logs ──
	case roll < 50:
		return "order_service", fmt.Sprintf(`[INFO] POST /orders user=%s items=%d total=%.2f status=pending`, user, rand.Intn(5)+1, float64(rand.Intn(50000)+500)/100.0)
	case roll < 53:
		return "order_cart", fmt.Sprintf(`[DEBUG] ManageCart() user=%s action=%s product_id=%d qty=%d`, user, []string{"add", "remove", "update"}[rand.Intn(3)], productID, rand.Intn(3)+1)
	case roll < 56:
		return "order_checkout", fmt.Sprintf(`[INFO] InitCheckout() order_id=%d saga_step=%s user=%s`, orderID, []string{"reserve_stock", "charge_payment", "confirm"}[rand.Intn(3)], user)
	case roll < 58:
		return "order_tracking", fmt.Sprintf(`[INFO] TrackOrder() order_id=%d status=%s eta=%dh`, orderID, []string{"processing", "shipped", "in_transit", "delivered"}[rand.Intn(4)], rand.Intn(72)+1)

	// ── Payment service logs ──
	case roll < 62:
		return "payment_service", fmt.Sprintf(`[INFO] Payment initiated order_id=%d user=%s amount=%.2f currency=USD`, orderID, user, float64(rand.Intn(50000)+500)/100.0)
	case roll < 66:
		return "pay_stripe", fmt.Sprintf(`[INFO] Stripe charge order_id=%d amount=%.2f status=succeeded`, orderID, float64(rand.Intn(50000)+500)/100.0)
	case roll < 69:
		return "fn_stripe_validate", fmt.Sprintf(`[DEBUG] ValidateToken() card_brand=%s last4=%04d user=%s ok=true`,
			[]string{"visa", "mastercard", "amex"}[rand.Intn(3)], rand.Intn(9000)+1000, user)
	case roll < 73:
		return "fn_stripe_charge", fmt.Sprintf(`[INFO] ProcessCharge() order_id=%d amount=%.2f txn_id=pi_%08x ok=true`, orderID, float64(rand.Intn(50000)+500)/100.0, rand.Int31())
	case roll < 75:
		return "fn_stripe_webhook", fmt.Sprintf(`[INFO] WebhookHandler() event=payment_intent.succeeded order_id=%d`, orderID)
	case roll < 78:
		return "pay_fraud", fmt.Sprintf(`[INFO] FraudCheck order_id=%d user=%s score=%.2f action=allow`, orderID, user, rand.Float64()*0.3)
	case roll < 80:
		return "pay_invoice", fmt.Sprintf(`[INFO] GenerateInvoice() order_id=%d user=%s pdf_size=%dKB uploaded=s3`, orderID, user, rand.Intn(200)+50)

	// ── API Gateway logs ──
	case roll < 85:
		return "api_gateway", fmt.Sprintf(`[INFO] %s /api/v1/%s from=%s latency=%dms rate_remaining=%d`,
			[]string{"GET", "POST", "PUT", "DELETE"}[rand.Intn(4)],
			[]string{"products", "orders", "users/me", "auth/login", "payments"}[rand.Intn(5)],
			ip, rand.Intn(100)+5, rand.Intn(60)+1)

	// ── Infrastructure logs ──
	case roll < 90:
		return "nginx", fmt.Sprintf(`%s - %s [%s] "GET /api/v1/products HTTP/1.1" 200 %d "-" "Mozilla/5.0"`, ip, user, time.Now().Format("02/Jan/2006:15:04:05 -0700"), rand.Intn(5000)+500)
	case roll < 95:
		return "redis", fmt.Sprintf(`[DEBUG] CACHE %s key=%s:%d ttl=%ds`, []string{"HIT", "MISS", "SET"}[rand.Intn(3)], []string{"product", "session", "cart", "price"}[rand.Intn(4)], productID, rand.Intn(300)+60)
	case roll < 100:
		return "db_master", fmt.Sprintf(`[LOG] duration: %.3f ms  statement: SELECT * FROM orders WHERE user_id = $1 LIMIT 20`, rand.Float64()*10+0.5)
	case roll < 104:
		return "db_replica", fmt.Sprintf(`[LOG] duration: %.3f ms  statement: SELECT * FROM products WHERE category_id = $1`, rand.Float64()*5+0.2)
	case roll < 108:
		return "waf", fmt.Sprintf(`ALLOW IN=eth0 SRC=%s DST=10.0.0.10 PROTO=TCP DPT=443 user=%s`, ip, user)

	// ── Analytics & Logging ──
	case roll < 112:
		return "analytics_service", fmt.Sprintf(`[INFO] Event tracked user=%s event=%s page=%s session=%s`,
			user, []string{"page_view", "add_to_cart", "checkout_start", "purchase", "search"}[rand.Intn(5)],
			[]string{"/", "/products", "/cart", "/checkout", "/account"}[rand.Intn(5)], session)
	case roll < 116:
		return "logging_service", fmt.Sprintf(`[DEBUG] Bulk indexed %d log entries to ES cluster latency=%dms`, rand.Intn(500)+50, rand.Intn(200)+10)

	// ── Kafka ──
	case roll < 120:
		return "kafka", fmt.Sprintf(`[INFO] Topic=%s partition=%d offset=%d key=%s`,
			[]string{"order.created", "payment.completed", "user.login", "analytics.events", "order.status.updated"}[rand.Intn(5)],
			rand.Intn(12), rand.Intn(100000)+1000, user)

	// ── CDN & DNS ──
	case roll < 124:
		return "cdn", fmt.Sprintf(`[INFO] %s %s/%s status=%d cache=%s edge=%s bytes=%d`,
			"GET", "assets", []string{"logo.png", "bundle.js", "styles.css", "hero.webp"}[rand.Intn(4)],
			200, []string{"HIT", "MISS", "STALE"}[rand.Intn(3)],
			[]string{"IST", "FRA", "IAD", "NRT"}[rand.Intn(4)], rand.Intn(500000)+1000)
	case roll < 126:
		return "dns", fmt.Sprintf(`[DEBUG] Resolved api.acme-ecom.com → %s ttl=300 geo=%s`, ip, []string{"TR", "DE", "US", "JP"}[rand.Intn(4)])

	// ── Notification ──
	default:
		return "notification_service", fmt.Sprintf(`[INFO] %s sent to=%s@example.com type=%s order_id=%d`,
			[]string{"Email", "SMS", "Push"}[rand.Intn(3)], user,
			[]string{"order_confirmation", "shipping_update", "payment_receipt", "welcome"}[rand.Intn(4)], orderID)
	}
}

// attackLogs: Güvenlik saldırı logları üretir
func buildAttackLog() (string, string) {
	ip := randomIP()
	user := randomUser()
	orderID := rand.Intn(100000) + 10000

	switch rand.Intn(7) {
	case 0:
		return "auth_service", fmt.Sprintf(
			`Failed password for root from %s port 22 ssh2`, ip)
	case 1:
		return "auth_service", fmt.Sprintf(
			`Invalid user %s from %s`, user, ip)
	case 2:
		return "db_master", fmt.Sprintf(
			`FATAL: password authentication failed for user "postgres" from %s`, ip)
	case 3:
		return "auth_session", fmt.Sprintf(
			`sudo: user %s NOT in sudoers ; TTY=pts/0 ; PWD=/root ; USER=root`, user)
	case 4:
		return "prod_search", fmt.Sprintf(
			`GET /search?q=1 UNION ALL SELECT username,password FROM users-- from %s`, ip)
	case 5:
		return "waf", fmt.Sprintf(
			`DROP IN=eth0 SRC=%s DST=10.0.0.10 LEN=60 PROTO=TCP DPT=3306 SYN`, ip)
	default:
		return "pay_fraud", fmt.Sprintf(
			`[ALERT] Fraud detected order_id=%d user=%s ip=%s score=0.97 action=block`,
			orderID, user, ip)
	}
}

// ---- HTTP Yardımcıları ----

var httpClient = &http.Client{Timeout: 5 * time.Second}

func postJSON(endpoint string, payload interface{}) int {
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+endpoint, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// ---- Topoloji Gönderici ----

func sendTopology() {
	topology := buildTopology()
	code := postJSON("/topology", topology)
	if code == 200 {
		fmt.Printf("✅ Topology registered: %d nodes, %d links\n",
			len(topology.Nodes), len(topology.Links))
	} else {
		fmt.Printf("⚠️  Topology registration status: %d\n", code)
	}
}

// ---- Ana Simülasyon ----

func main() {
	fmt.Printf("🚀 [%s] Simulator starting...\n", tenantName)

	// Backend hazır olana kadar bekle
	healthURL := os.Getenv("HEALTH_URL")
	if healthURL == "" {
		healthURL = "http://localhost:8080/health"
	}
	for i := 0; i < 10; i++ {
		resp, err := httpClient.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Println("✅ Backend is up!")
			break
		}
		fmt.Printf("⏳ Waiting for backend... (%d/10)\n", i+1)
		time.Sleep(2 * time.Second)
	}

	// Topolojiyi kaydet
	sendTopology()

	fmt.Println("🌊 Starting high-frequency traffic simulation...")
	fmt.Println("   Normal traffic: ~80%  |  Attack traffic: ~20%")

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			sendTopology() // Topolojiyi 5 dakikada bir yenile
		}
	}()

	for {
		var source, rawLog string

		if rand.Float32() < 0.20 {
			source, rawLog = buildAttackLog()
		} else {
			source, rawLog = buildNormalLog()
		}

		code := postJSON("/ingest", IngestRequest{Source: source, RawLog: rawLog})
		if code > 0 && code != 202 && code != 429 {
			fmt.Printf("⚠️  Unexpected status %d [%s]\n", code, source)
		}

		// 40-120ms arası bekleme = saniyede ~8-25 log
		time.Sleep(time.Duration(rand.Intn(80)+40) * time.Millisecond)
	}
}
